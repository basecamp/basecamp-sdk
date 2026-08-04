# frozen_string_literal: true

module Basecamp
  module Services
    # Tri-state +due_on+ for card updates, prepended onto the generated
    # {CardsService} (see the +on_load+ hook in +basecamp.rb+).
    #
    # BC3's card controller is presence-aware on the JSON representation
    # (+kanban/cards_controller.rb+, basecamp/bc3#12521): +card_update_params+
    # is plain +card_params+, so an update writes exactly the keys the body
    # carries. An omitted +due_on+ leaves the card's due date UNCHANGED; an
    # explicit <tt>""</tt> (or +null+) clears it. The
    # <tt>{ due_on: nil }.merge(card_params)</tt> default survives only for the
    # HTML/turbo_stream web forms, which post every field on every submit.
    #
    # A clear therefore has to be *stated*, never encoded as an omission —
    # omitting +due_on+ to clear it is a silent no-op. <tt>""</tt> is the
    # spelling that travels: JSON +null+ cannot reach the wire from here at
    # all, because +compact_params+ is +kwargs.compact+ and drops nils (SPEC
    # section 18 body compaction). Rails casts the blank string to nil on the
    # date column, so <tt>""</tt> is what a clear looks like end to end.
    #
    # There is no read-before-write. An earlier version GET the card and resent
    # its due date, because the server then nil'd an unmentioned one and a
    # sparse PUT was destructive. Presence-awareness removed the hazard the
    # extra round-trip covered, and with it the race the round-trip opened
    # between the read and the write. Every case is a single PUT.
    module CardsExtensions
      # Updates a card, addressing only what the caller named.
      #
      # +due_on+ is tri-state:
      #
      # * +nil+ (unaddressed) — no +due_on+ key is sent; BC3 leaves the current
      #   due date alone
      # * <tt>""</tt> — a stated clear, sent as <tt>""</tt>
      # * a date — the due date is set
      #
      # Every other argument is plain send-when-set: +nil+ leaves the field off
      # the body, and BC3 leaves the stored value untouched.
      #
      # This is the same single PUT as {#update_verbatim}, which stays as the
      # unnormalised path; +update+ differs only in mapping an empty +due_on+ to
      # the <tt>""</tt> the server reads as a clear.
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
            # Unaddressed. +compact_params+ drops the nil, so no key is sent and
            # the presence-aware update never touches the stored date.
            nil
          elsif due_on.to_s.empty?
            # A stated clear. "" survives +compact_params+ (it removes only
            # nils) and reaches the wire as {"due_on": ""}.
            ""
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
