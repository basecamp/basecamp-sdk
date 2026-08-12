# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SchedulesService(BaseService):
    def get_entry(self, *, entry_id: int) -> dict[str, Any]:
        """Get a single schedule entry by id.
        Note: Recurring entries will redirect (302) to their recordable URL.
        Use GetScheduleEntryOccurrence for recurring entries instead.

        Args:
            entry_id: The entry id.
        """
        return self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
            operation="GetScheduleEntry",
        )

    def replace_entry(
        self,
        *,
        entry_id: int,
        starts_at: str,
        ends_at: str,
        summary: str | None = None,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
    ) -> dict[str, Any]:
        """Replace a schedule entry with a new complete representation.
        The request body is the entry's full writable state: a writable field
        omitted from the request is cleared server-side, because BC3 builds a
        brand-new Schedule::Entry from the permitted params and swaps the recordable
        wholesale.
        Three writable fields are carved out of that swap and preserved when the
        request does not address them — participant_ids, url and highlighted, as
        declared by preservedOnOmission. Each is a field a caller could not safely
        resend from a read-back, which is why the guard is server-side: the response
        carries participants (objects, not ids) and join_url (the entry's own url
        key is its Basecamp API URL, a different value under a colliding name).
        Addressing one applies it normally, so participant_ids: [] clears the
        participants, url: "" clears the join link and highlighted: false removes
        the highlight.
        starts_at and ends_at are required: Schedule::Entry validates their presence
        and Recording validates the associated recordable on update, so omitting
        either is a 422 rather than a clear. summary carries no validation — omit it
        and the entry reads back as "Untitled" (Schedule::Entry#summary falls back
        when blank).
        Recurring entries are unreachable here. ensure_non_recurring_event redirects
        both show and update to the entry's occurrence, so this operation serves
        non-recurring entries only; read a recurring entry through
        GetScheduleEntryOccurrence.
        time_zone_name, recurs_until and recurrence_schedule are not modeled: BC3
        forces all three to nil for a non-recurring entry, which is the only kind
        this route serves.
        Subscribers follow the same carve-out logic one level up. A drafted entry
        keeps its current subscribers when the request addresses neither
        subscriptions, notify, nor the participant parameters.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current entry and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            entry_id: The entry id.
            starts_at: The entry's start, as a bare date ("2026-06-01") for an all-day entry or a
                full timestamp otherwise. Same rule as CreateScheduleEntry: send it verbatim, never
                parsed and re-rendered.
            ends_at: The entry's end. See starts_at for the date-vs-timestamp rule.
            summary: The summary.
            description: The description.
            participant_ids: Replaces the entry's participants. Omitting this member preserves the
                current participants; sending an empty array clears them. That guarantee is BC3-side
                and recent: until basecamp/bc3#12425, `Schedules::EntriesController#update` called
                `replace_participants` unconditionally, so any update omitting the key — including
                the shape in BC3's own "Update a schedule entry" doc example — silently removed
                every participant and notified each one. The controller now guards on the request
                actually addressing participants.
            all_day: Whether the entry occupies whole days rather than a time range. Not carved out,
                and the carve-out list is what makes that dangerous to forget:
                `schedule_entries.all_day` is NOT NULL with a `false` default, so omitting this
                member on a replace resets it — silently converting an all-day entry into a
                midnight-to-midnight timed one. The SDK's merge-safe update and edit resend it from
                the read-back for exactly this reason. Sending an explicit null is worse than
                omitting it: the column rejects NULL, so BC3 raises rather than falling back to the
                default. The same is true of highlighted.
            notify: The notify.
            url: The entry's join link — a video-call URL or similar, up to 2500 characters,
                validated as a URL when present. Omitting this member preserves the current join
                link; sending an empty string clears it. Read it back as `join_url`, never as `url`:
                the entry's `url` is its own Basecamp API URL, written by a partial that renders
                before this one, so BC3 emits the join link under a non-colliding key. Echoing the
                response's `url` into this member would write the API URL into the join link.
            highlighted: Whether the entry is highlighted on the schedule. Omitting this member
                preserves the current highlight; sending false removes it. Preserved on omission
                because until basecamp/bc3#12502 the field was writable but never returned, so no
                caller could resend it.
        """
        return self._request(
            OperationInfo(service="schedules", operation="replace_entry", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/schedule_entries/{entry_id}",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                url=url,
                highlighted=highlighted,
            ),
            operation="ReplaceScheduleEntry",
        )

    def get_entry_occurrence(self, *, entry_id: int, date: str) -> dict[str, Any]:
        """Get a specific occurrence of a recurring schedule entry.

        Args:
            entry_id: The entry id.
            date: The date.
        """
        return self._request(
            OperationInfo(
                service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=entry_id
            ),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
            operation="GetScheduleEntryOccurrence",
        )

    def get(self, *, schedule_id: int) -> dict[str, Any]:
        """Get a schedule.

        Args:
            schedule_id: The schedule id.
        """
        return self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
            operation="GetSchedule",
        )

    def update_settings(self, *, schedule_id: int, include_due_assignments: bool) -> dict[str, Any]:
        """Update schedule settings.

        Args:
            schedule_id: The schedule id.
            include_due_assignments: The include due assignments.
        """
        return self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    def list_entries(
        self, *, schedule_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List entries on a schedule.

        Args:
            schedule_id: The schedule id.
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListScheduleEntries",
        )

    def create_entry(
        self,
        *,
        schedule_id: int,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new schedule entry.

        Args:
            schedule_id: The schedule id.
            summary: The summary.
            starts_at: The entry's start, as a bare date ("2026-06-01") for an all-day entry or a
                full timestamp ("2026-06-01T09:00:00Z") otherwise — the same two forms the response
                renders, and the same two ReplaceScheduleEntry accepts. Create and replace share one
                permit list: `Schedules::Entries::BaseController#base_schedule_entry_params` is what
                both `new_schedule_entry_params` and `update_schedule_entry_params` call, and
                Schedule::Entry does no format-specific parsing of either bound, so whatever one
                operation takes the other takes too. Treat the value as opaque and send it verbatim.
                Parsing it into a date-time type and re-rendering rewrites an all-day entry's bounds
                into midnight timestamps, which is why every SDK models it as a string.
            ends_at: The entry's end. See starts_at for the date-vs-timestamp rule.
            description: The description.
            participant_ids: The participant ids.
            all_day: The all day.
            notify: The notify.
            url: The entry's join link — a video-call URL or similar, up to 2500 characters,
                validated as a URL when present. A scheme-less value is normalized to `https://`.
                Spell it `url` on the way in and read it back as `join_url`: the response key `url`
                is the entry's own Basecamp API URL, written by a partial that renders before this
                field, so BC3 emits the join link under a non-colliding name. Sending `join_url`
                instead is silently dropped by strong parameters — the create succeeds with no join
                link. Accepted on create since long before it was documented:
                `Schedules::Entries::BaseController#base_schedule_entry_params` permits it and
                `new_schedule_entry_params` passes it through unchanged for API requests. Modeling
                it only on ReplaceScheduleEntry forced callers into a three-request
                read-modify-write for a field the create already took — and create is the notifying
                write, so participants learned about a video call before its link existed.
            highlighted: Whether the entry is highlighted on the schedule. Defaults to false. Do not
                send an explicit null: `schedule_entries.highlighted` is NOT NULL, so BC3 raises
                rather than falling back to the default. Omit it instead — every SDK's request
                compactor already drops unset members.
            status: Publication state at creation — `active|drafted`, defaulting to `active` for an
                API create. A top-level parameter, not part of the entry's attributes: `status` is a
                Recording column, so `wrap_parameters` leaves it outside the `schedule_entry`
                envelope and `Recording::StatusParam#status_param` reads it directly. On create it
                accepts `drafted`, `active`, `archived` or `trashed` and raises
                `ActionController::BadRequest` — a 400, not a 422 — for anything else; the two
                documented values are the two worth sending. Unlike messages and documents,
                schedule-entry drafts are not listed by GetMyDrafts.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(service="schedules", operation="create_entry", is_mutation=True, resource_id=schedule_id),
            "POST",
            f"/schedules/{schedule_id}/entries.json",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                url=url,
                highlighted=highlighted,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateScheduleEntry",
        )


