"""Tests for the mention read/write helpers.

The two sides are deliberately asymmetric and the asymmetry is the point.
READING describes what a text says it mentions, from sgids whose signatures
cannot be checked here. WRITING never lets an unsigned id stand in for the
authoritative people read: it deduplicates on the exact ``attachable_sgid``
string, never on the person id an existing tag decodes to.
"""

from __future__ import annotations

import base64
import json

import pytest

from basecamp.errors import UsageError
from basecamp.mentions import (
    _unescape_like_go,
    mention_markup,
    mentioned_person_ids,
    person_id_from_sgid,
    with_mentions,
)

# A real BC3 attachable sgid for person 1049715915: Ruby Marshal, the older
# {"gid", "purpose", "expires_at"} layout, signed. Kept verbatim rather than
# regenerated, so the Marshal reader is exercised against the shape BC3 actually
# serves -- ivar-carrying strings and symbol links included.
VICTOR_SGID = (
    "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
    "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aeb392ebf54ffd820e45f27add22bae3a8c7da56"
)
VICTOR_ID = 1049715915

# A file attachment's sgid names an ActiveStorage::Blob, not a Person.
BLOB_SGID = "BAh7CEkiCGdpZAY6BkVUSSIsZ2lkOi8vYmMzL0FjdGl2ZVN0b3JhZ2U6OkJsb2IvMTA2OTQ4MDAxMAY6BkVU--c0mm3nt1"


def json_sgid(gid: str, *, purpose: str = "attachable", layout: str = "rails", signed: bool = True) -> str:
    """An sgid in the JSON spelling Rails' JSON message serializer emits."""
    if layout == "rails":
        envelope: dict = {"_rails": {"data": gid, "pur": purpose}}
    else:
        envelope = {"gid": gid, "purpose": purpose, "expires_at": None}
    payload = base64.urlsafe_b64encode(json.dumps(envelope).encode()).decode().rstrip("=")
    return f"{payload}--0123456789abcdef" if signed else payload


def person(person_id: int, sgid: str) -> dict:
    return {"id": person_id, "attachable_sgid": sgid}


def mention(sgid: str) -> str:
    return f'<bc-attachment sgid="{sgid}"></bc-attachment>'


def padded_sgid_payload() -> str:
    """A payload whose base64 actually carries ``=`` padding.

    Most envelopes happen to encode to a multiple of four characters, which
    makes every padding case in the table below vacuous. This finds one that
    does not.
    """
    import base64
    import json

    for filler in range(64):
        body = json.dumps({"gid": "gid://bc3/Person/77", "purpose": "attachable", "expires_at": "x" * filler}).encode()
        encoded = base64.urlsafe_b64encode(body).decode()
        if encoded.count("=") == 2:
            return encoded
    raise AssertionError("no padded payload found")


_BASE64_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"


def _with_dirty_trailing_bits(payload: str) -> str:
    """The same payload with the final group's discarded bits set non-zero.

    A group of two base64 characters carries one byte in twelve bits, four of
    which are thrown away. Several characters therefore decode to the same
    byte, and Go's non-strict decoder accepts all of them.
    """
    import base64

    trimmed = payload.rstrip("=")
    body = base64.b64decode(trimmed.replace("-", "+").replace("_", "/") + "=" * (-len(trimmed) % 4))
    two_char_tail = base64.urlsafe_b64encode(body).decode().rstrip("=")
    if len(two_char_tail) % 4 != 2:
        two_char_tail = base64.urlsafe_b64encode(body + b" ").decode().rstrip("=")
        body = body + b" "
    for candidate in _BASE64_ALPHABET:
        dirty = two_char_tail[:-1] + candidate
        if candidate != two_char_tail[-1] and base64.urlsafe_b64decode(dirty + "==") == body:
            return dirty
    raise AssertionError("no dirty-trailing-bits variant exists for this payload")


