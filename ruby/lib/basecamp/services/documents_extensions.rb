# frozen_string_literal: true

module Basecamp
  module Services
    # Merge-safe +update+ and read-modify-write +edit+ for documents,
    # prepended onto the generated {DocumentsService} (see the +on_load+ hook
    # in +basecamp.rb+).
    #
    # BC3's +DocumentsController#update+ builds a brand-new +Document+ from
    # only the permitted params and swaps the recordable wholesale, so
    # <tt>PUT /documents/{id}</tt> is a full replace: a body that omits
    # +content+ ERASES it, and one that omits +title+ erases that too — the
    # document then reads back as <tt>"Untitled"</tt>, because +Document#title+
    # falls back when blank. Neither attribute is presence-validated, so
    # *neither omission is a 422*; both are a 200 that quietly clears. What BC3
    # does require is the wrapping +document+ object, so a body naming neither
    # field is a 400. The sparse PUT — the natural thing to write — is
    # therefore destructive on the raw endpoint, which stays available as
    # {#replace}.
    #
    # Both compose the public +get+ and +replace+ methods, so hooks observe
    # the two wire operations (+get+ then +replace+), not a synthetic
    # composite.
    #
    # Neither is atomic: there is no conditional-update signal on this
    # endpoint, so a concurrent write between the GET and PUT is
    # overwritten — last write wins for the whole representation. The
    # window is one round-trip. Use +replace+ to overwrite deliberately.
    module DocumentsExtensions
      # A document's full writable state, yielded to the +edit+ block. The
      # whole struct is PUT back to the server, so clearing a field means
      # setting it empty (<tt>""</tt>) — there is no third state. The writable
      # set is exactly what BC3 permits: +title+ and +content+.
      DocumentFields = Struct.new(:title, :content, keyword_init: true)

      # The deliberate-overwrite escape hatch named in every malformed-response
      # hint raised out of this composite.
      ESCAPE_HATCH = "replace"

      # Sets the given fields on a document and preserves everything else:
      # GETs the current document, overlays the explicitly-passed keyword
      # arguments, and PUTs the full representation back. An omitted (+nil+)
      # field is untouched, guaranteed; an explicitly-passed <tt>""</tt>
      # clears.
      #
      # Not atomic — see the module docs for the GET→PUT race. Use {#replace}
      # to overwrite deliberately.
      #
      # @param document_id [Integer] document id
      # @param title [String, nil] new title (nil = keep current, "" clears)
      # @param content [String, nil] new content (nil = keep current, "" clears)
      # @return [Hash] the updated document
      def update(document_id:, title: nil, content: nil)
        fields = fields_from_document(get(document_id: document_id))
        fields.title = title unless title.nil?
        fields.content = content unless content.nil?
        put_fields(document_id, fields)
      end

      # Applies a read-modify-write block to a document: GETs the current
      # document, yields its full writable state ({DocumentFields}), and PUTs
      # the whole thing back. Clearing a field means setting it empty
      # (<tt>""</tt>) — an untouched field keeps its current value. If the
      # block raises, the edit aborts and nothing is written.
      #
      # Not atomic — see the module docs for the GET→PUT race.
      #
      # @example
      #   account.documents.edit(document_id: 123) do |doc|
      #     doc.title = "🚨 #{doc.title}"
      #     doc.content = "" # clearing = setting empty on a full object
      #   end
      #
      # @param document_id [Integer] document id
      # @yieldparam fields [DocumentFields] the document's writable state, to mutate in place
      # @return [Hash] the updated document
      # @raise [ArgumentError] if no block is given
      def edit(document_id:)
        raise ArgumentError, "edit requires a block" unless block_given?

        fields = fields_from_document(get(document_id: document_id))
        yield fields
        put_fields(document_id, fields)
      end

      private

      # Derives the full writable state from a GET response.
      #
      # Every value here is resent in the full-replace PUT, so every value is
      # validated before it is read. A plain <tt>|| ""</tt> would turn +false+
      # into <tt>""</tt> — erasing the field on a call that never mentioned
      # it — and pass arrays, hashes, numbers and +true+ straight through to be
      # written verbatim. Ruby has no typed decoder between the GET and this
      # read (+get+ returns a raw Hash), so the check is explicit work here
      # rather than something the layer below already did. See {MergeSafe} and
      # #576.
      #
      # The two writable fields read differently because the spec models them
      # differently: +title+ is <tt>@required</tt>, so absent or nil is
      # malformed; +content+ is optional, so absent or nil is a genuinely empty
      # body.
      def fields_from_document(document)
        body = MergeSafe.require_hash(
          document, record: "Document", operation: "GetDocument", escape: ESCAPE_HATCH
        )
        DocumentFields.new(
          title: required_writable_string(body, "title"),
          content: MergeSafe.writable_string(body, "content", record: "Document", escape: ESCAPE_HATCH)
        )
      end

      # Reads a writable string the record is *required* to carry.
      #
      # +MergeSafe.writable_string+ treats an absent key or an explicit +nil+ as
      # genuinely empty, which is right for an optional field — <tt>""</tt> is
      # what the server already holds. It is wrong for a required one.
      # +Document.title+ is <tt>@required</tt> in the spec and BC3 can never
      # render it blank (+Document#title+ is
      # <tt>super.presence || "Untitled"</tt>), so an absent or nil +title+ in a
      # 2xx body is a malformed response, not an empty title. Coalescing it to
      # <tt>""</tt> and sending that in the full-replace PUT would blank the real
      # title on a call that only touched +content+ — #576's defect exactly: a
      # value the caller never mentioned, silently substituted.
      #
      # The wrong-type branch is delegated to +MergeSafe.writable_string+, so a
      # required field and an optional one report a non-string identically.
      def required_writable_string(body, key)
        value = body[key]
        if value.nil? || value == ""
          raise MergeSafe.malformed(
            %(Document field "#{key}" is required but the response carried #{MergeSafe.describe(value)}),
            "The merge-safe update/edit resend this field verbatim, so a missing or blank value " \
            "would blank the current one. Use #{ESCAPE_HATCH} to write the record deliberately."
          )
        end

        MergeSafe.writable_string(body, key, record: "Document", escape: ESCAPE_HATCH)
      end

      # PUTs the full writable state via +replace+. Both fields are always
      # sent, empties included: the generated layer's +compact_params+ strips
      # nils, so a cleared field travels as <tt>""</tt> rather than JSON null
      # (SPEC section 18 body compaction) — and omitting it would hand the
      # clear back to the server's rebuild instead of stating it.
      #
      # +nil+ is normalised to <tt>""</tt> rather than coerced with +to_s+: the
      # struct starts nil-valued and clearing by assigning nil is idiomatic,
      # but +to_s+ would silently turn a block's +42+ into <tt>"42"</tt> and
      # write it — the same corruption {MergeSafe} refuses on the read side.
      # Validating what the *caller* assigns is the mirror of that rule and is
      # deliberately out of scope here, exactly as in #576: a value the caller
      # chose is not a value silently substituted for one they asked to
      # preserve.
      def put_fields(document_id, fields)
        replace(
          document_id: document_id,
          title: fields.title.nil? ? "" : fields.title,
          content: fields.content.nil? ? "" : fields.content
        )
      end
    end
  end
end
