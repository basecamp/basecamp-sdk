# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class EventFeedService(BaseService):
    def poll_events(
        self,
        *,
        since: str | None = None,
        position: str | None = None,
        types: str | None = None,
        buckets: str | None = None,
        creators: str | None = None,
        performers: str | None = None,
        exclude_performers: str | None = None,
        actor_types: str | None = None,
    ) -> dict[str, Any]:
        """Poll the account event feed for events after a position (oldest first, strict event-id order, up to 100 per page).

        **Entry.** With neither `since` nor `position` the feed begins at the present
        (equivalent to `since=now`). `since=<event id>` starts after that id and
        `since=0` replays all served history back to the feed's epoch; `since` is a
        signed 64-bit integer written in decimal, or the literal `now`. `position`
        resumes from a token a previous page issued — signed, opaque, bound to the
        account and the filter set.

        **Pagination**: the body envelope, not the Link header. `position` is the
        durable cursor (persist it only after processing the page's events); `next`
        is an absolute continuation URL present only while this walk has more to
        serve. Not wired into the generic Link paginator — see the section note.

        **Errors.** 400 for a malformed position (resume with `since=`) or a malformed
        filter (the body names the filter; a position reset will not help) — both the
        flat `{error}` body. 409 (FeedFilterMismatchError) when the position was
        minted for a different filter set. 410 (FeedPositionGoneError) when the
        position predates the feed's epoch; follow its `resume` URL.

        Args:
            since: Entry point: a decimal event id (start after it; `0` replays served history back
                to the epoch), or the literal `now` (skip history). Mutually exclusive with
                `position` in practice; omit both to enter at the present.
            position: Resume token from a previous page's `position`. Opaque and signed; never
                constructed or parsed client-side.
            types: Comma-separated event types from the catalog (e.g.
                `message.created,comment.created`).
            buckets: Comma-separated bucket (project) ids, at most 100.
            creators: Comma-separated creator person ids, at most 100.
            performers: Comma-separated effective-performer ids (the agent on a delegated action,
                else the creator), at most 100. The literal `self` means the request's own effective
                actor and is resolved server-side before filtering.
            exclude_performers: Comma-separated effective-performer ids to exclude, at most 100;
                `self` as on `performers`. `exclude_performers=self` is the loop guard for an agent
                that acts on what it hears.
            actor_types: Comma-separated actor kinds: `agent`, `person`, or both. A filter, not a
                default — agent activity is real account activity.
        """
        return self._request(
            OperationInfo(service="eventfeed", operation="poll_events", is_mutation=False),
            "GET",
            "/events.json",
            params=self._compact(
                since=since,
                position=position,
                types=types,
                buckets=buckets,
                creators=creators,
                performers=performers,
                exclude_performers=exclude_performers,
                actor_types=actor_types,
            ),
            operation="PollEvents",
        )

    def create_stream_ticket(self) -> dict[str, Any]:
        """Mint a short-lived stream ticket and the exact WebSocket URL to open a live event stream with.

        The ticket is signed and lives about two minutes; the response's `url` is
        the one to connect to. Connect to `url` verbatim — never assemble the WebSocket URL (scheme, host,
        path, account prefix) client-side; the topology is the server's to change.
        Mint a fresh ticket for every connection attempt: tickets expire and are not
        refreshed by an open socket. The mint is a stateless signed capability with
        no server-side consumption, so a replayed POST is harmless and the operation
        is marked idempotent (safe to retry) — deliberately not a claim that two
        mints return the same ticket. The ticket is a replayable bearer credential
        within its window; `ticket` and `url` are redacted from SDK logs.

        Serves agent principals as well as people: an agent's client-credentials
        token can mint tickets for its own live stream. A ticket minted on a
        delegated request carries its agent, so `self` on the socket it opens
        resolves to that agent.
        """
        return self._request(
            OperationInfo(service="eventfeed", operation="create_stream_ticket", is_mutation=True),
            "POST",
            "/events/stream_ticket.json",
            operation="CreateStreamTicket",
        )

    def poll_inbox(
        self,
        *,
        since: str | None = None,
        position: str | None = None,
        reasons: str | None = None,
        types: str | None = None,
        buckets: str | None = None,
    ) -> dict[str, Any]:
        """Poll the authenticated agent's inbox for the items that addressed it (oldest first, strict item-id order); people receive 403.

        The inbox is the low-noise "someone addressed you" lane as its own resource
        rather than a filter over the account feed. **Agents only for now**: any
        other principal receives 403.

        An item is a first-class delivery with its own identity: one event can
        address the same principal for several reasons, and each reason is its own
        item. Deduplicate by `addressing_id`, never by event id. Items are never
        self-addressed, are kept for 30 days, and are dropped at read time when the
        event is no longer readable.

        **Entry**: `since=0` replays the earliest retained items, `since=now` enters
        at the present, `position` resumes. Inbox positions are bound to the
        account, the principal, and the filter set, and are never interchangeable
        with feed positions.

        **Pagination**: the body envelope (`items`, `position`, `next`), exactly as
        PollEvents — not the Link header, and not the generic paginator.

        **Errors** follow PollEvents, except that 410 (FeedPositionGoneError) here
        means the position fell behind the retention window: `epoch_after_id` is
        absent and `resume` re-enters at `since=0`, the earliest retained item.

        Args:
            since: Entry point: `0` (earliest retained), `now` (present), or a decimal item id to
                start after.
            position: Resume token from a previous inbox page's `position`.
            reasons: Comma-separated addressing reasons: `mentioned`, `assigned`, `subscribed`,
                `watched`, `pinged`, `boosted`.
            types: Comma-separated event types, as a narrowing filter.
            buckets: Comma-separated bucket ids, as a narrowing filter (at most 100).
        """
        return self._request(
            OperationInfo(service="eventfeed", operation="poll_inbox", is_mutation=False),
            "GET",
            "/inbox.json",
            params=self._compact(since=since, position=position, reasons=reasons, types=types, buckets=buckets),
            operation="PollInbox",
        )