class TestPersonIDFromSGID:
    def test_reads_a_marshal_envelope(self):
        assert person_id_from_sgid(VICTOR_SGID) == VICTOR_ID

    def test_reads_both_json_layouts(self):
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/77")) == 77
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/77", layout="legacy")) == 77

    def test_reads_an_unsigned_envelope(self):
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/77", signed=False)) == 77

    def test_refuses_a_purpose_other_than_attachable(self):
        # A Person sgid minted for bookmarking is not a mention, however valid
        # its gid: BC3 refuses it in rich text.
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/77", purpose="bookmarkable")) is None
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/77", purpose="readable", layout="legacy")) is None

    def test_refuses_a_model_that_is_not_a_person(self):
        assert person_id_from_sgid(BLOB_SGID) is None
        assert person_id_from_sgid(json_sgid("gid://bc3/Recording/77")) is None

    @pytest.mark.parametrize(
        "gid",
        [
            "gid://bc3/Person",  # no id
            "gid://bc3/Person/",  # empty id
            "gid://bc3/Person/77/extra",  # a path longer than Model/id
            "gid://bc3/Person/abc",  # not a number
            "gid://bc3/Person/0",  # not a real id
            "gid://bc3/Person/-1",
            "gid:///Person/77",  # no app
            "https://3.basecamp.com/Person/77",  # not a gid at all
            "gid://bc3/Document/gid://bc3/Person/77",  # a Person gid INSIDE another value
        ],
    )
    def test_refuses_a_gid_that_does_not_name_a_person(self, gid):
        assert person_id_from_sgid(json_sgid(gid)) is None

    def test_refuses_a_non_ascii_digit_id(self):
        # `str.isdigit` is true for these; `int()` would parse them into an id
        # BC3 never wrote.
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/١٢")) is None

    def test_a_negative_ivar_count_does_not_slip_an_envelope_through(self):
        # Marshal's packed integers are signed, so an "I" object's ivar count
        # can be written negative; the reader must refuse it rather than skip
        # the ivar loop and accept the object. Named after Go's
        # TestPersonIDFromSGID_HostileMarshal subtest.
        import base64 as b64

        # 0x04 0x08 "I" '"' <len 4> "gid:" ... with an ivar count of -1.
        payload = b'\x04\x08I"\x09gid://bc3/Person/77\xfa'
        assert person_id_from_sgid(b64.urlsafe_b64encode(payload).decode().rstrip("=")) is None

    @pytest.mark.parametrize("sgid", ["", "   ", "not base64!!", "--", "e30", "eyJhIjoxfQ"])
    def test_refuses_undecodable_input(self, sgid):
        assert person_id_from_sgid(sgid) is None

    def test_reads_the_last_separator_because_a_payload_may_contain_one(self):
        # "-" is a base64url character, so a payload can itself contain "--";
        # Rails' verifier splits on the LAST one and so does this.
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        assert person_id_from_sgid(f"{payload}--sig--nature".replace("--sig--nature", "--signature")) == 77

    def test_refuses_an_oversized_payload_without_decoding_it(self):
        assert person_id_from_sgid("A" * 100_000) is None

    @pytest.mark.parametrize("control", ["\n", "\r", "\t", "\x00", "\x7f"])
    def test_refuses_a_control_character_in_the_gid(self, control):
        # Python's URL parser STRIPS tab, CR and LF before parsing, so without
        # an explicit refusal this would report a mention of somebody the API
        # never named. Go's net/url rejects any control character outright.
        assert person_id_from_sgid(json_sgid(f"gid://bc3/Person/104{control}9715915")) is None

    @pytest.mark.parametrize(
        ("authority", "accepted"),
        [
            ("bc3", True),  # the only shape BC3 emits
            ("bc3:80", True),
            ("bc3:", True),  # Go's validOptionalPort accepts an empty port
            ("bc3:xx", False),
            ("bc 3", False),
            ("bc3|x", False),
            ("bc3{", False),
            ("bc3^", False),
            ("café", True),  # Go checks its character set for ASCII bytes ONLY
            ("b]c", True),  # ']' is an ordinary host character
            ("bc[3", False),  # but a stray '[' is not
            ("b%zz", False),  # malformed escape
            ("b%41", False),  # a host may escape only a NON-ASCII byte
            ("b%C3%A9", True),
            (":80", True),  # Go reads this as host ':80'
            ("user@bc3", True),  # userinfo is not part of the host
            ("a@b@bc3", True),  # split on the LAST '@'
            ("a%41@bc3", True),  # an escape of an ASCII byte IS allowed in userinfo
            ("%b@bc3", False),  # but a malformed one is not
            ("a b@bc3", False),  # userinfo has a narrower character set than a host
            ("é@bc3", False),  # and unlike a host it is ASCII-only
            ("[::1]", True),
            ("[::1]:80", True),
            ("[v1.fe80::a+en1]", False),  # IPvFuture: Go parses the address
            ("[fe80::1%25eth0]", True),  # RFC 6874 zone
            ("[fe80::1%eth0]", False),  # a bare '%' is not a zone marker
            ("[not-an-ip]", False),
            ("[1.2.3.4]", False),  # a bare IPv4 is not an IP-literal
            ("[::ffff:1.2.3.4]", True),
            ("[fe80::1%25eth0]:80", True),
            ("[fe80::1%25eth0]:xx", False),
            ("[fe80::1%25]", False),  # the zone may not be empty
            ("[%25eth0]", False),  # nor the address
            ("[notanip%25eth0]", False),
            # A zone may escape anything it could have written literally, PLUS
            # a space, because Windows puts spaces in zone identifiers. So the
            # escaped form is accepted and the literal one is not.
            ("[fe80::1%25%20en0]", True),
            ("[fe80::1%25en 0]", False),
            ("[fe80::1%25%25en0]", True),
            ("[fe80::1%25e%6e0]", True),
            ("[fe80::1%25%41]", True),
            ("[fe80::1%25%C3]", False),  # but not a non-ASCII byte
            ("[fe80::1%25é]", True),  # written literally, though, it is fine
            ("[fe80::1%25a|b]", False),
            ("[fe80::1%25a]b]", True),  # the CLOSING bracket is the last one
            ("[fe80::1%25en[0]", False),  # a second "[" never is
            ("[::1[]", False),
            ("[::1]]", False),
        ],
    )
    def test_validates_the_authority_as_go_does(self, authority, accepted):
        # Every row measured against a linked `url.Parse`, because the obvious
        # reading of "validate the authority" is wrong three ways: Go strips
        # userinfo before validating, permits any non-ASCII byte in a host, and
        # refuses a percent-escape of an ASCII byte that a permissive reading
        # waves through. An earlier version of this port got all three wrong
        # and was a net regression on the corpus.
        sgid = json_sgid(f"gid://{authority}/Person/77")
        assert (person_id_from_sgid(sgid) == 77) is accepted

    @pytest.mark.parametrize("separator", ["\x1c", "\x1d", "\x1e", "\x1f"])
    def test_does_not_trim_the_c0_separators_python_strips(self, separator):
        # `str.strip()` removes these four; Go's unicode.IsSpace does not, and
        # trimming one turns an undecodable sgid into a decodable one.
        assert person_id_from_sgid(separator + json_sgid("gid://bc3/Person/77")) is None

    def test_reads_a_json_envelope_carrying_invalid_utf8(self):
        # Go's encoding/json substitutes U+FFFD and carries on.
        import base64

        body = b'{"gid":"gid://bc3/Person/77","purpose":"attachable","x":"\xff\xfe"}'
        assert person_id_from_sgid(base64.urlsafe_b64encode(body).decode().rstrip("=")) == 77

    @pytest.mark.parametrize(
        ("suffix", "resolves"),
        [
            # Python's html.unescape follows HTML5, which DROPS a numeric
            # reference naming a C0 control; Go emits the character. Dropping
            # is the dangerous direction — it makes a forged tag match the real
            # sgid, and the writer then skips the mention it was asked for.
            ("&#1;", False),
            ("&#x1;", False),
            ("&#127;", False),
            ("&#0;", False),
            # Decimal without a semicolon needs TWO digits to decode; hex needs
            # one. "&#9" stays literal, so the "&" lands in the payload.
            ("&#9", False),
            ("&#09", True),
            ("&#x9", True),
            # A whitespace expansion decodes and is then erased by the trim, so
            # these resolve — in both implementations.
            ("&#160;", True),
            ("&ensp;", True),
        ],
    )
    def test_entity_references_decode_as_go_decodes_them(self, suffix, resolves):
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        content = f'<bc-attachment sgid="{payload}{suffix}"></bc-attachment>'
        assert (mentioned_person_ids(content) == [77]) is resolves

    @pytest.mark.parametrize(
        ("character", "entity", "payload"),
        [
            (
                "fj",
                "&fjlig;",
                "eyJnaWQiOiAiZ2lkOi8vYmMzL1BlcnNvbi83NyIsICJwdXJwb3NlIjogImF0dGFjaGFibGUiLCAieCI6ICLfjMKixqnPuMqk1IHRs9KF04nes8i707EifQ",
            ),
            (
                "+",
                "&plus;",
                "eyJnaWQiOiAiZ2lkOi8vYmMzL1BlcnNvbi83NyIsICJwdXJwb3NlIjogImF0dGFjaGFibGUiLCAieCI6ICLVr8WcyJHKks+nzZzdhN6e0qHKnSJ9",
            ),
            (
                "/",
                "&sol;",
                "eyJnaWQiOiAiZ2lkOi8vYmMzL1BlcnNvbi83NyIsICJwdXJwb3NlIjogImF0dGFjaGFibGUiLCAieCI6ICLPgMWV0LHPlMSSyY/Pn8K31IXTlMuR1rbFnNez0pfMlt+3In0",
            ),
            (
                "_",
                "&lowbar;",
                "eyJnaWQiOiAiZ2lkOi8vYmMzL1BlcnNvbi83NyIsICJwdXJwb3NlIjogImF0dGFjaGFibGUiLCAieCI6ICLOi8-_xavKrcuz06jQodyRz6zDkMis3o7Snt-YxpHOjdeA1pIifQ",
            ),
            (
                "=",
                "&equals;",
                "eyJnaWQiOiAiZ2lkOi8vYmMzL1BlcnNvbi83NyIsICJwdXJwb3NlIjogImF0dGFjaGFibGUiLCAieCI6ICJwYWQifQ==",
            ),
        ],
    )
    def test_an_entity_respelling_a_real_payload_character_still_resolves(self, character, entity, payload):
        """The case an INSERTION-style differential can never reach.

        Inserting an entity into a payload breaks it on both sides, and both
        answer "no mention" — a clean diff proving nothing. These respell a
        character the payload legitimately contains, so a decoder that fails to
        expand the entity gets a different answer from Go. ``&fjlig;`` is the
        sharp one: it lives in Go's SECOND table, the two-rune one, and expands
        to two base64 characters.

        On the write side this decides deduplication — content already carrying
        a mention in its escaped spelling must dedupe to one tag, not two.
        """
        assert mentioned_person_ids(f'<bc-attachment sgid="{payload}"></bc-attachment>') == [77]
        respelled = payload.replace(character, entity, 1)
        assert respelled != payload
        assert mentioned_person_ids(f'<bc-attachment sgid="{respelled}"></bc-attachment>') == [77]

    def test_a_run_of_ampersands_does_not_sweep_the_table_per_character(self):
        # Longest-match-against-the-table is the right RULE; sweeping every
        # length up to the longest name in the table is the wrong mechanism for
        # it. That costs a few hundred comparisons per "&" whatever follows,
        # which is linear with a constant big enough to matter on content an
        # author writes — and a run of ampersands is its worst case.
        #
        # The bound is generous by three orders of magnitude against the
        # sweeping version, so this catches a regression without timing noise.
        import time

        start = time.perf_counter()
        assert mentioned_person_ids("&" * 200_000) == []
        assert time.perf_counter() - start < 2.0

    @pytest.mark.parametrize(
        ("reference", "expected"),
        [
            # Go accumulates the code point in a rune -- an int32 -- and lets
            # it WRAP. 0x100000041 truncates to 0x41, an ordinary "A", where an
            # arbitrary-precision accumulator calls it out of range and emits
            # U+FFFD. No corpus generates this by accident: it takes a value
            # that wraps back INTO the valid range.
            ("&#4294967361;", "A"),
            ("&#0000004294967361;", "A"),
            ("&#x100000041;", "A"),
            # These exceed 2**32, so they need the MASK and not merely the
            # signed conversion. Without them the table passed with the mask
            # deleted — a row that named the wrap and did not test it.
            ("&#8589934657;", "A"),
            ("&#12884901953;", "A"),
            ("&#18446744073709551681;", "A"),
            ("&#x200000041;", "A"),
            # Leading zeros are an ordinary spelling and must not be capped
            # away, which is what a digit-count limit would do.
            ("&#00000000065;", "A"),
            ("&#" + "0" * 40 + "65;", "A"),
            # A wrap that lands negative is still refused, as Go's EncodeRune
            # refuses it.
            ("&#2147483713;", "\ufffd"),
        ],
    )
    def test_numeric_overflow_wraps_as_gos_int32_does(self, reference, expected):
        assert _unescape_like_go(reference) == expected

    @pytest.mark.parametrize("name", ["nGt;", "nLt;"])
    def test_a_name_absent_from_gos_table_is_left_literal(self, name):
        # Python's table decodes these two and Go's does not. Measured across
        # all 2231 names in four forms each; these are the only disagreements.
        #
        # Asserted on the DECODER, not through a mention: both expansions are
        # non-base64, so a mention-level assertion reads [] whether the name was
        # expanded or left alone and passes with the exclusion set emptied.
        assert _unescape_like_go(f"&{name}") == f"&{name}"
        assert _unescape_like_go(f"x&{name}y") == f"x&{name}y"

    @pytest.mark.parametrize(("prefix", "resolves"), [("&nbsp", True), ("&nbsp;", True), ("&amp", False)])
    def test_a_named_reference_matches_the_table_not_the_longest_name_run(self, prefix, resolves):
        # "&nbspBAh7" is a non-breaking space followed by "BAh7" — the match is
        # against the table, longest entry first, not the longest run of name
        # characters. Reading it greedily leaves the whole value undecoded and
        # loses the mention.
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        content = f'<bc-attachment sgid="{prefix}{payload}"></bc-attachment>'
        assert (mentioned_person_ids(content) == [77]) is resolves

    @pytest.mark.parametrize(
        ("suffix", "resolves"),
        [
            # Go cuts the fragment off BEFORE refusing control characters, so a
            # control character behind "#" is not the URL's problem — while a
            # malformed escape in the fragment still fails the parse.
            ("#x", True),
            ("#\n", True),
            ("#\x7f", True),
            ("#%41", True),
            ("#%", False),
            ("#%zz", False),
            ("?q=1", True),
            ("\n#x", False),
        ],
    )
    def test_the_fragment_is_cut_before_the_control_character_check(self, suffix, resolves):
        sgid = json_sgid(f"gid://bc3/Person/77{suffix}")
        assert (person_id_from_sgid(sgid) == 77) is resolves

    def test_refuses_a_percent_encoded_control_character(self):
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/104%0A9715915")) is None

    def test_reads_the_path_decoded(self):
        # Go reads url.Path, which is the decoded form.
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/%31%32%33")) == 123
        assert person_id_from_sgid(json_sgid("gid://bc3/Pers%6fn/123")) == 123

    def test_refuses_an_id_wider_than_int64(self):
        # Python's int is arbitrary-precision where BC3's id is not, and no
        # other SDK in this repo could even produce the answer.
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/9223372036854775807")) == 2**63 - 1
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/9223372036854775808")) is None
        assert person_id_from_sgid(json_sgid("gid://bc3/Person/99999999999999999999999")) is None

    @pytest.mark.parametrize(
        ("label", "mutate", "accepted"),
        [
            # Measured behaviour of Go's RawStdEncoding, probed rather than
            # inferred. Note the shape: CR and LF are skipped, space and tab
            # are not, so a decoder that lumps all whitespace together is wrong
            # in one direction or the other whichever way it jumps.
            ("embedded LF", lambda p: p[:8] + "\n" + p[8:], True),
            ("embedded CR", lambda p: p[:8] + "\r" + p[8:], True),
            ("embedded space", lambda p: p[:8] + " " + p[8:], False),
            ("embedded tab", lambda p: p[:8] + "\t" + p[8:], False),
            # Only Encoding.Strict() checks the final group's discarded bits.
            ("non-zero trailing bits", lambda p: _with_dirty_trailing_bits(p), True),
            ("length % 4 == 1", lambda p: p[: len(p) - ((len(p) - 1) % 4)], False),
        ],
    )
    def test_base64_leniency_matches_go(self, label, mutate, accepted):
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        assert (person_id_from_sgid(mutate(payload)) == 77) is accepted, label

    @pytest.mark.parametrize(
        ("label", "build", "accepted"),
        [
            # Go's ORDER: TrimSpace the whole value, split on the last "--",
            # then TrimRight the padding and hand what is left to a decoder
            # that skips line breaks and refuses every other non-alphabet
            # byte -- "=" included, since RawStdEncoding has no padding
            # character. So a "=" the trim could not reach is fatal, and the
            # reason it could not reach it is a line break sitting after it.
            # Two other ports got this family wrong in opposite directions.
            ("padded, unsigned", lambda p, s: p, True),
            ("padded then separator", lambda p, s: p + s, True),
            ("break between padding and separator", lambda p, s: p + "\n" + s, False),
            ("break amid the padding", lambda p, s: p[:-1] + "\n" + p[-1:], False),
            # TrimSpace runs on the WHOLE value first, so a break at either end
            # is gone before any of that.
            ("padding then a break at the end", lambda p, s: p + "\n", True),
            ("leading break on the whole value", lambda p, s: "\n" + p + s, True),
            ("trailing break on the whole value", lambda p, s: p + s + "\n", True),
            # TrimRight reaches this padding, so the break before it is only a
            # break, and the decoder skips it.
            ("break before the padding", lambda p, s: p.rstrip("=") + "\n==", True),
            ("break inside the payload", lambda p, s: p[:8] + "\n" + p[8:], True),
            ("break inside the signature", lambda p, s: p + "--0123\n456789", True),
            # Space and tab are not line breaks and Go's decoder does not skip
            # them.
            ("space between padding and separator", lambda p, s: p + " " + s, False),
            ("tab inside the payload", lambda p, s: p[:8] + "\t" + p[8:], False),
        ],
    )
    def test_padding_and_line_break_ordering_matches_go(self, label, build, accepted):
        payload = padded_sgid_payload()
        value = build(payload, "--0123456789abcdef")
        assert (person_id_from_sgid(value) == 77) is accepted, label

    def test_reads_a_line_wrapped_payload_as_go_does(self):
        # Go's base64 decoder skips CR and LF mid-stream. It does NOT skip
        # spaces or tabs, and neither does this.
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        assert person_id_from_sgid(payload[:8] + "\n" + payload[8:]) == 77
        assert person_id_from_sgid(payload[:8] + "\r\n" + payload[8:]) == 77
        assert person_id_from_sgid(payload[:8] + " " + payload[8:]) is None


