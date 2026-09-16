# frozen_string_literal: true

module Basecamp
  module Services
    # Service for EventFeed operations
    #
    # @generated from OpenAPI spec
    class EventFeedService < BaseService

      # Poll the account event feed for events after a position (oldest first, strict event-id order, up to 100 per page).
      # @param since [String, nil] Entry point: a decimal event id (start after it; `0` replays served
      #   history back to the epoch), or the literal `now` (skip history). Mutually
      #   exclusive with `position` in practice; omit both to enter at the present.
      # @param position [String, nil] Resume token from a previous page's `position`. Opaque and signed; never
      #   constructed or parsed client-side.
      # @param types [String, nil] Comma-separated event types from the catalog (e.g. `message.created,comment.created`).
      # @param buckets [String, nil] Comma-separated bucket (project) ids, at most 100.
      # @param creators [String, nil] Comma-separated creator person ids, at most 100.
      # @param performers [String, nil] Comma-separated effective-performer ids (the agent on a delegated action,
      #   else the creator), at most 100. The literal `self` means the request's own
      #   effective actor and is resolved server-side before filtering.
      # @param exclude_performers [String, nil] Comma-separated effective-performer ids to exclude, at most 100; `self`
      #   as on `performers`. `exclude_performers=self` is the loop guard for an
      #   agent that acts on what it hears.
      # @param actor_types [String, nil] Comma-separated actor kinds: `agent`, `person`, or both. A filter, not a
      #   default — agent activity is real account activity.
      # @return [Hash] response data
      def poll_events(since: nil, position: nil, types: nil, buckets: nil, creators: nil, performers: nil, exclude_performers: nil, actor_types: nil)
        with_operation(service: "eventfeed", operation: "poll_events", is_mutation: false) do
          http_get("/events.json", params: compact_query_params(since: since, position: position, types: types, buckets: buckets, creators: creators, performers: performers, exclude_performers: exclude_performers, actor_types: actor_types), operation: "PollEvents").json(operation: "PollEvents")
        end
      end

      # Mint a short-lived stream ticket and the exact WebSocket URL to open a live event stream with.
      # @return [Hash] response data
      def create_stream_ticket()
        with_operation(service: "eventfeed", operation: "create_stream_ticket", is_mutation: true) do
          http_post("/events/stream_ticket.json").json(operation: "CreateStreamTicket")
        end
      end

      # Poll the authenticated agent's inbox for the items that addressed it (oldest first, strict item-id order); people receive 403.
      # @param since [String, nil] Entry point: `0` (earliest retained), `now` (present), or a decimal item
      #   id to start after.
      # @param position [String, nil] Resume token from a previous inbox page's `position`.
      # @param reasons [String, nil] Comma-separated addressing reasons: `mentioned`, `assigned`, `subscribed`,
      #   `watched`, `pinged`, `boosted`.
      # @param types [String, nil] Comma-separated event types, as a narrowing filter.
      # @param buckets [String, nil] Comma-separated bucket ids, as a narrowing filter (at most 100).
      # @return [Hash] response data
      def poll_inbox(since: nil, position: nil, reasons: nil, types: nil, buckets: nil)
        with_operation(service: "eventfeed", operation: "poll_inbox", is_mutation: false) do
          http_get("/inbox.json", params: compact_query_params(since: since, position: position, reasons: reasons, types: types, buckets: buckets), operation: "PollInbox").json(operation: "PollInbox")
        end
      end
    end
  end
end
