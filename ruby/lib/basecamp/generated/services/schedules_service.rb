# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Schedules operations
    #
    # @generated from OpenAPI spec
    class SchedulesService < BaseService

      # Get a single schedule entry by id.
      # @param entry_id [Integer] entry id ID
      # @return [Hash] response data
      def get_entry(entry_id:)
        with_operation(service: "schedules", operation: "get_entry", is_mutation: false, resource_id: entry_id) do
          http_get("/schedule_entries/#{entry_id}", operation: "GetScheduleEntry").json
        end
      end

      # Replace a schedule entry with a new complete representation.
      # @param entry_id [Integer] entry id ID
      # @param summary [String, nil] summary
      # @param starts_at [String] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] Replaces the entry's participants.
      #   
      #   Omitting this member preserves the current participants; sending an empty
      #   array clears them. That guarantee is BC3-side and recent: until
      #   basecamp/bc3#12425, `Schedules::EntriesController#update` called
      #   `replace_participants` unconditionally, so any update omitting the key —
      #   including the shape in BC3's own "Update a schedule entry" doc example —
      #   silently removed every participant and notified each one. The controller
      #   now guards on the request actually addressing participants.
      # @param all_day [Boolean, nil] Whether the entry occupies whole days rather than a time range.
      #   
      #   Not carved out, and the carve-out list is what makes that dangerous to
      #   forget: `schedule_entries.all_day` is NOT NULL with a `false` default, so
      #   omitting this member on a replace resets it — silently converting an
      #   all-day entry into a midnight-to-midnight timed one. The SDK's merge-safe
      #   update and edit resend it from the read-back for exactly this reason.
      #   
      #   Sending an explicit null is worse than omitting it: the column rejects
      #   NULL, so BC3 raises rather than falling back to the default. The same is
      #   true of highlighted.
      # @param notify [Boolean, nil] notify
      # @param url [String, nil] The entry's join link — a video-call URL or similar, up to 2500
      #   characters, validated as a URL when present.
      #   
      #   Omitting this member preserves the current join link; sending an empty
      #   string clears it. Read it back as `join_url`, never as `url`: the entry's
      #   `url` is its own Basecamp API URL, written by a partial that renders
      #   before this one, so BC3 emits the join link under a non-colliding key.
      #   Echoing the response's `url` into this member would write the API URL into
      #   the join link.
      # @param highlighted [Boolean, nil] Whether the entry is highlighted on the schedule.
      #   
      #   Omitting this member preserves the current highlight; sending false
      #   removes it. Preserved on omission because until basecamp/bc3#12502 the
      #   field was writable but never returned, so no caller could resend it.
      # @return [Hash] response data
      def replace_entry(entry_id:, starts_at:, ends_at:, summary: nil, description: nil, participant_ids: nil, all_day: nil, notify: nil, url: nil, highlighted: nil)
        with_operation(service: "schedules", operation: "replace_entry", is_mutation: true, resource_id: entry_id) do
          http_put("/schedule_entries/#{entry_id}", body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify, url: url, highlighted: highlighted)).json
        end
      end

      # Get a specific occurrence of a recurring schedule entry
      # @param entry_id [Integer] entry id ID
      # @param date [String] date ID
      # @return [Hash] response data
      def get_entry_occurrence(entry_id:, date:)
        with_operation(service: "schedules", operation: "get_entry_occurrence", is_mutation: false, resource_id: entry_id) do
          http_get("/schedule_entries/#{entry_id}/occurrences/#{date}", operation: "GetScheduleEntryOccurrence").json
        end
      end

      # Get a schedule
      # @param schedule_id [Integer] schedule id ID
      # @return [Hash] response data
      def get(schedule_id:)
        with_operation(service: "schedules", operation: "get", is_mutation: false, resource_id: schedule_id) do
          http_get("/schedules/#{schedule_id}", operation: "GetSchedule").json
        end
      end

      # Update schedule settings
      # @param schedule_id [Integer] schedule id ID
      # @param include_due_assignments [Boolean] include due assignments
      # @return [Hash] response data
      def update_settings(schedule_id:, include_due_assignments:)
        with_operation(service: "schedules", operation: "update_settings", is_mutation: true, resource_id: schedule_id) do
          http_put("/schedules/#{schedule_id}", body: compact_params(include_due_assignments: include_due_assignments)).json
        end
      end

      # List entries on a schedule
      # @param schedule_id [Integer] schedule id ID
      # @param status [String, nil] active|archived|trashed
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_entries(schedule_id:, status: nil, page: nil, max_items: nil)
        wrap_paginated(service: "schedules", operation: "list_entries", is_mutation: false, resource_id: schedule_id) do
          params = compact_query_params(status: status, page: page)
          paginate("/schedules/#{schedule_id}/entries.json", params: params, operation: "ListScheduleEntries", max_items: max_items)
        end
      end

      # Create a new schedule entry
      # @param schedule_id [Integer] schedule id ID
      # @param summary [String] summary
      # @param starts_at [String] starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param ends_at [String] ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z))
      # @param description [String, nil] description
      # @param participant_ids [Array, nil] participant ids
      # @param all_day [Boolean, nil] all day
      # @param notify [Boolean, nil] notify
      # @param url [String, nil] The entry's join link — a video-call URL or similar, up to 2500
      #   characters, validated as a URL when present. A scheme-less value is
      #   normalized to `https://`.
      #   
      #   Spell it `url` on the way in and read it back as `join_url`: the response
      #   key `url` is the entry's own Basecamp API URL, written by a partial that
      #   renders before this field, so BC3 emits the join link under a
      #   non-colliding name. Sending `join_url` instead is silently dropped by
      #   strong parameters — the create succeeds with no join link.
      #   
      #   Accepted on create since long before it was documented:
      #   `Schedules::Entries::BaseController#base_schedule_entry_params` permits it
      #   and `new_schedule_entry_params` passes it through unchanged for API
      #   requests. Modeling it only on ReplaceScheduleEntry forced callers into a
      #   three-request read-modify-write for a field the create already took — and
      #   create is the notifying write, so participants learned about a video call
      #   before its link existed.
      # @param highlighted [Boolean, nil] Whether the entry is highlighted on the schedule. Defaults to false.
      #   
      #   Do not send an explicit null: `schedule_entries.highlighted` is NOT NULL,
      #   so BC3 raises rather than falling back to the default. Omit it instead —
      #   every SDK's request compactor already drops unset members.
      # @param status [String, nil] Publication state at creation — `active|drafted`, defaulting to `active`
      #   for an API create.
      #   
      #   A top-level parameter, not part of the entry's attributes: `status` is a
      #   Recording column, so `wrap_parameters` leaves it outside the
      #   `schedule_entry` envelope and `Recording::StatusParam#status_param` reads
      #   it directly. On create it accepts `drafted`, `active`, `archived` or
      #   `trashed` and raises `ActionController::BadRequest` — a 400, not a 422 —
      #   for anything else; the two documented values are the two worth sending.
      #   
      #   Unlike messages and documents, schedule-entry drafts are not listed by
      #   GetMyDrafts.
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create_entry(schedule_id:, summary:, starts_at:, ends_at:, description: nil, participant_ids: nil, all_day: nil, notify: nil, url: nil, highlighted: nil, status: nil, subscriptions: nil, visible_to_clients: nil)
        with_operation(service: "schedules", operation: "create_entry", is_mutation: true, resource_id: schedule_id) do
          http_post("/schedules/#{schedule_id}/entries.json", body: compact_params(summary: summary, starts_at: starts_at, ends_at: ends_at, description: description, participant_ids: participant_ids, all_day: all_day, notify: notify, url: url, highlighted: highlighted, status: status, subscriptions: subscriptions, visible_to_clients: visible_to_clients)).json
        end
      end
    end
  end
end