class TestMentionedPersonIDs:
    def test_reports_each_mention_once_in_document_order(self):
        second = json_sgid("gid://bc3/Person/42")
        content = f"<div>{mention(VICTOR_SGID)} and {mention(second)} and {mention(VICTOR_SGID)}</div>"
        assert mentioned_person_ids(content) == [VICTOR_ID, 42]

    def test_skips_attachments_that_are_not_mentions(self):
        assert mentioned_person_ids(f"<div>{mention(BLOB_SGID)}</div>") == []

    def test_counts_a_quoted_mention(self):
        # BC3 notifies quoted mentions too, so the read matches the write.
        assert mentioned_person_ids(f"<blockquote>{mention(VICTOR_SGID)}</blockquote>") == [VICTOR_ID]

    def test_ignores_a_tag_inside_an_html_comment(self):
        assert mentioned_person_ids(f"<div><!-- {mention(VICTOR_SGID)} --></div>") == []

    def test_ignores_a_tag_spelled_inside_another_tags_attribute(self):
        content = f'<div title="{mention(VICTOR_SGID)}">hi</div>'.replace('"<bc', "'<bc").replace('">hi', "'>hi")
        assert mentioned_person_ids(content) == []

    def test_reads_attributes_in_any_order_case_and_quote_style(self):
        content = f"<BC-Attachment content-type='x' SGID='{VICTOR_SGID}'>x</bc-attachment>"
        assert mentioned_person_ids(content) == [VICTOR_ID]

    def test_a_greater_than_inside_a_quoted_value_does_not_end_the_tag(self):
        content = f'<bc-attachment alt="a > b" sgid="{VICTOR_SGID}"></bc-attachment>'
        assert mentioned_person_ids(content) == [VICTOR_ID]

    def test_decodes_entity_escapes_in_the_value(self):
        payload = json_sgid("gid://bc3/Person/77", signed=False)
        assert mentioned_person_ids(f'<bc-attachment sgid="{payload}&#45;&#45;sig"></bc-attachment>') == [77]

    def test_the_first_sgid_attribute_wins(self):
        other = json_sgid("gid://bc3/Person/42")
        content = f'<bc-attachment sgid="{VICTOR_SGID}" sgid="{other}"></bc-attachment>'
        assert mentioned_person_ids(content) == [VICTOR_ID]

    def test_a_similarly_named_tag_is_not_a_bc_attachment(self):
        assert mentioned_person_ids(f'<bc-attachment-preview sgid="{VICTOR_SGID}"></bc-attachment-preview>') == []

    def test_an_unterminated_tag_ends_the_walk(self):
        assert mentioned_person_ids(f'<div><bc-attachment sgid="{VICTOR_SGID}"') == []

    def test_an_unterminated_comment_swallows_the_rest(self):
        assert mentioned_person_ids(f"<!-- <div>{mention(VICTOR_SGID)}") == []

    def test_plain_text_mentions_nobody(self):
        assert mentioned_person_ids("just words, and a stray < too") == []


