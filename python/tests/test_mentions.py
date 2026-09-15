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
        with pytest.raises(UsageError, match="no id"):
            mention_markup({"attachable_sgid": VICTOR_SGID})

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
