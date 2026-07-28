# frozen_string_literal: true

module Basecamp
  module Services
    # Merge-safe +update+ for cards, prepended onto the generated
    # {CardsService} (see the +on_load+ hook in +basecamp.rb+).
    #
    # BC3 builds the card's update params as
    # <tt>{ due_on: nil }.merge(card_params)</tt>
    # (+kanban/cards_controller.rb+), so *any* update whose body omits +due_on+
    # erases the card's due date. A sparse PUT — the natural thing to write —
    # is therefore destructive on the raw endpoint, which remains available as
    # {#update_verbatim}.
    #
    # +update+ composes the public +get+ and +update_verbatim+ methods, so
    # hooks observe the two wire operations, not a synthetic composite.
    #
    # Not atomic: a concurrent due-date change landing between the GET and the
    # PUT is overwritten with the value this call read. The window is one
    # round-trip.
    module CardsExtensions
      # Updates a card without disturbing fields the caller did not mention.
      #
      # +due_on+ is tri-state, which is what makes this safe:
      #
      # * +nil+ (omitted) — the current due date is fetched and resent
      # * <tt>""</tt> — the due date is cleared
      # * a date — the due date is set
      #
      # The extra GET is only paid for in the +nil+ case, the one where the
      # API would otherwise destroy something.
      #
      # Assignees are never resent on the caller's behalf: BC3 filters incoming
      # IDs through +reachable_people+, so echoing back an id belonging to
      # someone who has since lost board access would silently unassign them.
      #
      # @param card_id [Integer] card id
      # @param title [String, nil] new title (nil = keep current)
      # @param content [String, nil] new content (nil = keep current, "" clears)
      # @param due_on [String, nil] new due date (nil = keep current, "" clears)
      # @param assignee_ids [Array, nil] new assignees (nil = keep current, [] clears)
      # @return [Hash] the updated card
      def update(card_id:, title: nil, content: nil, due_on: nil, assignee_ids: nil)
        resolved_due_on =
          if due_on.nil?
            get(card_id: card_id)["due_on"]
          elsif due_on.to_s.empty?
            # Clearing is encoded by OMITTING due_on — compact_params strips the
            # nil below, and BC3 nils an omitted due date. Sending an explicit
            # null would violate body compaction (SPEC §18), and sending ""
            # risks a date-format error.
            nil
          else
            due_on
          end

        update_verbatim(
          card_id: card_id,
          title: title,
          content: content,
          due_on: resolved_due_on,
          assignee_ids: assignee_ids
        )
      end
    end
  end
end