class TestMentionMarkup:
    def test_renders_the_write_side_tag(self):
        assert mention_markup(person(VICTOR_ID, VICTOR_SGID)) == mention(VICTOR_SGID)

    def test_refuses_a_person_with_no_sgid(self):
        with pytest.raises(UsageError, match="no attachable_sgid"):
            mention_markup({"id": VICTOR_ID, "name": "Victor Cooper"})

    def test_refuses_a_person_with_no_id(self):
        # Go reads a missing id as 0 and lets the sgid/id check report it, so
        # the diagnosis is the same rung of the ladder rather than one of our
        # own invention.
        with pytest.raises(UsageError, match="does not name that person"):
            mention_markup({"attachable_sgid": VICTOR_SGID})

    def test_refuses_a_person_with_neither_id_nor_sgid_on_the_sgid(self):
        # The absent sgid is the earlier rung, and it wins, as in Go.
        with pytest.raises(UsageError, match="no attachable_sgid"):
            mention_markup({})

    def test_refuses_an_sgid_that_names_someone_else(self):
        with pytest.raises(UsageError, match="does not name that person"):
            mention_markup(person(42, VICTOR_SGID))

    def test_refuses_an_sgid_that_names_a_file(self):
        with pytest.raises(UsageError, match="does not name that person"):
            mention_markup(person(1069480010, BLOB_SGID))

    @pytest.mark.parametrize("bad", ['a"b', "a'b", "a<b", "a>b", "a&b"])
    def test_refuses_an_sgid_that_could_break_out_of_the_attribute(self, bad):
        with pytest.raises(UsageError, match="malformed attachable_sgid"):
            mention_markup(person(VICTOR_ID, bad))


