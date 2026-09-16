"""The person-id grammar corpus: 74 rows, every expectation a measured Go verdict.

Not derived from documentation and not from this implementation. A probe linked
against the real ``go/pkg/types.FlexibleInt64`` and the real
``normalizeEmbeddedPeopleJSON`` (``go/pkg/basecamp/normalize.go:40``, the
``coercePersonID`` rule) produced every row, so a disagreement here is a
disagreement with Go rather than with a reading of it.

One table, two sites. The rule is ``strconv.ParseInt(s, 10, 64)`` and its three
outcomes, and both of the SDK's person-id sites read the SAME row differently
only in what they DO with the outcome:

===========  =============================================  ==========================
``kind``     the pre-decode normalizer must                 the flexible reader must
===========  =============================================  ==========================
``"value"``  write the int, no ``system_label``             return the int
``"label"``  write id ``0`` and ``system_label`` = the raw  return 0
``"refuse"`` leave the string untouched                     fail the read
===========  =============================================  ==========================

Rows that exist to discriminate, so do not drop them as redundant: ``"+7"`` and
``"+007"`` (the sign a ``^-?\\d+$`` regex refuses); ``"007"``, ``"010"``,
``"0009223372036854775807"`` (leading zeros -- ``"010"`` is TEN, and Ruby's
``Integer()`` said eight); the Unicode digit rows (Python's ``int()`` parses
every one of them); the whitespace rows (``int()`` strips ASCII whitespace); the
underscore rows (PEP 515, which ``int()`` accepts and base-10 ``ParseInt`` never
does); ``"18446744073709551615x"`` against ``"18446744073709551616x"`` -- one
digit apart, OPPOSITE refusals, because Go checks the magnitude inside the scan
against ``uint64`` and the first disqualifying byte wins; and
``"9007199254740992"`` / ``"9007199254740993"``, past JS's safe-integer range
but real ``int64`` ids.
"""

from __future__ import annotations

#: ``(raw, kind, value)``. ``value`` is Go's own number for a ``"value"`` row
#: and ``None`` for the other two, which carry no number.
PERSON_ID_CORPUS: list[tuple[str, str, int | None]] = [
    # --- plain, signed and zero-padded: ParseInt's whole accepting grammar ---
    ("7", "value", 7),
    ("0", "value", 0),
    ("-0", "value", 0),
    ("+0", "value", 0),
    ("+7", "value", 7),
    ("-7", "value", -7),
    ("007", "value", 7),
    ("+007", "value", 7),
    ("-007", "value", -7),
    ("0009223372036854775807", "value", 9223372036854775807),
    ("0000000000000000000000009", "value", 9),
    ("010", "value", 10),  # TEN. Base 10 has no octal prefix.
    ("00", "value", 0),
    ("0000", "value", 0),
    ("-00", "value", 0),
    # --- an empty digit run, with or without a sign ---
    ("", "label", None),
    ("+", "label", None),
    ("-", "label", None),
    # --- whitespace: Go trims NONE of it ---
    (" ", "label", None),
    (" 7", "label", None),
    ("7 ", "label", None),
    (" 7 ", "label", None),
    ("\n7", "label", None),
    ("7\n", "label", None),
    ("\t7", "label", None),
    ("7\t", "label", None),
    # --- PEP 515 underscores: a base-0 feature in Go, never a base-10 one ---
    ("1_0", "label", None),
    ("1_2", "label", None),
    # --- base prefixes: also base-0 features ---
    ("0x10", "label", None),
    ("0b11", "label", None),
    ("0o17", "label", None),
    ("0X1F", "label", None),
    # --- junk around or instead of the digits ---
    ("7x", "label", None),
    ("x7", "label", None),
    ("12.0", "label", None),
    ("1e3", "label", None),
    ("12,3", "label", None),
    # --- the system actors this whole rule exists to name ---
    ("basecamp", "label", None),
    ("campfire", "label", None),
    ("LocalPerson", "label", None),
    # --- Unicode decimal digits: ASCII 0-9 only, so none of these is a digit ---
    ("１２３", "label", None),  # fullwidth 123
    ("７", "label", None),  # fullwidth 7
    ("٠١٢", "label", None),  # Arabic-Indic 012
    ("٠", "label", None),  # Arabic-Indic 0
    ("৭", "label", None),  # Bengali 7
    ("۷", "label", None),  # Extended Arabic-Indic 7
    ("৭7", "label", None),  # Bengali 7 then ASCII 7
    ("7৭", "label", None),  # ASCII 7 then Bengali 7
    # --- the int64 boundary, from both sides ---
    ("9223372036854775806", "value", 9223372036854775806),
    ("9223372036854775807", "value", 9223372036854775807),
    ("9223372036854775808", "refuse", None),
    ("9223372036854775809", "refuse", None),
    ("-9223372036854775807", "value", -9223372036854775807),
    ("-9223372036854775808", "value", -9223372036854775808),  # the one asymmetric row
    ("-9223372036854775809", "refuse", None),
    ("+9223372036854775807", "value", 9223372036854775807),
    ("+9223372036854775808", "refuse", None),
    ("0000000000000000000009223372036854775807", "value", 9223372036854775807),
    # --- past int64 but still inside uint64: ParseInt's own bound refuses them ---
    ("18446744073709551614", "refuse", None),
    ("18446744073709551615", "refuse", None),
    ("18446744073709551616", "refuse", None),
    ("99999999999999999999999", "refuse", None),
    ("00000000000000000000018446744073709551616", "refuse", None),
    # --- scan order: which refusal fires depends on WHERE the bad byte sits ---
    # One digit apart, opposite answers. "...615x" gets to the 'x' with the
    # accumulator at exactly uint64 max, so it is a SYNTAX error and reads as
    # the system actor; "...616x" overflows uint64 on the last digit, before
    # the scan ever looks at the 'x', so it is a RANGE error and fails.
    # Testing the string for well-formedness first gets this pair backwards --
    # in the accepting direction, which turns an unreadable id into "basecamp".
    ("18446744073709551615x", "label", None),
    ("18446744073709551616x", "refuse", None),
    ("1844674407370955161x", "label", None),
    ("-18446744073709551615x", "label", None),
    ("-18446744073709551616x", "refuse", None),
    ("99999999999999999999999x", "refuse", None),
    # --- beyond JS's safe integers, but ordinary int64 person ids ---
    ("9007199254740991", "value", 9007199254740991),
    ("9007199254740992", "value", 9007199254740992),
    ("9007199254740993", "value", 9007199254740993),
    ("90071992547409931", "value", 90071992547409931),
    ("-9007199254740993", "value", -9007199254740993),
]
