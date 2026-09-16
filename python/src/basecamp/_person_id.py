"""Reading a person id off the wire the way Go's ``strconv.ParseInt(s, 10, 64)`` reads one.

BC3 serializes person ids as JSON strings in some responses and as JSON numbers
in others, and it spells the system actors -- ``LocalPerson``, whose id is
``"basecamp"`` or ``"campfire"`` -- in the same string field. Two places in this
SDK have to turn that string into a number: the pre-decode normalizer that every
response body walks (``generated/services/_base.py`` and its async twin) and the
flexible id reader in the recording-summary decode
(``services/_campfire_index.py``). The rule lives HERE so those two cannot drift
apart; they had two copies of it, and the copies disagreed.

So does the WALK that finds those objects (:func:`normalize_person_ids`), for the
same reason: the sync and async base services had a copy each, and which people a
walk finds is as much of the rule as what it does when it finds them.

The grammar is Go's and nothing looser. One optional ASCII ``+`` or ``-``, then
one or more ASCII digits, and nothing else. Python's built-in ``int()`` is the
wrong tool in four separate directions, each of them ACCEPTING -- and every
acceptance here mints an id BC3 never wrote, or collapses a real person onto the
system-actor ``0``:

* ``int()`` strips surrounding whitespace, so ``int(" 7")`` is 7 where Go's scan
  refuses the leading space (Go trims nothing).
* ``int()`` honours PEP 515 underscores, so ``int("1_0")`` is ten. Go allows
  ``_`` only at base 0; at base 10 it is not a digit.
* ``int()`` accepts every Unicode decimal digit, so ``int("７")`` -- fullwidth --
  is 7, and ``int("7৭")`` is 77. Go tests ``'0' <= c && c <= '9'`` on BYTES.
* ``int()`` is arbitrary-precision, so ``"18446744073709551616"`` parses instead
  of overflowing. The wire id is an ``int64``.

Not the same rule as the gid person id
--------------------------------------
``basecamp.mentions.person_id_from_sgid`` reads the id out of
``gid://bc3/Person/<id>`` under a DIFFERENT rule, deliberately: the reference
walks the bytes and refuses anything outside ``0-9`` BEFORE it parses
(``go/pkg/basecamp/mentions.go:252-256``) and then refuses ``id <= 0``
(``:258``), so it rejects a leading ``+`` that this function accepts. That shape
is CORRECT at that site -- relaxing it to this one reintroduces the ``+77``
defect PR #886 closed -- and it is WRONG here, because the Go lines governing
these two sites (``go/pkg/types/flexible_int64.go:34`` and
``go/pkg/basecamp/normalize.go:45``) have no pre-walk at all. Two rules, two
sites, on purpose: do not hoist either into the other, in either direction.
"""

from __future__ import annotations

import re
from collections.abc import MutableMapping
from enum import Enum
from itertools import islice
from typing import Any
from urllib.parse import urlsplit

_UINT64_MAX = 2**64 - 1
_INT64_MIN = -(2**63)
_INT64_MAX = 2**63 - 1


class Refusal(Enum):
    """Which way ``strconv.ParseInt`` refused.

    Keeping the two apart is the whole point: Go answers ``0`` to one and an
    error to the other, and no single "is this a number?" predicate can tell
    them apart, because WHICH refusal comes first depends on where in the string
    each disqualifying byte sits (see :func:`parse_int64`).
    """

    SYNTAX = "syntax"
    RANGE = "range"


def parse_int64(text: str) -> int | Refusal:
    """``strconv.ParseInt(text, 10, 64)``, scan order included.

    Returns the value, or the :class:`Refusal` Go would have raised.

    The subtlety worth the hand-rolled loop: ``ParseInt`` defers the magnitude
    to ``ParseUint``, which checks it INSIDE the scan and against ``uint64`` --
    not ``int64`` -- returning ``ErrRange`` the moment the accumulator would
    overflow, before it ever looks at the rest of the string. So the first
    disqualifying byte wins, and these two rows, one digit apart, refuse in
    opposite directions::

        "18446744073709551615x"   digits reach uint64 max, scan hits 'x' -> SYNTAX
        "18446744073709551616x"   the overflow fires first               -> RANGE

    Testing the whole string for well-formedness first ("is it all digits? then
    parse") gets that pair backwards, and backwards in the ACCEPTING direction:
    it answers the syntax refusal, which both callers read as the system actor,
    so an unreadable id silently becomes "basecamp" instead of failing.
    """
    # One optional ASCII sign. `+` IS accepted -- the `^-?\d+$` regex most ports
    # reach for is not this grammar.
    sign = text[:1]
    # An offset, not a slice: `text[1:]` copied the whole remaining id before the
    # scan even started. The scan itself stops at the first non-digit or at the
    # first overflow -- within ~20 significant digits -- so a long malformed id
    # cost a copy of itself for nothing. (Leading zeros carry no magnitude, so an
    # all-zero-padded id is still walked in full, exactly as Go walks it.)
    start = 1 if sign in ("+", "-") else 0
    # An empty digit run, with or without a sign, is a syntax error: "", "+", "-".
    if start == len(text):
        return Refusal.SYNTAX

    # `ParseUint`'s loop, byte for byte. `str.isdigit()` would be wrong here --
    # it is true for "৭" -- and so would `str.isdecimal()`; the test is on the
    # ASCII range itself.
    magnitude = 0
    for character in islice(text, start, None):
        if not ("0" <= character <= "9"):
            return Refusal.SYNTAX
        magnitude = magnitude * 10 + (ord(character) - ord("0"))
        # Checked every digit, not once at the end: Python would happily carry a
        # 200-digit accumulator, and the point is to refuse where Go refuses.
        if magnitude > _UINT64_MAX:
            return Refusal.RANGE

    # `ParseInt`'s own bound, applied to what `ParseUint` returned. The negative
    # side reaches one further than the positive: -9223372036854775808 is a
    # value, +9223372036854775808 is a range error.
    value = -magnitude if sign == "-" else magnitude
    if not (_INT64_MIN <= value <= _INT64_MAX):
        return Refusal.RANGE
    return value


