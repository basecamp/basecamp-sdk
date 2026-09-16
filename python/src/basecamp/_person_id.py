"""Reading a person id off the wire the way Go's ``strconv.ParseInt(s, 10, 64)`` reads one.

BC3 serializes person ids as JSON strings in some responses and as JSON numbers
in others, and it spells the system actors -- ``LocalPerson``, whose id is
``"basecamp"`` or ``"campfire"`` -- in the same string field. Three places in
this SDK have to turn that string into a number: the pre-decode normalizer that
every response body walks (``generated/services/_base.py`` and its async twin),
the flexible id reader in the recording-summary decode
(``services/_campfire_index.py``), and the merge-safe composites' id-list guard
(``services/_merge_safe.py``). The rule lives HERE so they cannot drift apart;
the first two had two copies of it, and the copies disagreed.

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

import json
import re
from collections.abc import Iterable, Mapping, MutableMapping
from enum import Enum
from itertools import islice
from typing import Any
from urllib.parse import urlsplit

from basecamp._security import truncate as _truncate
from basecamp.errors import ApiError

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
      "overflows int64", and ``services/_merge_safe`` refuses a RANGE
      refusal as not a person id. Coercing it here instead -- which ``int()``
      was doing -- hands back a Python arbitrary-precision int for a value that
      does not fit the wire's ``int64``, and every reader downstream then
      believes it.

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
    The generated services hand back a plain ``dict``; the one decoder standing
    in for ``FlexibleInt64`` there is :func:`decode_person_id_sites`, which reads
    the id but writes no ``system_label``. So on a notification or gauge body
    only this pass gives an untagged system actor its label, as the reference's
    does.

    This pass does NOT close every string person id off the wire, and does not
    try to -- see "WHERE PASS 2 RUNS" below. Off Go's two surfaces a string id
    in ``creator``, ``assignees`` or a schedule's ``participants`` is decoder
    coverage rather than normalizer reach, and :func:`decode_person_id_sites`
    closes it at exactly the ``Person`` sites the schema declares.

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

    Narrowing costs no coverage: schedules is not one of Go's two surfaces, and
    a string ``participants`` id on a ``Person`` site there is read by
    :func:`decode_person_id_sites`, as Go's typed decode reads it (SPEC.md
    section 10, "Person Ids Off the Wire").
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


def decode_person_id_sites(
    body: Any,
    table: Mapping[str, Iterable[tuple[str, ...]]],
    operation: str | None,
    *,
    followed_page: bool = False,
    only_under: str | None = None,
) -> None:
    """Read each person id at ``operation``'s sites in ``table`` as Go's ``types.FlexibleInt64``, in place.

    Go types ``Person.id`` as ``types.FlexibleInt64``
    (``go/pkg/types/flexible_int64.go``) and every generated Go service decodes
    its body through ``Parse<Op>Response`` first, so an untagged ``{"id": "7"}``
    comes back as ``7`` wherever a ``Person`` sits. These dicts have no decoder,
    so this stands in for that one field, at exactly the sites the generator
    found by the ``x-go-type`` marker (``generated/services/_person_id_sites.py``)
    -- never by key name, which would reach the plain-int64 people
    (``UpcomingSchedulePerson`` and co.) where Go refuses a string.

    Runs AFTER :func:`normalize_person_ids`, on the same body the paths are
    relative to. Per id (``flexible_int64.go:28-63``):

    * a string is ``ParseInt``: a value becomes the ``int``; a syntax refusal
      becomes ``0`` with NO ``system_label`` (the decoder writes none, ``:47``);
      a range refusal fails the read (``:44``);
    * an ``int`` within int64 stays; any other JSON value -- float, out-of-range
      int, ``null``, bool, array, object -- fails the read (``:55-60``).

    A person that is not an object, or has no ``id`` key, and a container of
    the wrong shape, are left alone: Go zero-fills or refuses those as part of a
    whole-body typed decode this SDK does not do for any field. Idempotent,
    because every surviving id is an ``int``.

    FOLLOWED PAGES are not ``Parse<Op>Response``, and Go decodes less of them;
    the paginators pass what Go reads and flag it:

    * a bare-array list passes only the items the cap keeps -- ``followPagination``
      trims the raw items before its caller decodes any (``client.go:604-631``);
    * a wrapped listing passes ``only_under`` its items key, because Go reads a
      followed page as ``struct{ Events []json.RawMessage }`` and decodes each
      event, never the page's ``person`` (``timeline.go:444-457``);
    * ``followed_page`` on :data:`HAND_DECODED_FOLLOWED_PAGES` lets a ``null`` id
      through, below.
    """
    if operation is None:
        return
    null_passes = followed_page and operation in HAND_DECODED_FOLLOWED_PAGES
    for site in table.get(operation, ()):
        if only_under is None or site[:1] == (only_under,):
            _decode_site(body, site, 0, operation, null_passes)


#: Operations whose FOLLOWED pages Go decodes into hand-written types rather than
#: through the generated ``Person``: ``Gauge`` and ``GaugeNeedle``
#: (``gauges.go:236-241, 307-312``) and ``Notification``
#: (``my_notifications.go:285-305``), each after the positional normalizer. Their
#: ``Person.ID`` is a plain ``int64``, which reads JSON ``null`` as ``0`` with no
#: error where ``FlexibleInt64`` refuses it; every other refusal is the same. Go
#: SDK behaviour, not schema, so it is listed by hand. Page 1 of each still goes
#: through ``Parse<Op>Response`` and still refuses ``null``.
HAND_DECODED_FOLLOWED_PAGES: frozenset[str] = frozenset({"ListGauges", "ListGaugeNeedles", "GetBubbleUps"})


def _decode_site(node: Any, site: tuple[str, ...], index: int, operation: str | None, null_passes: bool) -> None:
    if index == len(site):
        # A passed `null` stays `null`, the same representation residual as an
        # absent id, rather than the `0` Go's plain int64 leaves.
        if isinstance(node, dict) and "id" in node and not (null_passes and node["id"] is None):
            node["id"] = _flexible_int64(node["id"], operation, site)
        return
    segment = site[index]
    if segment == "[]":
        children: Iterable[Any] = node if isinstance(node, list) else ()
    elif segment == "{}":
        children = node.values() if isinstance(node, dict) else ()
    else:
        children = (node[segment],) if isinstance(node, dict) and segment in node else ()
    for child in children:
        _decode_site(child, site, index + 1, operation, null_passes)


def _flexible_int64(raw: Any, operation: str | None, site: tuple[str, ...]) -> int:
    # `bool` is an `int` subclass in Python and a JSON boolean in Go: refused.
    if isinstance(raw, int) and not isinstance(raw, bool):
        if _INT64_MIN <= raw <= _INT64_MAX:
            return raw
    elif isinstance(raw, str):
        outcome = parse_int64(raw)
        if outcome is Refusal.SYNTAX:
            return 0
        if outcome is not Refusal.RANGE:
            return outcome
    where = ".".join(site) or "$"
    raise ApiError(
        f"{operation or 'response'}: person id at {where} is not an int64: {_truncate(json.dumps(raw, default=repr))}"
    )