class TestWithMentions:
    def test_places_mentions_inside_a_leading_block(self):
        assert with_mentions("<div>On it.</div>", [person(VICTOR_ID, VICTOR_SGID)]) == (
            f"<div>{mention(VICTOR_SGID)} On it.</div>"
        )

    def test_places_mentions_inside_a_leading_paragraph_with_attributes(self):
        content = '<p class="x" data-a="b>c">On it.</p>'
        assert with_mentions(content, [person(VICTOR_ID, VICTOR_SGID)]) == (
            f'<p class="x" data-a="b>c">{mention(VICTOR_SGID)} On it.</p>'
        )

    @pytest.mark.parametrize("opening", ["<paragraph>", "<divx>", "<pre>", "<b>", "<p_>"])
    def test_a_tag_that_only_starts_like_a_block_is_a_bare_prefix(self, opening):
        # "<p" and "<div" match only when the NAME ends there. Named after Go's
        # WithMentions subtest of the same description.
        content = f"{opening}On it.</x>"
        assert with_mentions(content, [person(VICTOR_ID, VICTOR_SGID)]) == f"{mention(VICTOR_SGID)} {content}"

    def test_prefixes_content_that_does_not_open_with_a_block(self):
        assert with_mentions("On it.", [person(VICTOR_ID, VICTOR_SGID)]) == f"{mention(VICTOR_SGID)} On it."

    def test_adds_nothing_when_no_people_are_given(self):
        assert with_mentions("<div>On it.</div>", []) == "<div>On it.</div>"

    def test_does_not_repeat_a_person_passed_twice(self):
        out = with_mentions("<div>x</div>", [person(VICTOR_ID, VICTOR_SGID), person(VICTOR_ID, VICTOR_SGID)])
        assert out.count("<bc-attachment ") == 1

    def test_leaves_alone_a_mention_the_content_already_carries_verbatim(self):
        content = f"<div>{mention(VICTOR_SGID)} hi</div>"
        assert with_mentions(content, [person(VICTOR_ID, VICTOR_SGID)]) == content

    def test_still_mentions_a_person_whose_sgid_differs_from_the_one_in_the_content(self):
        # The trust boundary: a tag naming the right person id with a DIFFERENT
        # sgid proves nothing -- the signature cannot be checked here -- so the
        # authoritative sgid is written anyway. Deduplicating by person id
        # instead would let a forged or stale tag suppress the real mention.
        stale = json_sgid(f"gid://bc3/Person/{VICTOR_ID}")
        content = f"<div>{mention(stale)} hi</div>"
        out = with_mentions(content, [person(VICTOR_ID, VICTOR_SGID)])
        assert out.count("<bc-attachment ") == 2
        assert VICTOR_SGID in out

    def test_refuses_the_whole_expansion_when_one_person_is_unmentionable(self):
        with pytest.raises(UsageError):
            with_mentions("<div>x</div>", [person(VICTOR_ID, VICTOR_SGID), {"id": 42}])

    def test_validates_even_a_person_the_content_already_mentions(self):
        with pytest.raises(UsageError, match="does not name that person"):
            with_mentions(f"<div>{mention(VICTOR_SGID)}</div>", [person(42, VICTOR_SGID)])

    def test_round_trips_through_the_reader(self):
        out = with_mentions("<div>x</div>", [person(VICTOR_ID, VICTOR_SGID)])
        assert mentioned_person_ids(out) == [VICTOR_ID]
