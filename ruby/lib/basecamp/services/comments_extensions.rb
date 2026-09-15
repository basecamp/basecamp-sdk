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
        person_ids = Array(person_ids)
        return content if person_ids.empty?

        seen = {}
        people = person_ids.filter_map do |person_id|
          id = Ids.integer(person_id, "mention person id")
          raise UsageError.new("invalid mention person id #{person_id.inspect}") unless id.positive?
          next if seen.key?(id)

          seen[id] = true
          @client.people.get(person_id: id)
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
        raise UsageError.new("comment content is required") if content.to_s.empty?

        create(recording_id: recording_id, content: expand_mentions(content: content, person_ids: person_ids))
      end
    end
  end
end