def coerce_person_id(obj: MutableMapping[str, Any]) -> None:
    """Rewrite a Person-shaped object's string ``id`` in place, as Go's normalizer does.

    ``coercePersonID`` (``go/pkg/basecamp/normalize.go:40``), one outcome per
    ``ParseInt`` result:

    * a value becomes the ``int`` (``:58`` -- Go writes the CANONICAL digits
      back, so ``"+7"`` and ``"007"`` both land as ``7``, never held verbatim);
    * a syntax error is the non-numeric system-actor sentinel: id ``0`` with the
      original string preserved as ``system_label`` (``:66-67``);
    * a range error leaves the string UNTOUCHED (``:63``) so the reader refuses
      it. The readers that do refuse it are the ones with a Go struct behind
      them: ``services/_campfire_index._decoded_flexible_int64`` raises
      "overflows int64", and ``services/_merge_safe`` refuses a non-int id
      outright. Coercing it here instead -- which ``int()`` was doing -- hands
      back a Python arbitrary-precision int for a value that does not fit the
      wire's ``int64``, and every reader downstream then believes it.

    An object whose ``id`` is already a number, or absent, is left alone -- which
    is also what makes this IDEMPOTENT, and :func:`normalize_person_ids` depends
    on that. A second call after a value or a label finds an ``int`` and returns;
    a second call after a range refusal re-reads the same string and refuses it
    again, for the same reason.
    """
    raw_id = obj.get("id")
    if not isinstance(raw_id, str):
        return
    outcome = parse_int64(raw_id)
    if outcome is Refusal.RANGE:
        return
    if outcome is Refusal.SYNTAX:
        obj["system_label"] = raw_id
        obj["id"] = 0
        return
    obj["id"] = outcome


