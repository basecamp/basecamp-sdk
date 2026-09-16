"""Reading a person id off the wire the way Go's ``strconv.ParseInt(s, 10, 64)`` reads one.

BC3 serializes person ids as JSON strings in some responses and as JSON numbers
in others, and it spells the system actors -- ``LocalPerson``, whose id is
``"basecamp"`` or ``"campfire"`` -- in the same string field. Two places in this
SDK have to turn that string into a number: the pre-decode normalizer that every
response body walks (``generated/services/_base.py`` and its async twin) and the
flexible id reader in the recording-summary decode
(``services/_campfire_index.py``). The rule lives HERE so those two cannot drift
apart; they had two copies of it, and the copies disagreed.

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

from collections.abc import MutableMapping
from enum import Enum
from typing import Any

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
    digits = text[1:] if sign in ("+", "-") else text
    # An empty digit run, with or without a sign, is a syntax error: "", "+", "-".
    if not digits:
        return Refusal.SYNTAX

    # `ParseUint`'s loop, byte for byte. `str.isdigit()` would be wrong here --
    # it is true for "৭" -- and so would `str.isdecimal()`; the test is on the
    # ASCII range itself.
    magnitude = 0
    for character in digits:
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

    An object whose ``id`` is already a number, or absent, is left alone.
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