class AsyncEventFeedService(AsyncBaseService):
    async def poll_events(
        self,
        *,
        since: str | None = None,
        position: str | None = None,
        types: str | None = None,
        buckets: str | None = None,
        creators: str | None = None,
        performers: str | None = None,
        exclude_performers: str | None = None,
        actor_types: str | None = None,
    ) -> dict[str, Any]:
        """Poll the account event feed for events after a position (oldest first, strict event-id order, up to 100 per page).

        **Entry.** With neither `since` nor `position` the feed begins at the present
        (equivalent to `since=now`). `since=<event id>` starts after that id and
        `since=0` replays all served history back to the feed's epoch; `since` is a
        signed 64-bit integer written in decimal, or the literal `now`. `position`
        resumes from a token a previous page issued — signed, opaque, bound to the
        account and the filter set.

        **Pagination**: the body envelope, not the Link header. `position` is the
        durable cursor (persist it only after processing the page's events); `next`
        is an absolute continuation URL present only while this walk has more to
        serve. Not wired into the generic Link paginator — see the section note.

        **Errors.** 400 for a malformed position (resume with `since=`) or a malformed
        filter (the body names the filter; a position reset will not help) — both the
        flat `{error}` body. 409 (FeedFilterMismatchError) when the position was
        minted for a different filter set. 410 (FeedPositionGoneError) when the
        position predates the feed's epoch; follow its `resume` URL.

        Args:
            since: Entry point: a decimal event id (start after it; `0` replays served history back
                to the epoch), or the literal `now` (skip history). Mutually exclusive with
                `position` in practice; omit both to enter at the present.
            position: Resume token from a previous page's `position`. Opaque and signed; never
                constructed or parsed client-side.
            types: Comma-separated event types from the catalog (e.g.
                `message.created,comment.created`).
            buckets: Comma-separated bucket (project) ids, at most 100.
            creators: Comma-separated creator person ids, at most 100.
            performers: Comma-separated effective-performer ids (the agent on a delegated action,
                else the creator), at most 100. The literal `self` means the request's own effective
                actor and is resolved server-side before filtering.
            exclude_performers: Comma-separated effective-performer ids to exclude, at most 100;
                `self` as on `performers`. `exclude_performers=self` is the loop guard for an agent
                that acts on what it hears.
            actor_types: Comma-separated actor kinds: `agent`, `person`, or both. A filter, not a
                default — agent activity is real account activity.
        """
        return await self._request(
            OperationInfo(service="eventfeed", operation="poll_events", is_mutation=False),
            "GET",
            "/events.json",
            params=self._compact(
                since=since,
                position=position,
                types=types,
                buckets=buckets,
                creators=creators,
                performers=performers,
                exclude_performers=exclude_performers,
                actor_types=actor_types,
            ),
            operation="PollEvents",
        )

    async def create_stream_ticket(self) -> dict[str, Any]:
        """Mint a short-lived stream ticket and the exact WebSocket URL to open a live event stream with.

        The ticket is signed and lives about two minutes; the response's `url` is
        the one to connect to. Connect to `url` verbatim — never assemble the WebSocket URL (scheme, host,
        path, account prefix) client-side; the topology is the server's to change.
        Mint a fresh ticket for every connection attempt: tickets expire and are not
        refreshed by an open socket. The mint is a stateless signed capability with
        no server-side consumption, so a replayed POST is harmless and the operation
        is marked idempotent (safe to retry) — deliberately not a claim that two
        mints return the same ticket. The ticket is a replayable bearer credential
        within its window; `ticket` and `url` are redacted from SDK logs.

        Serves agent principals as well as people: an agent's client-credentials
        token can mint tickets for its own live stream. A ticket minted on a
        delegated request carries its agent, so `self` on the socket it opens
        resolves to that agent.
        """
        return await self._request(
            OperationInfo(service="eventfeed", operation="create_stream_ticket", is_mutation=True),
            "POST",
            "/events/stream_ticket.json",
            operation="CreateStreamTicket",
        )

    async def poll_inbox(
        self,
        *,
        since: str | None = None,
        position: str | None = None,
        reasons: str | None = None,
        types: str | None = None,
        buckets: str | None = None,
    ) -> dict[str, Any]:
        """Poll the authenticated agent's inbox for the items that addressed it (oldest first, strict item-id order); people receive 403.

        The inbox is the low-noise "someone addressed you" lane as its own resource
        rather than a filter over the account feed. **Agents only for now**: any
        other principal receives 403.

        An item is a first-class delivery with its own identity: one event can
        address the same principal for several reasons, and each reason is its own
        item. Deduplicate by `addressing_id`, never by event id. Items are never
        self-addressed, are kept for 30 days, and are dropped at read time when the
        event is no longer readable.

        **Entry**: `since=0` replays the earliest retained items, `since=now` enters
        at the present, `position` resumes. Inbox positions are bound to the
        account, the principal, and the filter set, and are never interchangeable
        with feed positions.

        **Pagination**: the body envelope (`items`, `position`, `next`), exactly as
        PollEvents — not the Link header, and not the generic paginator.

        **Errors** follow PollEvents, except that 410 (FeedPositionGoneError) here
        means the position fell behind the retention window: `epoch_after_id` is
        absent and `resume` re-enters at `since=0`, the earliest retained item.

        Args:
            since: Entry point: `0` (earliest retained), `now` (present), or a decimal item id to
                start after.
            position: Resume token from a previous inbox page's `position`.
            reasons: Comma-separated addressing reasons: `mentioned`, `assigned`, `subscribed`,
                `watched`, `pinged`, `boosted`.
            types: Comma-separated event types, as a narrowing filter.
            buckets: Comma-separated bucket ids, as a narrowing filter (at most 100).
        """
        return await self._request(
            OperationInfo(service="eventfeed", operation="poll_inbox", is_mutation=False),
            "GET",
            "/inbox.json",
            params=self._compact(since=since, position=position, reasons=reasons, types=types, buckets=buckets),
            operation="PollInbox",
        )
