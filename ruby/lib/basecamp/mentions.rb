# frozen_string_literal: true

require "cgi/escape"
require "json"
require "uri"

module Basecamp
  # Mention helpers over Basecamp rich text.
  #
  # A mention in Basecamp rich text is a +<bc-attachment>+ whose +sgid+
  # attribute is the mentioned person's +attachable_sgid+
  # (doc/api/sections/rich_text.md, "Inserting a mention"). BC3 renders the
  # same tag back with +content-type="application/vnd.basecamp.mention"+ and an
  # avatar figure inside it, but the sgid is the only part of the markup that
  # names the person on both the write and the read side, so both helpers here
  # work from it:
  #
  # * {mentioned_person_ids} reads the person ids a rich text names, by
  #   decoding the sgid of every +<bc-attachment>+ and keeping the ones that
  #   point at a Person.
  # * {mention_markup} writes the tag for a person, from their
  #   +attachable_sgid+.
  #
  # An +attachable_sgid+ is a Rails SignedGlobalID: a base64 payload, then
  # <tt>--</tt>, then an HMAC only BC3 can verify. The payload is an envelope
  # carrying the global id — <tt>gid://bc3/Person/1049715915</tt> — as a string,
  # and that string is what these helpers read. They do not (and cannot) verify
  # the signature; what they decode is the same person id BC3 renders into the
  # mention's avatar, read off content the API already served, and a caller that
  # needs the id verified reads the person back through +people.get+.
  #
  # That sets a trust boundary between the two sides. READING —
  # {mentioned_person_ids}, {person_id_from_sgid} — describes what a text says
  # it mentions, and unsigned is fine for description: the ids are reported, not
  # acted on as proof. WRITING — {with_mentions},
  # +CommentsExtensions#expand_mentions+ — never treats an unsigned id as proof
  # that a valid mention already exists: a forged or stale sgid in
  # caller-supplied content naming the right id would otherwise make the writer
  # skip the authoritative people read and post a tag Basecamp will not honour,
  # so the person is silently not mentioned. +expand_mentions+ therefore
  # resolves every requested person through +people.get+ and deduplicates only
  # against the exact +attachable_sgid+ string that read returned. The pure
  # helpers beneath it — {with_mentions}, {mention_markup} — take person hashes
  # the caller built and can only check that an sgid is well-formed and names
  # the person it is given, never that it is authentic: hand them people the API
  # returned, not people assembled from content. Do not reuse the read-side
  # helpers to decide whether a write can be skipped.
  #
  # The markup is read as BC3 serves it: a sanitized tree of the tags
  # doc/api/sections/rich_text.md allows, which has no raw-text elements. The
  # tag walk skips comments and quoted attribute values but does not model
  # +<script>+ or +<style>+ content, which BC3 strips on write; a caller reading
  # mentions out of content it authored itself should not put a bc-attachment
  # inside such an element and expect it ignored.
  #
  # The envelope is decoded structurally, never searched as bytes, so a Person
  # gid that merely appears inside some other value — a Document gid built from
  # one, a purpose string that looks like one — is not a mention, and the
  # envelope's purpose must be "attachable", the one BC3 accepts in rich text.
  # Three envelopes are read: Rails' current Marshal layout
  # <tt>{"_rails" => {"data" => gid, "pur" => purpose}}</tt>, the older Marshal
  # layout <tt>{"gid" => gid, "purpose" => …, "expires_at" => …}</tt>, and the
  # JSON spelling of either, which Rails' JSON message serializer emits.
  #
  # Marshal payloads are decoded by {MarshalReader}, a reader for the small
  # subset of the format an envelope uses — never +Marshal.load+, which would
  # instantiate arbitrary objects out of attacker-supplied rich text.
  module Mentions
    # The SignedGlobalID purpose BC3 mints attachable sgids with
    # (doc/api/sections/rich_text.md: +attachable_sgid+). Pinned by the purpose
    # cases in the mention tests, so a rename upstream breaks a test here rather
    # than silently turning every mention invisible.
    SGID_PURPOSE_ATTACHABLE = "attachable"

    # Bounds the decoded sgid payload. A Person sgid's payload is under 200
    # bytes; the cap keeps a hostile one from costing more than its own size to
    # reject.
    MAX_SGID_PAYLOAD_BYTES = 4096

    # The same bound on the base64 form (4/3 of the payload, plus padding),
    # checked before anything is allocated.
    MAX_SGID_ENCODED_BYTES = (MAX_SGID_PAYLOAD_BYTES / 3 * 4) + 4

    # Characters that may follow "<" in a tag name. The whole name is consumed,
    # punctuation included, so "<bc-attachment:preview" or "<bc-attachment_x" is
    # its own name and never compares equal to "bc-attachment".
    SPACE_CHARS = [ " ", "\t", "\n", "\r", "\f" ].freeze

    module_function

    # Returns the ids of the people a rich text mentions: the Person named by
    # the sgid of each +<bc-attachment>+, in document order, with repeats
    # removed. Attachments that are not mentions — files, images, embeds — are
    # skipped, as is any sgid that does not decode to a Person.
    #
    # This is the read side: a description of what the text says, from sgids
    # whose signatures cannot be checked here. Report it; do not treat an id in
    # it as proof that a valid mention exists (see the trust boundary above).
    #
    # Every +<bc-attachment>+ in the text counts, including one inside a
    # +<blockquote>+: BC3 notifies quoted mentions too, so the read matches what
    # the server does with the write.
    #
    # @param rich_text [String, nil] rich text as BC3 serves it
    # @return [Array<Integer>] mentioned person ids, in document order
    def mentioned_person_ids(rich_text)
      ids = []
      bc_attachment_sgids(rich_text.to_s).each do |sgid|
        id = person_id_from_sgid(sgid)
        next if id.nil? || ids.include?(id)

        ids << id
      end
      ids
    end

    # Decodes the Person id an +attachable_sgid+ names, or nil when the sgid
    # does not decode or names something other than a Person (a file
    # attachment's sgid names an +ActiveStorage::Blob+).
    #
    # This reads the id out of the sgid's payload; it does not verify the sgid's
    # signature, which only BC3 can. It is a read-side helper: never use its
    # answer to decide that a write may skip the authoritative people read (see
    # the trust boundary in the module docs).
    #
    # @param sgid [String, nil]
    # @return [Integer, nil]
    def person_id_from_sgid(sgid)
      gid = global_id_from_sgid(sgid)
      return nil if gid.nil?

      uri = begin
        URI.parse(gid)
      rescue URI::Error
        return nil
      end
      return nil unless uri.scheme == "gid" && !uri.host.to_s.empty?

      # A GlobalID path is exactly "/<Model>/<id>": no more, no less.
      model, raw_id = uri.path.to_s.delete_prefix("/").split("/", 2)
      return nil unless model == "Person" && raw_id.to_s.match?(/\A\d+\z/)

      id = raw_id.to_i
      id.positive? ? id : nil
    end

    # Renders the +<bc-attachment>+ that mentions a person, from their
    # +attachable_sgid+ — the write-side form in doc/api/sections/rich_text.md,
    # which BC3 expands into the avatar figure on read.
    #
    # Raises when the person carries no +attachable_sgid+, which is the case for
    # a person projection that came from somewhere other than a people read (a
    # webhook payload, say), and when the sgid does not name the person it is
    # given. That is all it can check: it cannot verify the signature, so the
    # person must come from the API — a +people.get+, a recording's creator or
    # assignees — not be assembled from an sgid found in content.
    #
    # @param person [Hash] a person as the API returns one
    # @return [String] the mention markup
    # @raise [Basecamp::UsageError] when the person cannot be mentioned
    def mention_markup(person)
      raise UsageError.new("cannot mention a nil person") if person.nil?

      id = field(person, "id")
      sgid = field(person, "attachable_sgid").to_s
      if sgid.empty?
        raise UsageError.new(
          "person #{id} has no attachable_sgid to mention",
          hint: "read the person through people.get to obtain one"
        )
      end
      raise UsageError.new("person #{id} has a malformed attachable_sgid") if sgid.match?(/["'<>&]/)

      # The tag mentions whoever the sgid names. Refuse to write one that names
      # someone else — or a file — under this person's id.
      unless person_id_from_sgid(sgid) == id
        raise UsageError.new(
          "person #{id}'s attachable_sgid does not name that person",
          hint: "read the person through people.get to obtain their own"
        )
      end

      %(<bc-attachment sgid="#{sgid}"></bc-attachment>)
    end

    # Returns content that mentions each of the given people, for posting as a
    # comment or a Campfire line. A person whose exact +attachable_sgid+ the
    # content already carries is left alone, so passing the same person twice —
    # or a person the author already mentioned with that sgid — never duplicates
    # the mention; the rest are added at the start of the content, inside its
    # first +<p>+ or +<div>+ when it opens with one, so they render on the first
    # line rather than as a block of their own.
    #
    # This is the write side, and it deduplicates on the sgid string alone,
    # never on the person id an existing tag's sgid decodes to: that id is
    # unsigned, and a forged or stale tag naming the right person must not stand
    # in for the real mention (see the trust boundary in the module docs). Every
    # person needs their own +attachable_sgid+, and it must be one the API
    # returned: this helper can check that an sgid is well-formed and names the
    # person, not that it is authentic (see {mention_markup}). The
    # account-bound +CommentsExtensions#expand_mentions+ resolves ids to people
    # first and is the entry point that carries that guarantee.
    #
    # @param content [String] rich text to mention into
    # @param people [Array<Hash>] people as the API returns them
    # @return [String] the content with the mentions placed
    # @raise [Basecamp::UsageError] when a person cannot be mentioned
    def with_mentions(content, people)
      content = content.to_s
      present = {}
      bc_attachment_sgids(content).each { |sgid| present[sgid] = true }

      tags = []
      Array(people).each do |person|
        # Rendered before the dedupe check, so an unusable sgid is refused even
        # when the content already carries it.
        tag = mention_markup(person)
        sgid = field(person, "attachable_sgid").to_s
        next if present.key?(sgid)

        present[sgid] = true
        tags << tag
      end
      return content if tags.empty?

      prefix = "#{tags.join(" ")} "
      block_end = leading_block_end(content)
      return prefix + content if block_end.negative?

      content[0, block_end] + prefix + content[block_end..]
    end

    # Returns the sgid attribute of every +<bc-attachment>+ in the text, in
    # document order.
    #
    # It walks the markup as a stream of tags rather than pattern-matching for
    # one tag name, so a +<bc-attachment>+ inside an HTML comment or inside
    # another element's quoted attribute is not an element; and it tokenizes
    # each tag's attributes rather than pattern-matching them, so a ">" inside a
    # quoted value does not end the tag, an "sgid=" inside another attribute's
    # value is not an attribute, either quote style works, attribute order and
    # case are free, the first sgid attribute wins as in HTML, and entity
    # escapes in the value are decoded as a browser would.
    #
    # @param text [String]
    # @return [Array<String>]
    def bc_attachment_sgids(text)
      sgids = []
      pos = 0
      length = text.length

      while pos < length
        open = text.index("<", pos)
        break if open.nil?

        pos = open + 1
        if text[pos, 3] == "!--"
          stop = text.index("-->", pos)
          return sgids if stop.nil? # an unterminated comment swallows the rest

          pos = stop + 3
          next
        elsif [ "!", "?", "/" ].include?(text[pos])
          stop = text.index(">", pos)
          return sgids if stop.nil?

          pos = stop + 1
          next
        end

        name_end = 0
        name_end += 1 while pos + name_end < length && tag_name_char?(text[pos + name_end])
        next if name_end.zero? # a bare "<" in text

        name = text[pos, name_end]
        attrs, tag_end, closed = parse_attributes(text, pos + name_end)
        return sgids unless closed # an unterminated tag: nothing after it is markup

        sgids << attrs[:sgid] if name.casecmp?("bc-attachment") && !attrs[:sgid].to_s.empty?
        pos = tag_end
      end

      sgids
    end

    # Walks the attributes of an opening tag from +pos+ (just after the tag
    # name) to its closing ">".
    #
    # The first sgid attribute wins, present-but-empty included, as HTML
    # resolves a repeated attribute.
    #
    # @param text [String]
    # @param pos [Integer] index just after the tag name
    # @return [Array(Hash, Integer, Boolean)] what was found, the index after
    #   the ">", and whether the tag was closed at all
    def parse_attributes(text, pos)
      attrs = { sgid: nil, sgid_seen: false }
      length = text.length

      while pos < length
        pos += 1 while pos < length && (space?(text[pos]) || text[pos] == "/")
        return [ attrs, pos, false ] if pos >= length
        return [ attrs, pos + 1, true ] if text[pos] == ">"

        name_start = pos
        pos += 1 while pos < length && !space?(text[pos]) && ![ "=", ">", "/" ].include?(text[pos])
        name = text[name_start...pos]
        pos += 1 while pos < length && space?(text[pos])

        value = ""
        if pos < length && text[pos] == "="
          pos += 1
          pos += 1 while pos < length && space?(text[pos])
          if pos < length && [ '"', "'" ].include?(text[pos])
            quote = text[pos]
            pos += 1
            closing = text.index(quote, pos)
            return [ attrs, length, false ] if closing.nil?

            value = text[pos...closing]
            pos = closing + 1
          else
            value_start = pos
            pos += 1 while pos < length && !space?(text[pos]) && text[pos] != ">"
            value = text[value_start...pos]
          end
        end

        if name.empty?
          # A stray "=" or quote where a name should be: step over it.
          pos += 1
          next
        end

        if !attrs[:sgid_seen] && name.casecmp?("sgid")
          attrs[:sgid_seen] = true
          attrs[:sgid] = CGI.unescapeHTML(value)
        end
      end

      [ attrs, pos, false ]
    end

    # Returns the index just past the opening <p …> or <div …> tag a rich text
    # starts with, or -1 when it starts with anything else, so mentions can be
    # placed inside the first block rather than as a bare prefix in front of it.
    # The tag's attributes are scanned quote-aware: a ">" inside an attribute
    # value does not end it.
    #
    # @param content [String]
    # @return [Integer]
    def leading_block_end(content)
      i = 0
      i += 1 while i < content.length && space?(content[i])

      [ "<p", "<div" ].each do |name|
        next if content.length < i + name.length
        next unless content[i, name.length].casecmp?(name)

        after = i + name.length
        next if after < content.length && !tag_name_end?(content[after])

        _attrs, tag_end, closed = parse_attributes(content, after)
        return closed ? tag_end : -1
      end

      -1
    end

    # Returns the global id string an sgid's envelope carries, or nil.
    #
    # A signed sgid is <tt><payload>--<digest></tt>, and "-" is a base64url
    # character, so the payload itself may contain <tt>--</tt>. The separator is
    # therefore the LAST one, as Rails' own verifier reads it; the whole value is
    # tried as a bare payload when that fails, which is what an unsigned envelope
    # — one that happens to contain <tt>--</tt> included — needs.
    #
    # @param sgid [String, nil]
    # @return [String, nil]
    def global_id_from_sgid(sgid)
      value = sgid.to_s.strip
      separator = value.rindex("--")
      if separator && separator.positive?
        gid = envelope_gid(value[0, separator])
        return gid if gid
      end
      envelope_gid(value)
    end

    # Decodes one base64 payload and returns the gid its envelope carries.
    #
    # @param payload [String]
    # @return [String, nil]
    def envelope_gid(payload)
      raw = decode_payload(payload)
      return nil if raw.nil?

      envelope =
        if raw.bytesize >= 2 && raw.getbyte(0) == 0x04 && raw.getbyte(1) == 0x08
          MarshalReader.parse(raw.byteslice(2..))
        elsif raw.getbyte(0) == 0x7B # "{"
          begin
            JSON.parse(raw)
          rescue JSON::ParserError
            nil
          end
        end
      return nil unless envelope.is_a?(Hash)

      # A SignedGlobalID is bound to a purpose, and only an "attachable" one may
      # be placed in rich text: BC3 refuses any other, so a Person sgid minted
      # for bookmarking or reading is not a mention however valid its gid. Both
      # layouts carry the purpose; an envelope without one is not a Rails
      # envelope.
      rails = envelope["_rails"]
      if rails.is_a?(Hash)
        # Current layout: {"_rails" => {"data" => gid, "pur" => purpose}}.
        return nil unless rails["pur"] == SGID_PURPOSE_ATTACHABLE

        gid = rails["data"]
        return gid.is_a?(String) && !gid.empty? ? gid : nil
      end

      # Older layout: {"gid" => gid, "purpose" => …, "expires_at" => …}.
      return nil unless envelope["purpose"] == SGID_PURPOSE_ATTACHABLE

      gid = envelope["gid"]
      gid.is_a?(String) && !gid.empty? ? gid : nil
    end

    # Decodes an sgid payload's base64, bounded on both the encoded and decoded
    # forms. The bound is applied to the encoded form first, so an oversized
    # sgid costs nothing to refuse — no normalization, no decode buffer.
    #
    # Rails' MessageVerifier emits either alphabet; base64url is current. Both
    # decode through the standard alphabet once the two symbols are mapped, and
    # stripping the padding lets a truncated-but-valid payload through.
    #
    # @param payload [String]
    # @return [String, nil] binary-encoded bytes
    def decode_payload(payload)
      return nil if payload.nil? || payload.empty? || payload.length > MAX_SGID_ENCODED_BYTES

      normalized = payload.tr("-_", "+/").sub(/=+\z/, "")
      return nil unless normalized.match?(%r{\A[A-Za-z0-9+/]+\z})
      return nil if (normalized.length % 4) == 1

      raw = begin
        (normalized + ("=" * ((4 - (normalized.length % 4)) % 4))).unpack1("m0")
      rescue ArgumentError
        return nil
      end
      return nil if raw.nil? || raw.empty? || raw.bytesize > MAX_SGID_PAYLOAD_BYTES

      raw
    end

    # Reads a field off a person hash, string keys first, symbol keys second, so
    # a hash the SDK returned and one a caller typed both work.
    def field(person, key)
      return nil unless person.respond_to?(:[])

      person.key?(key) ? person[key] : person[key.to_sym]
    end

    def space?(char)
      SPACE_CHARS.include?(char)
    end

    def tag_name_char?(char)
      !space?(char) && ![ "/", ">", "<", "=", '"', "'" ].include?(char)
    end

    def tag_name_end?(char)
      space?(char) || char == "/" || char == ">"
    end

    # The helpers above that exist only to serve the four public ones. They are
    # module methods so the public ones can call them with an implicit receiver,
    # and private so the module's documented surface is the four — plus
    # {bc_attachment_sgids}, which a caller deduplicating its own writes needs.
    private_class_method :parse_attributes, :leading_block_end, :global_id_from_sgid,
                         :envelope_gid, :decode_payload, :field, :space?,
                         :tag_name_char?, :tag_name_end?

    # A reader for the subset of Ruby's Marshal 4.8 format a SignedGlobalID
    # payload uses — nil, booleans, fixnums, strings (with their encoding
    # ivars), symbols and symbol links, arrays and hashes — decoded into plain
    # values: Hash, Array, String, Integer, true/false, nil.
    #
    # Deliberately NOT +Marshal.load+. An sgid arrives inside rich text the API
    # served, which is content other people wrote; +Marshal.load+ on it would
    # instantiate arbitrary classes and run their +marshal_load+. Anything
    # outside the subset is nil here, and the caller then treats the sgid as
    # undecodable rather than guessing.
    class MarshalReader
      # Bounds nesting in a payload; an envelope is two deep.
      MAX_DEPTH = 32

      # Parses a complete Marshal document (the 4.8 header already stripped),
      # returning nil for anything this reader does not model.
      #
      # @param data [String] binary bytes after the "\x04\x08" header
      # @return [Object, nil]
      def self.parse(data)
        reader = new(data)
        value = reader.read_value(0)
        # A Marshal dump is exactly one value; bytes after it are corruption.
        reader.exhausted? ? value : nil
      rescue Error
        nil
      end

      # Raised internally for any byte sequence outside the modeled subset.
      class Error < StandardError; end

      def initialize(data)
        @data = data.b
        @pos = 0
        @symbols = []
      end

      def exhausted?
        @pos == @data.bytesize
      end

      def read_value(depth)
        raise Error, "nesting too deep" if depth > MAX_DEPTH

        case read_byte
        when 0x30 then nil                     # "0"
        when 0x54 then true                    # "T"
        when 0x46 then false                   # "F"
        when 0x69 then read_int                # "i"
        when 0x22 then read_bytes(read_count)  # '"'
        when 0x3A then read_symbol             # ":"
        when 0x3B then read_symbol_link        # ";"
        when 0x49 then read_ivar_object(depth) # "I"
        when 0x5B then read_array(depth)       # "["
        when 0x7B then read_hash(depth)        # "{"
        else raise Error, "unsupported type"
        end
      end

      private

      def read_byte
        raise Error, "unexpected end of data" if @pos >= @data.bytesize

        byte = @data.getbyte(@pos)
        @pos += 1
        byte
      end

      # Takes the next n bytes. The bound is checked against what remains, never
      # by adding n to the position, and every length reaches here through
      # {read_count}, which already rejected anything past the remaining bytes.
      def read_bytes(count)
        raise Error, "unexpected end of data" if count.negative? || count > @data.bytesize - @pos

        bytes = @data.byteslice(@pos, count)
        @pos += count
        bytes
      end

      # Reads Marshal's packed integer: 0 is 0; 1..4 and -1..-4 are a byte count
      # for a little-endian value; anything else is the value itself offset by 5.
      def read_int
        lead = read_byte
        # The lead byte is a signed int8; widen it into its signed meaning.
        lead -= 256 if lead > 127

        return 0 if lead.zero?
        return lead - 5 if lead > 4
        return lead + 5 if lead < -4

        if lead.positive?
          value = 0
          read_bytes(lead).each_byte.with_index { |byte, i| value |= byte << (8 * i) }
          value
        else
          value = -1
          read_bytes(-lead).each_byte.with_index do |byte, i|
            value &= ~(0xff << (8 * i))
            value |= byte << (8 * i)
          end
          value
        end
      end

      # Reads a length or count — string and symbol bytes, array elements, hash
      # pairs, ivar pairs — and rejects one that cannot be honest: negative, or
      # more than the bytes left (every element takes at least one byte).
      # Allocation follows what actually decodes, so a hostile count costs its
      # own bytes to refuse, never the capacity it claims.
      def read_count
        count = read_int
        raise Error, "bad count" if count.negative? || count > @data.bytesize - @pos

        count
      end

      def read_symbol
        symbol = read_bytes(read_count)
        @symbols << symbol
        symbol
      end

      def read_symbol_link
        index = read_int
        raise Error, "bad symbol link" if index.negative? || index >= @symbols.length

        @symbols[index]
      end

      # An object followed by its instance variables — a String's encoding.
      def read_ivar_object(depth)
        inner = read_value(depth + 1)
        read_count.times do
          read_value(depth + 1) # ivar name
          read_value(depth + 1) # ivar value
        end
        inner
      end

      def read_array(depth)
        count = read_count
        Array.new(0).tap do |out|
          count.times { out << read_value(depth + 1) }
        end
      end

      def read_hash(depth)
        count = read_count
        out = {}
        count.times do
          key = read_value(depth + 1)
          value = read_value(depth + 1)
          raise Error, "non-string hash key" unless key.is_a?(String)

          out[key] = value
        end
        out
      end
    end
  end
end
