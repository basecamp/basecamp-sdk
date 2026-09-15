# frozen_string_literal: true

module Basecamp
  module Services
    # Mention-expanding comment writes, prepended onto the generated
    # {CommentsService} (see the +on_load+ hook in +basecamp.rb+).
    #
    # Both methods compose public generated operations — +people.get+ and
    # +comments.create+ — so hooks observe those two wire operations under their
    # own identities, never a synthetic composite (SPEC.md section 18 rule 3).
    module CommentsExtensions
      # Returns content that mentions each of the given people, for posting as a
      # comment — or, since the markup is the same, as a rich-text Campfire line.
      #
      # Every requested id is read through +people.get+ for its
      # +attachable_sgid+ — one read per distinct id, ALWAYS: an sgid already in
      # the content is unsigned and cannot prove the person is mentioned, so it
      # never stands in for the read — and the mentions are placed as
      # {Basecamp::Mentions.with_mentions} places them, which adds nothing for a
      # person whose exact +attachable_sgid+ the content already carries.
      #
      # A person read that fails — an id that is not a person in this account, a
      # 403 — fails the expansion; nothing is posted on a partial mention list.
      # That error is raised unchanged rather than wrapped: in Ruby the error's
      # CLASS and +code+ are the identity a caller matches on, and re-raising a
      # wrapper to carry "which person" in the message would cost both.
      #
      # The rendered mentions round-trip:
      # {Basecamp::Mentions.mentioned_person_ids} on the returned content reports
      # every id passed here, and +recordings.summarize+ reports them on the
      # comment once posted.
      #
      # @param content [String] the comment's rich text
      # @param person_ids [Array<Integer>, nil] people to mention
      # @return [String] the content with the mentions placed
      # @raise [Basecamp::UsageError] on a non-positive person id
      def expand_mentions(content:, person_ids: nil)
        content = require_content(content)
        person_ids = Array(person_ids)
        return content if person_ids.empty?

        seen = {}
        people = []
        person_ids.each do |person_id|
          id = Ids.integer(person_id, "mention person id")
          raise UsageError.new("invalid mention person id #{person_id.inspect}") unless id.positive?
          next if seen.key?(id)

          seen[id] = true
          # Collected with an explicit push rather than by filter_map. That
          # dropped every FALSY return as well as the duplicates it was meant
          # to skip, so a people read answering JSON null or false removed the
          # mention and let the comment post without it — the exact opposite of
          # what this method's own doc promises, and a defect a comment saying
          # "nothing is posted on a partial mention list" made harder to see.
          people << read_person(id, @client.people.get(person_id: id))
        end

        Mentions.with_mentions(content, people)
      end

      # Creates a comment on a recording whose content mentions the given people:
      # {#expand_mentions}, then the generated +create+. The mention reads happen
      # before the write, so a failed lookup posts nothing.
      #
      # @param recording_id [Integer] the recording to comment on
      # @param content [String] the comment's rich text
      # @param person_ids [Array<Integer>, nil] people to mention
      # @return [Hash] the created comment
      # @raise [Basecamp::UsageError] when the content is empty, or on a
      #   non-positive person id
      def create_with_mentions(recording_id:, content:, person_ids: nil)
        # Checked as a STRING, not as `content.to_s.empty?`. That validated a
        # coerced copy and then handed the ORIGINAL to create, so a Hash was
        # posted as a JSON object when no mentions were requested and was
        # rendered into Ruby text by with_mentions when they were — two
        # different wire bodies for one argument, neither of them the documented
        # String. The reference takes `content string`, so a non-string cannot
        # reach it at all; this is the equivalent refusal in a tier that has to
        # make it explicitly.
        content = require_content(content)
        raise UsageError.new("comment content is required") if content.empty?

        # Typed, not bounded. Ruby has no int64 to receive this in, so a value
        # that is not an id at all is refused here — but the reference validates
        # the CONTENT only and sends whatever id it is given, so a zero or a
        # negative goes to the wire and comes back a 404, as it does there.
        recording_id = Ids.integer(recording_id, "recording id")

        create(recording_id: recording_id, content: expand_mentions(content: content, person_ids: person_ids))
      end

      private

      # The comment's rich text, which has to be a String before anything is
      # measured, written or posted.
      def require_content(content)
        return content if content.is_a?(String)

        raise UsageError.new("comment content must be a string, got #{content.class}")
      end

      # The people read's body, which has to be an object before a mention is
      # built from it.
      #
      # The reference decodes into a typed Person, so a scalar, an array or a
      # null body fails the read there and never reaches the write. Here they
      # reached {Basecamp::Mentions.with_mentions}, which would have gone on to
      # index them.
      def read_person(id, person)
        return person if person.is_a?(Hash)

        raise MergeSafe.malformed(
          "the read for person #{id} returned #{MergeSafe.describe(person)}, not a person object",
          "Every requested mention is resolved before the comment is written, so a person that " \
            "cannot be read fails the whole write rather than posting without that mention."
        )
      end
    end
  end
end