def normalize_person_ids(obj: Any, *, embedded_people: bool = False) -> None:
    """Normalize every Person-shaped object in a decoded response body, in place.

    This stands where ``normalizeEmbeddedPeopleJSON`` stands
    (``go/pkg/basecamp/normalize.go:117-134``), so it owns BOTH of the passes
    that function runs, and it finds a person TWO ways:

    1. **By ``personable_type``** (``normalizePersonIds``, ``:16-29``) -- any
       object at any depth carrying that key, whatever its value.
    2. **By structural position** (``normalizeEmbeddedPersonIds``, ``:83-104``)
       -- the ``creator`` object and each element of the ``participants`` array,
       on any object at any depth, WHETHER OR NOT it carries a
       ``personable_type``.

    The second pass is not redundant, and Go's own comment says why (``:78-82``):
    embedded creator and participant people frequently omit ``personable_type``,
    so the first pass skips exactly the payloads the second exists to fix.

    Missing it is observable HERE, where in most ports it would not be. Every
    port with a runtime decoder behind the normalizer -- Kotlin's
    ``FlexibleLongSerializer``, Swift's ``FlexibleInt``, Rust's ``flexible_i64``
    -- converts the string at read time whichever pass did or did not touch it.
    Python's only such decoder, ``_decoded_flexible_int64``, runs inside the
    recording-summary composite and nowhere else; the generated services hand
    back a plain ``dict`` with nothing between it and the wire. So on a
    notification or gauge body an un-normalized ``creator.id`` would reach the
    caller as the STRING it arrived as, where the reference reads the number.

    That missing decoder is also why this pass does NOT close every string person
    id off the wire, and does not try to -- see "WHERE PASS 2 RUNS" below. Off
    Go's two surfaces a string id in ``creator``, ``assignees`` or a schedule's
    ``participants`` stays a string, which ``services/_merge_safe``'s
    ``writable_id_list`` refuses: ``schedules.edit_entry`` refuses a body the
    reference accepts. That is decoder coverage rather than normalizer reach, and
    it is tracked in PR #913.

    ONE walk here where Go runs two, and the two are equivalent because
    :func:`coerce_person_id` is idempotent and both passes apply it unchanged.
    A decoded body is a tree, so each node is reached once per pass; the set of
    coerced nodes is the union of "has ``personable_type``" and "is the
    ``creator`` or a ``participants`` element of its parent", which is the same
    set either way; and a node in both sets is coerced twice under both schemes,
    a no-op the second time. Only the order differs, and idempotence is what
    makes order not matter. One walk rather than two keeps this off a second
    full traversal of every response body.

    WHERE PASS 2 RUNS, which is a correction. Pass 1 runs on every body, as it
    always has: an object that declares ``personable_type`` IS the Person
    projection. Pass 2 runs only when ``embedded_people`` is true, which the
    service base sets for exactly the endpoints Go calls
    ``normalizeEmbeddedPeopleJSON`` from -- ``decodeGaugePayload``
    (``gauges.go:170``) and the notification decoders
    (``my_notifications.go:171, 281, 296``). See :data:`EMBEDDED_PEOPLE_PATHS`.

    It used to run on every body, on the reasoning that wider is safe when it is
    wider in the accepting direction. On these keys it is not. ``creator`` and
    ``participants`` also sit on ``UpcomingScheduleEntry``, where they hold
    ``UpcomingSchedulePerson`` -- a plain ``int64`` id in the reference
    (``client.gen.go:4194-4198``) -- so a string there is a decode error in Go,
    and this turned it into person 0 with a ``system_label``: the SYSTEM ACTOR,
    for a body the reference refuses outright. Accepting direction, identity
    field, which is the class this work exists to remove.

    The cost of narrowing is real and is stated rather than hidden: the
    ``schedules.edit_entry`` write path described above refuses string
    ``participants`` ids again, because schedules is not one of Go's two
    surfaces and Python has no decoder standing in for ``FlexibleInt64``
    there. That is decoder coverage, not normalizer reach, and PR #913
    (card 42) owns it.
    """
    if isinstance(obj, list):
        for item in obj:
            normalize_person_ids(item, embedded_people=embedded_people)
        return
    if not isinstance(obj, dict):
        return
    # Pass 1's test is the KEY'S PRESENCE, whatever its value (`:19`), not that
    # it is a string naming a type.
    if "personable_type" in obj:
        coerce_person_id(obj)
    # Pass 2's type assertions are Go's, mirrored: a `creator` that is not an
    # object and a `participants` that is not an array are not people, so they
    # are skipped rather than coerced (`:85-95`). `dict` is what Go's
    # `.(map[string]any)` accepts and a list is not.
    if embedded_people:
        creator = obj.get("creator")
        if isinstance(creator, dict):
            coerce_person_id(creator)
        participants = obj.get("participants")
        if isinstance(participants, list):
            for participant in participants:
                if isinstance(participant, dict):
                    coerce_person_id(participant)
    # Both of Go's passes then recurse through every child, so a creator nested
    # under a comment under an event is reached at whatever depth it sits
    # (`:22-24`, `:96-102`).
    for value in obj.values():
        if isinstance(value, (dict, list)):
            normalize_person_ids(value, embedded_people=embedded_people)


#: The endpoints the reference runs the POSITIONAL pass over, and only those.
#:
#: ``normalizeEmbeddedPeopleJSON`` is a function in the reference, not a layer:
#: it is called from ``decodeGaugePayload`` (every gauge and needle body) and
#: from the notification decoders, and from nowhere else. Matched on the request
#: path because Go's own boundary is the call site.
#:
#: Each pattern is anchored to the END of the URL's path, and only the path is
#: matched -- never the host, never the query. An unanchored pattern over the
#: whole URL would let a base URL whose own path contained ``/gauge_needles/``
#: switch the pass on for every request.
EMBEDDED_PEOPLE_PATHS: tuple[re.Pattern[str], ...] = (
    re.compile(r"/my/readings\.json\Z"),  # GetMyNotifications
    re.compile(r"/my/readings/bubble_ups\.json\Z"),  # GetBubbleUps
    re.compile(r"/gauge_needles/\d+\Z"),  # GetGaugeNeedle, UpdateGaugeNeedle
    re.compile(r"/projects/\d+/gauge/needles\.json\Z"),  # ListGaugeNeedles, CreateGaugeNeedle
    re.compile(r"/reports/gauges\.json\Z"),  # ListGauges
)


def embedded_people_url(url: str) -> bool:
    """Whether ``url`` is one of the reference's two normalization surfaces."""
    path = urlsplit(url).path
    return any(pattern.search(path) for pattern in EMBEDDED_PEOPLE_PATHS)