class AsyncSchedulesService(AsyncBaseService):
    async def get_entry(self, *, entry_id: int) -> dict[str, Any]:
        """Get a single schedule entry by id.
        Note: Recurring entries will redirect (302) to their recordable URL.
        Use GetScheduleEntryOccurrence for recurring entries instead.

        Args:
            entry_id: The entry id.
        """
        return await self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
            operation="GetScheduleEntry",
        )

    async def replace_entry(
        self,
        *,
        entry_id: int,
        starts_at: str,
        ends_at: str,
        summary: str | None = None,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
    ) -> dict[str, Any]:
        """Replace a schedule entry with a new complete representation.
        The request body is the entry's full writable state: a writable field
        omitted from the request is cleared server-side, because BC3 builds a
        brand-new Schedule::Entry from the permitted params and swaps the recordable
        wholesale.
        Three writable fields are carved out of that swap and preserved when the
        request does not address them — participant_ids, url and highlighted, as
        declared by preservedOnOmission. Each is a field a caller could not safely
        resend from a read-back, which is why the guard is server-side: the response
        carries participants (objects, not ids) and join_url (the entry's own url
        key is its Basecamp API URL, a different value under a colliding name).
        Addressing one applies it normally, so participant_ids: [] clears the
        participants, url: "" clears the join link and highlighted: false removes
        the highlight.
        starts_at and ends_at are required: Schedule::Entry validates their presence
        and Recording validates the associated recordable on update, so omitting
        either is a 422 rather than a clear. summary carries no validation — omit it
        and the entry reads back as "Untitled" (Schedule::Entry#summary falls back
        when blank).
        Recurring entries are unreachable here. ensure_non_recurring_event redirects
        both show and update to the entry's occurrence, so this operation serves
        non-recurring entries only; read a recurring entry through
        GetScheduleEntryOccurrence.
        time_zone_name, recurs_until and recurrence_schedule are not modeled: BC3
        forces all three to nil for a non-recurring entry, which is the only kind
        this route serves.
        Subscribers follow the same carve-out logic one level up. A drafted entry
        keeps its current subscribers when the request addresses neither
        subscriptions, notify, nor the participant parameters.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current entry and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            entry_id: The entry id.
            starts_at: The entry's start, as a bare date ("2026-06-01") for an all-day entry or a
                full timestamp otherwise. Same rule as CreateScheduleEntry: send it verbatim, never
                parsed and re-rendered.
            ends_at: The entry's end. See starts_at for the date-vs-timestamp rule.
            summary: The summary.
            description: The description.
            participant_ids: Replaces the entry's participants. Omitting this member preserves the
                current participants; sending an empty array clears them. That guarantee is BC3-side
                and recent: until basecamp/bc3#12425, `Schedules::EntriesController#update` called
                `replace_participants` unconditionally, so any update omitting the key — including
                the shape in BC3's own "Update a schedule entry" doc example — silently removed
                every participant and notified each one. The controller now guards on the request
                actually addressing participants.
            all_day: Whether the entry occupies whole days rather than a time range. Not carved out,
                and the carve-out list is what makes that dangerous to forget:
                `schedule_entries.all_day` is NOT NULL with a `false` default, so omitting this
                member on a replace resets it — silently converting an all-day entry into a
                midnight-to-midnight timed one. The SDK's merge-safe update and edit resend it from
                the read-back for exactly this reason. Sending an explicit null is worse than
                omitting it: the column rejects NULL, so BC3 raises rather than falling back to the
                default. The same is true of highlighted.
            notify: The notify.
            url: The entry's join link — a video-call URL or similar, up to 2500 characters,
                validated as a URL when present. Omitting this member preserves the current join
                link; sending an empty string clears it. Read it back as `join_url`, never as `url`:
                the entry's `url` is its own Basecamp API URL, written by a partial that renders
                before this one, so BC3 emits the join link under a non-colliding key. Echoing the
                response's `url` into this member would write the API URL into the join link.
            highlighted: Whether the entry is highlighted on the schedule. Omitting this member
                preserves the current highlight; sending false removes it. Preserved on omission
                because until basecamp/bc3#12502 the field was writable but never returned, so no
                caller could resend it.
        """
        return await self._request(
            OperationInfo(service="schedules", operation="replace_entry", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/schedule_entries/{entry_id}",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                url=url,
                highlighted=highlighted,
            ),
            operation="ReplaceScheduleEntry",
        )

    async def get_entry_occurrence(self, *, entry_id: int, date: str) -> dict[str, Any]:
        """Get a specific occurrence of a recurring schedule entry.

        Args:
            entry_id: The entry id.
            date: The date.
        """
        return await self._request(
            OperationInfo(
                service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=entry_id
            ),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
            operation="GetScheduleEntryOccurrence",
        )

    async def get(self, *, schedule_id: int) -> dict[str, Any]:
        """Get a schedule.

        Args:
            schedule_id: The schedule id.
        """
        return await self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
            operation="GetSchedule",
        )

    async def update_settings(self, *, schedule_id: int, include_due_assignments: bool) -> dict[str, Any]:
        """Update schedule settings.

        Args:
            schedule_id: The schedule id.
            include_due_assignments: The include due assignments.
        """
        return await self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    async def list_entries(
        self, *, schedule_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List entries on a schedule.

        Args:
            schedule_id: The schedule id.
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListScheduleEntries",
        )

    async def create_entry(
        self,
        *,
        schedule_id: int,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new schedule entry.

        Args:
            schedule_id: The schedule id.
            summary: The summary.
            starts_at: The entry's start, as a bare date ("2026-06-01") for an all-day entry or a
                full timestamp ("2026-06-01T09:00:00Z") otherwise — the same two forms the response
                renders, and the same two ReplaceScheduleEntry accepts. Create and replace share one
                permit list: `Schedules::Entries::BaseController#base_schedule_entry_params` is what
                both `new_schedule_entry_params` and `update_schedule_entry_params` call, and
                Schedule::Entry does no format-specific parsing of either bound, so whatever one
                operation takes the other takes too. Treat the value as opaque and send it verbatim.
                Parsing it into a date-time type and re-rendering rewrites an all-day entry's bounds
                into midnight timestamps, which is why every SDK models it as a string.
            ends_at: The entry's end. See starts_at for the date-vs-timestamp rule.
            description: The description.
            participant_ids: The participant ids.
            all_day: The all day.
            notify: The notify.
            url: The entry's join link — a video-call URL or similar, up to 2500 characters,
                validated as a URL when present. A scheme-less value is normalized to `https://`.
                Spell it `url` on the way in and read it back as `join_url`: the response key `url`
                is the entry's own Basecamp API URL, written by a partial that renders before this
                field, so BC3 emits the join link under a non-colliding name. Sending `join_url`
                instead is silently dropped by strong parameters — the create succeeds with no join
                link. Accepted on create since long before it was documented:
                `Schedules::Entries::BaseController#base_schedule_entry_params` permits it and
                `new_schedule_entry_params` passes it through unchanged for API requests. Modeling
                it only on ReplaceScheduleEntry forced callers into a three-request
                read-modify-write for a field the create already took — and create is the notifying
                write, so participants learned about a video call before its link existed.
            highlighted: Whether the entry is highlighted on the schedule. Defaults to false. Do not
                send an explicit null: `schedule_entries.highlighted` is NOT NULL, so BC3 raises
                rather than falling back to the default. Omit it instead — every SDK's request
                compactor already drops unset members.
            status: Publication state at creation — `active|drafted`, defaulting to `active` for an
                API create. A top-level parameter, not part of the entry's attributes: `status` is a
                Recording column, so `wrap_parameters` leaves it outside the `schedule_entry`
                envelope and `Recording::StatusParam#status_param` reads it directly. On create it
                accepts `drafted`, `active`, `archived` or `trashed` and raises
                `ActionController::BadRequest` — a 400, not a 422 — for anything else; the two
                documented values are the two worth sending. Unlike messages and documents,
                schedule-entry drafts are not listed by GetMyDrafts.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(service="schedules", operation="create_entry", is_mutation=True, resource_id=schedule_id),
            "POST",
            f"/schedules/{schedule_id}/entries.json",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                url=url,
                highlighted=highlighted,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateScheduleEntry",
        )
