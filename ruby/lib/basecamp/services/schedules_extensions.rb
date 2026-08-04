# frozen_string_literal: true

module Basecamp
  module Services
    # Merge-safe +update_entry+ and read-modify-write +edit_entry+ for schedule
    # entries, prepended onto the generated {SchedulesService} (see the
    # +on_load+ hook in +basecamp.rb+).
    #
    # BC3's +Schedules::EntriesController#update+ rebuilds the recordable from
    # the submitted params, so <tt>PUT /schedule_entries/{id}</tt> is a full
    # replace: a body that omits +description+ ERASES it, and one that omits
    # +summary+ erases that too — the entry then reads back as
    # <tt>"Untitled"</tt>, because +Schedule::Entry#summary+ is
    # <tt>super.presence || "Untitled"</tt>. The sparse PUT — the natural thing
    # to write — is therefore destructive on the raw endpoint, which stays
    # available as +replace_entry+.
    #
    # == Two classes of writable field
    #
    # Unlike documents and todolists, this record's writable set does not have
    # one uniform rule. It splits in two:
    #
    # [full state] +summary+, +starts_at+, +ends_at+, +description+, +all_day+.
    #              Always resent, empties included: <tt>""</tt> is how a clear is
    #              expressed on a full-replace endpoint, never JSON null and
    #              never an omission.
    # [addressed-only] +participant_ids+, +url+, +highlighted+, +notify+. Sent
    #                  *only* when the caller addressed them, and never seeded
    #                  onto the wire from the read-back.
    #
    # The first three of the addressed-only set are the operation's
    # +preservedOnOmission+ carve-out: BC3 seeds them from the existing
    # recordable when the request does not address them, so resending them is
    # redundant at best and wrong if the GET raced a concurrent change. Echoing
    # the response's +url+ would be worse than redundant — that key is the
    # entry's own Basecamp API URL, written by +recordings/_recording+ before
    # the entry partial renders, so BC3 emits the join link under the
    # non-colliding +join_url+. Writing +url+ back would store the API URL as
    # the join link. +notify+ is addressed-only for a different reason: it is a
    # directive, not state — sending it makes BC3 recompute a drafted entry's
    # subscriber list — and the read-back carries nothing to seed it from.
    #
    # An explicitly *empty* value in that second class is an address, not an
    # absence: <tt>participant_ids: []</tt> clears participants,
    # <tt>url: ""</tt> clears the join link, <tt>highlighted: false</tt> removes
    # the highlight. All three survive body compaction, which strips only nil
    # (SPEC section 18).
    #
    # == Recurring entries
    #
    # +ensure_non_recurring_event+ 302-redirects both +show+ and +update+ for a
    # recurring entry, so this route serves non-recurring entries only. The SDK
    # does not follow redirects on a PUT, and the GET's redirect lands on a body
    # this composite refuses rather than reads. Recurrence itself
    # (+recurrence_schedule+, +recurs_until+, +time_zone_name+) is unmodelled
    # here and stays unmodelled: BC3 forces all three to nil for a non-recurring
    # entry.
    #
    # Both methods compose the public +get_entry+ and +replace_entry+, so hooks
    # observe the two wire operations (+get_entry+ then +replace_entry+), not a
    # synthetic composite.
    #
    # Neither is atomic: there is no conditional-update signal on this endpoint,
    # so a concurrent write between the GET and PUT is overwritten — last write
    # wins for the whole representation. The window is one round-trip. Use
    # +replace_entry+ to overwrite deliberately.
    module SchedulesExtensions
      # The deliberate-overwrite escape hatch named in every malformed-response
      # hint raised out of this composite.
      ESCAPE_HATCH = "replace_entry"

      # The record name interpolated into {MergeSafe}'s messages.
      RECORD = "Schedule entry"

      # The writable members BC3 preserves when the request does not address
      # them, plus +notify+, which is a directive rather than state. Sent only
      # on an explicit address; never seeded from the read-back.
      CARVE_OUTS = %i[participant_ids url highlighted notify].freeze

      # How each writable member spells "cleared" on the wire.
      #
      # +compact_params+ strips nil (SPEC section 18), so a nil that reached the
      # request would silently become an omission — an address turned back into
      # an absence, which on a full-replace endpoint is exactly the defect this
      # composite exists to prevent. Every member whose type has an empty value
      # is normalised to it here. The three booleans (+all_day+, +highlighted+,
      # +notify+) have none: a boolean is true or false, so a nil assigned to
      # one is caller error and is refused rather than dropped.
      CLEARED = {
        summary: "",
        starts_at: "",
        ends_at: "",
        description: "",
        participant_ids: [].freeze,
        url: ""
      }.freeze

      # A schedule entry's writable state, yielded to the {#edit_entry} block.
      #
      # The full-state members are plain accessors: they are resent whether or
      # not the block touches them, so nothing has to be recorded. The
      # carve-outs are readable — seeded from the read-back so a block can
      # inspect the current join link, highlight and participants before
      # deciding — but each writer *records the address*, and only an addressed
      # carve-out reaches the wire.
      #
      # Dirty tracking is by setter invocation, deliberately, and not by
      # comparing the block's result against the read-back: assigning a
      # carve-out the value the GET just returned is an address like any other.
      # A diff would drop it, and the server would then hold whatever a
      # concurrent writer left there instead of the value the caller stated.
      class ScheduleEntryFields
        attr_accessor :summary, :starts_at, :ends_at, :description, :all_day
        attr_reader :participant_ids, :url, :highlighted, :notify

        def initialize(summary:, starts_at:, ends_at:, description:, all_day:,
                       participant_ids:, url:, highlighted:)
          @summary = summary
          @starts_at = starts_at
          @ends_at = ends_at
          @description = description
          @all_day = all_day
          @participant_ids = participant_ids
          @url = url
          @highlighted = highlighted
          # Nothing in the response seeds a directive.
          @notify = nil
          @addressed = {}
        end

        def participant_ids=(value)
          @addressed[:participant_ids] = true
          @participant_ids = value
        end

        def url=(value)
          @addressed[:url] = true
          @url = value
        end

        def highlighted=(value)
          @addressed[:highlighted] = true
          @highlighted = value
        end

        def notify=(value)
          @addressed[:notify] = true
          @notify = value
        end

        # Whether the caller addressed this carve-out, by assignment.
        def addressed?(name)
          @addressed.key?(name)
        end
      end

      # Sets the given fields on a schedule entry and preserves everything else:
      # GETs the current entry, overlays the explicitly-passed keyword
      # arguments, and PUTs the full representation back.
      #
      # An omitted (+nil+) argument is untouched, guaranteed. For the full-state
      # fields that means the read-back value is resent; for the addressed-only
      # fields it means the key never reaches the wire, leaving BC3 to seed it
      # from the record it already holds. An explicitly-passed <tt>""</tt>,
      # <tt>[]</tt> or +false+ is an address and is sent.
      #
      # +nil+ is an unambiguous "not addressed" for every one of these
      # arguments: none of them has a JSON null wire spelling — a clear is
      # <tt>""</tt>, <tt>[]</tt> or +false+ — and +compact_params+ strips nil
      # before serialization anyway, so no sentinel is needed to tell "passed
      # nil" from "not passed".
      #
      # Not atomic — see the module docs for the GET→PUT race, and for the
      # recurring-entry redirect. Use +replace_entry+ to overwrite deliberately.
      #
      # @param entry_id [Integer] entry id
      # @param summary [String, nil] new summary (nil = keep current)
      # @param starts_at [String, nil] new start, a date or timestamp (nil = keep current)
      # @param ends_at [String, nil] new end, a date or timestamp (nil = keep current)
      # @param description [String, nil] new description (nil = keep current, "" clears)
      # @param all_day [Boolean, nil] new all-day flag (nil = keep current)
      # @param participant_ids [Array<Integer>, nil] replaces participants (nil = leave to BC3, [] clears)
      # @param url [String, nil] new join link (nil = leave to BC3, "" clears)
      # @param highlighted [Boolean, nil] new highlight (nil = leave to BC3, false removes)
      # @param notify [Boolean, nil] notify participants (nil = do not address)
      # @return [Hash] the updated schedule entry
      def update_entry(entry_id:, summary: nil, starts_at: nil, ends_at: nil, description: nil,
                       all_day: nil, participant_ids: nil, url: nil, highlighted: nil, notify: nil)
        # Delegating to edit_entry is not a shortcut: it makes "the caller
        # addressed this" one rule with one implementation. A non-nil argument
        # invokes the same writer a block would, so the carve-outs are recorded
        # by exactly the mechanism edit_entry documents.
        edit_entry(entry_id: entry_id) do |entry|
          entry.summary = summary unless summary.nil?
          entry.starts_at = starts_at unless starts_at.nil?
          entry.ends_at = ends_at unless ends_at.nil?
          entry.description = description unless description.nil?
          entry.all_day = all_day unless all_day.nil?
          entry.participant_ids = participant_ids unless participant_ids.nil?
          entry.url = url unless url.nil?
          entry.highlighted = highlighted unless highlighted.nil?
          entry.notify = notify unless notify.nil?
        end
      end

      # Applies a read-modify-write block to a schedule entry: GETs the current
      # entry, yields its writable state ({ScheduleEntryFields}), and PUTs it
      # back. The full-state fields are resent whether or not the block touches
      # them; a carve-out is sent only if the block assigns it, even when it
      # assigns the value the read already returned. If the block raises, the
      # edit aborts and nothing is written.
      #
      # Not atomic — see the module docs for the GET→PUT race, and for the
      # recurring-entry redirect.
      #
      # @example Clear the description, leave the join link and highlight alone
      #   account.schedules.edit_entry(entry_id: 123) do |entry|
      #     entry.summary = "🚨 #{entry.summary}"
      #     entry.description = "" # clearing = setting empty on a full object
      #   end
      #
      # @example Address a carve-out
      #   account.schedules.edit_entry(entry_id: 123) do |entry|
      #     entry.url = "" if entry.url.start_with?("https://meet.example.com/")
      #   end
      #
      # @param entry_id [Integer] entry id
      # @yieldparam fields [ScheduleEntryFields] the entry's writable state, to mutate in place
      # @return [Hash] the updated schedule entry
      # @raise [ArgumentError] if no block is given
      def edit_entry(entry_id:)
        raise ArgumentError, "edit_entry requires a block" unless block_given?

        fields = fields_from_entry(get_entry(entry_id: entry_id))
        yield fields
        put_entry_fields(entry_id, fields)
      end

      private

      # Derives the writable state from a GET response.
      #
      # Every full-state value here is resent in the full-replace PUT, so every
      # one is validated before it is read. Ruby has no typed decoder between
      # the GET and this read (+get_entry+ returns a raw Hash), so the check is
      # explicit work here rather than something the layer below already did.
      # See {MergeSafe} and #576.
      #
      # The guards differ per field because the spec models the fields
      # differently:
      #
      # * +summary+, +starts_at+ and +ends_at+ are <tt>@required</tt> on the
      #   response and BC3 can never render them absent, null or blank —
      #   +Schedule::Entry#summary+ falls back to <tt>"Untitled"</tt>, and
      #   +starts_at+/+ends_at+ are NOT NULL columns every partial emits — so
      #   any of those shapes is a malformed response, not an empty value.
      # * +starts_at+/+ends_at+ are read as strings and round-tripped verbatim,
      #   never parsed: the wire value is a bare date (<tt>"2026-06-05"</tt>)
      #   for an all-day entry and a timestamp otherwise, and reformatting it
      #   would rewrite a value the caller never mentioned.
      # * +all_day+ is <tt>@required</tt> and NOT NULL DEFAULT false. It cannot
      #   be read with a truthiness test, because +false+ is the value the read
      #   most needs to admit; and defaulting a missing one to +false+ would
      #   silently convert an all-day event into a midnight-to-midnight timed
      #   one on a call that only changed the summary.
      # * +description+ is optional and nullable — the rich-text partial always
      #   sets the key but the value may be null — so absent or null is
      #   genuinely empty.
      #
      # The carve-outs are seeded for *reading* only, so a block can inspect
      # them before deciding; nothing here puts them on the wire. +url+ is
      # seeded from +join_url+, never from +url+ (see the module docs).
      # +highlighted+ is taken verbatim: it is optional, absent from the reduced
      # calendar partial +GetUpcomingSchedule+ renders, and — unlike every other
      # member — cannot reach the wire unless the caller assigns it, so there is
      # nothing to refuse a malformed value on behalf of.
      def fields_from_entry(entry)
        body = MergeSafe.require_hash(
          entry, record: RECORD, operation: "GetScheduleEntry", escape: ESCAPE_HATCH
        )
        ScheduleEntryFields.new(
          summary: required_string(body, "summary"),
          starts_at: required_string(body, "starts_at"),
          ends_at: required_string(body, "ends_at"),
          description: MergeSafe.writable_string(body, "description", record: RECORD, escape: ESCAPE_HATCH),
          all_day: MergeSafe.required_writable_boolean(body, "all_day", record: RECORD, escape: ESCAPE_HATCH),
          participant_ids: MergeSafe.writable_id_list(body, "participants", record: RECORD, escape: ESCAPE_HATCH),
          url: MergeSafe.writable_string(body, "join_url", record: RECORD, escape: ESCAPE_HATCH),
          highlighted: MergeSafe.writable_boolean(body, "highlighted", record: RECORD, escape: ESCAPE_HATCH)
        )
      end

      def required_string(body, key)
        MergeSafe.required_writable_string(body, key, record: RECORD, escape: ESCAPE_HATCH)
      end

      # PUTs the writable state via +replace_entry+.
      #
      # The five full-state members are always sent, empties included: a cleared
      # field travels as <tt>""</tt> rather than JSON null, and omitting it
      # would hand the clear back to the server's rebuild instead of stating it.
      # The carve-outs are splatted in only when addressed, so an untouched one
      # leaves no key at all and BC3 seeds it from the record it holds.
      #
      # As in Documents (#576), validating the *type* of what the caller assigns
      # is out of scope: a value the caller chose is not a value silently
      # substituted for one they asked to preserve. Normalising nil is not that
      # check — it is the addressedness contract, since a nil would be stripped
      # by +compact_params+ and turn a stated clear into an omission.
      def put_entry_fields(entry_id, fields)
        replace_entry(
          entry_id: entry_id,
          summary: caller_value(fields.summary, :summary),
          starts_at: caller_value(fields.starts_at, :starts_at),
          ends_at: caller_value(fields.ends_at, :ends_at),
          description: caller_value(fields.description, :description),
          all_day: caller_value(fields.all_day, :all_day),
          **addressed_carve_outs(fields)
        )
      end

      # The addressed carve-outs, as replace_entry keyword arguments. An
      # unaddressed one is absent from the Hash, so it is absent from the body.
      def addressed_carve_outs(fields)
        CARVE_OUTS.select { |name| fields.addressed?(name) } \
                  .to_h { |name| [ name, caller_value(fields.public_send(name), name) ] }
      end

      # Normalises a nil the caller left or assigned into that member's empty
      # spelling, and refuses it where the member has none.
      def caller_value(value, key)
        if value.nil?
          CLEARED.fetch(key) { raise nil_boolean(key) }
        else
          value
        end
      end

      def nil_boolean(key)
        UsageError.new(
          "schedule entry #{key} must be true or false, not nil",
          hint: "#{key} is a boolean with no empty value, and body compaction drops nil — " \
            "sending it would omit the field rather than state it, letting the server " \
            "decide. Assign true or false, or leave the member alone."
        )
      end
    end
  end
end
