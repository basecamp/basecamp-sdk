"""Tests for ``comments.expand_mentions`` / ``create_with_mentions`` (sync + async).

The account-bound half of the mention write side. Its one guarantee over the
pure helpers is the people read: every requested id is resolved through
``people.get`` for its ``attachable_sgid`` before anything is posted, so a
mention can never be written from an sgid the caller supplied.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, UsageError

ACCOUNT = "12345"
BASE = f"https://3.basecampapi.com/{ACCOUNT}"
RECORDING_ID = 1069479351

VICTOR_ID = 1049715915
VICTOR_SGID = (
    "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
    "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aeb392ebf54ffd820e45f27add22bae3a8c7da56"
)
ANNIE_ID = 1049715914
ANNIE_SGID = (
    "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE0P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
    "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--anniesignature"
)


def _person_route(person_id: int, sgid: str):
    return respx.get(f"{BASE}/people/{person_id}").mock(
        return_value=httpx.Response(200, json={"id": person_id, "attachable_sgid": sgid, "name": "Someone"})
    )


def _create_route():
    return respx.post(f"{BASE}/recordings/{RECORDING_ID}/comments.json").mock(
        return_value=httpx.Response(201, json={"id": 9, "type": "Comment", "content": "posted"})
    )


def _posted_content(route) -> str:
    return json.loads(route.calls[-1].request.content)["content"]


def _comments():
    return Client(access_token="test-token").for_account(ACCOUNT).comments


def _async_comments():
    return AsyncClient(access_token="test-token").for_account(ACCOUNT).comments


class TestExpandMentions:
    @respx.mock
    def test_resolves_each_person_and_writes_the_tag_from_their_sgid(self):
        _person_route(VICTOR_ID, VICTOR_SGID)

        out = _comments().expand_mentions(content="<div>On it.</div>", person_ids=[VICTOR_ID])

        assert out == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    def test_reads_each_distinct_id_exactly_once(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        annie = _person_route(ANNIE_ID, ANNIE_SGID)

        out = _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, ANNIE_ID, VICTOR_ID])

        assert victor.call_count == 1 and annie.call_count == 1
        assert out.count("<bc-attachment ") == 2

    @respx.mock
    def test_reads_the_person_even_when_the_content_already_carries_their_sgid(self):
        # The sgid in caller-supplied content is unsigned and proves nothing, so
        # it never stands in for the authoritative read. What it does do is stop
        # the tag being written twice.
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        content = f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> hi</div>'

        out = _comments().expand_mentions(content=content, person_ids=[VICTOR_ID])

        assert victor.call_count == 1
        assert out == content

    @respx.mock
    def test_makes_no_request_when_nothing_is_mentioned(self):
        route = respx.route(host="3.basecampapi.com")

        assert _comments().expand_mentions(content="<div>x</div>") == "<div>x</div>"
        assert _comments().expand_mentions(content="<div>x</div>", person_ids=[]) == "<div>x</div>"
        assert not route.called

    @respx.mock
    def test_a_failed_person_read_fails_the_expansion(self):
        _person_route(VICTOR_ID, VICTOR_SGID)
        respx.get(f"{BASE}/people/{ANNIE_ID}").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, ANNIE_ID])

        # The canonical code survives: the failure is annotated with which
        # person it was, not replaced by a different error.
        assert raised.value.code == "not_found"
        assert f"resolving mention for person {ANNIE_ID}" in str(raised.value)

    @respx.mock
    def test_checks_each_id_where_go_checks_it(self):
        # Go validates inside the resolve loop, so a bad id after a good one
        # still costs the good one's read. The request sequence is exactly what
        # a shared fixture pins, so it has to mean the same thing here.
        victor = _person_route(VICTOR_ID, VICTOR_SGID)

        with pytest.raises(UsageError, match="invalid mention person id"):
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, -1])

        assert victor.call_count == 1

    @respx.mock
    def test_a_failed_read_says_which_mention_failed_in_the_message(self):
        # Go wraps this, so the prefix is part of the message — and the
        # conformance runners assert on message text. The error's class, code
        # and status survive intact.
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert f"resolving mention for person {VICTOR_ID}" in str(raised.value)
        assert raised.value.code == "not_found"
        assert raised.value.http_status == 404

    @respx.mock
    def test_annotating_a_failure_twice_does_not_repeat_the_prefix(self):
        # Go builds a new error per wrap and cannot double-prefix. Rewriting
        # args in place can — and the standard mock idiom is a `side_effect`
        # holding one pre-built exception instance, re-raised on every call.
        failure = NotFoundError("Not found", http_status=404)
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=failure)

        for _ in range(2):
            with pytest.raises(NotFoundError) as raised:
                _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value).count("resolving mention for person") == 1

    @respx.mock
    def test_a_failure_with_no_message_still_says_which_mention(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=ValueError())

        with pytest.raises(ValueError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        # No message to prefix, so the context goes where it can: a note.
        assert f"resolving mention for person {VICTOR_ID}" in raised.value.__notes__

    @respx.mock
    def test_an_error_whose_str_is_not_its_args_is_not_corrupted(self):
        # `OSError` renders "[Errno 2] ..." whatever its args say, so rewriting
        # them would hide the context AND leave the args wrong.
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=OSError(2, "No such file"))

        with pytest.raises(OSError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert raised.value.args == (2, "No such file")
        assert str(raised.value) == "[Errno 2] No such file"
        assert f"resolving mention for person {VICTOR_ID}" in raised.value.__notes__

    @respx.mock
    @pytest.mark.parametrize(
        "body",
        [
            # `null` is caught by the older `person is None` rung two lines
            # above the Mapping guard, so on its own it cannot discriminate the
            # guard this row exists for. It stays because the OUTCOME is the
            # property under test, and the other two reach the new rung.
            pytest.param(b"null", id="null-via-the-older-rung"),
            b"[]",
            b'"oops"',
            b"7",
        ],
    )
    def test_a_person_read_that_is_not_a_person_posts_nothing(self, body):
        # The write path's only safe failure is a refusal. Go's typed decode
        # refuses these before MentionMarkup sees one; here they arrive as
        # None, a list or a str, and `person.get` would have raised an
        # AttributeError from outside the taxonomy. What must never happen is
        # the mention being dropped and the comment posted without it.
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(
            return_value=httpx.Response(200, content=body, headers={"Content-Type": "application/json; charset=utf-8"})
        )
        posted = respx.post(url__regex=rf"{BASE}/buckets/\d+/recordings/\d+/comments\.json").mock(
            return_value=httpx.Response(201, json={"id": 9})
        )

        with pytest.raises(UsageError):
            _comments().create_with_mentions(recording_id=2, content="<div>hi</div>", mentions=[VICTOR_ID])

        assert not posted.called, "a comment must never post with its mention silently dropped"

    @respx.mock
    def test_re_annotating_names_the_person_that_just_failed(self):
        # Go builds a new wrap each time and always names the id it just failed
        # on. A re-raised instance must not keep naming the first.
        failure = NotFoundError("Not found", http_status=404)
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(NotFoundError):
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])
        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[ANNIE_ID])

        assert str(raised.value) == f"resolving mention for person {ANNIE_ID}: Not found"
        assert raised.value.http_status == 404

    @respx.mock
    def test_annotation_never_masks_the_read_it_describes(self):
        # _annotate runs inside an `except` whose job is to re-raise the read's
        # own failure, so an exception escaping from ANNOTATION would destroy
        # the error it was describing. An exception type is free to give `args`
        # a read-only property or `__notes__` a non-list.
        class Permissive(Exception):
            def __getattr__(self, name):
                return "anything"

        class ReadOnlyArgs(Exception):
            @property
            def args(self):
                return ("fixed",)

        for failure in (Permissive("boom"), ReadOnlyArgs()):
            respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=failure)
            with pytest.raises(type(failure)):
                _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

    @respx.mock
    def test_a_non_list_notes_does_not_escape_the_annotation(self):
        # The note path's own suppress. `__notes__` is a plain list anyone may
        # touch; a non-iterable one raises out of the comprehension, and that
        # must cost the annotation, never the 404 it was describing.
        failure = OSError(2, "No such file")
        failure.__notes__ = 5
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(OSError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert raised.value is failure
        assert raised.value.args == (2, "No such file")

    @respx.mock
    def test_a_non_string_note_costs_nothing(self):
        # Items in `__notes__` that are not ours are not ours to drop, and a
        # non-string one must not take the annotation down with it: the filter
        # tests `isinstance` before it tests the prefix.
        failure = OSError(2, "No such file")
        failure.__notes__ = [object()]
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(OSError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        notes = raised.value.__notes__
        assert len(notes) == 2, f"the foreign note is kept and ours is added; got {notes}"
        assert notes[-1] == f"resolving mention for person {VICTOR_ID}"

    @respx.mock
    def test_the_saved_args_are_trusted_only_when_they_are_args(self):
        # `_ORIGINAL_ARGS` is read off the exception, and an exception is free
        # to answer every getattr. Only a tuple is the saved args; anything
        # else means nothing was saved, and the message must be built from
        # `error.args` rather than from whatever __getattr__ invented.
        class Inventive(Exception):
            def __getattr__(self, name):
                return "x"

        failure = Inventive("boom")
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(Inventive) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value) == f"resolving mention for person {VICTOR_ID}: boom"

    @respx.mock
    def test_the_args_path_does_not_also_leave_a_note(self):
        # The two paths are alternatives, not a sequence. Falling through after
        # rewriting `args` annotates the same failure twice, once in the
        # message and once in a note.
        failure = NotFoundError("Not found", http_status=404)
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value) == f"resolving mention for person {VICTOR_ID}: Not found"
        assert not getattr(raised.value, "__notes__", []), "the args path annotates once, in the message"

    @respx.mock
    def test_saving_the_original_args_never_costs_the_annotation(self):
        # The saved-args write is a courtesy for the NEXT annotation; a type
        # that refuses the attribute for any reason -- not only the
        # AttributeError a __slots__ class raises -- must still get annotated
        # this time round.
        class Rigid(Exception):
            def __setattr__(self, name, value):
                # Only the bookkeeping attribute is refused; `args` and the
                # dunders the interpreter itself writes must still go through.
                if not name.startswith("__") and name != "args":
                    raise TypeError("nothing else may be set")
                super().__setattr__(name, value)

        failure = Rigid("boom")
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(Rigid) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value) == f"resolving mention for person {VICTOR_ID}: boom"

    @respx.mock
    def test_the_note_path_also_names_only_the_latest_person(self):
        # The args path was made idempotent; the note path had the same defect
        # left standing, so a reused instance named every person it ever failed
        # on. `OSError` takes the note path because its __str__ is not its args.
        failure = OSError(2, "No such file")
        respx.get(url__regex=rf"{BASE}/people/\d+").mock(side_effect=failure)

        with pytest.raises(OSError):
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])
        with pytest.raises(OSError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[ANNIE_ID])

        notes = [n for n in raised.value.__notes__ if n.startswith("resolving mention")]
        assert notes == [f"resolving mention for person {ANNIE_ID}"]

    @respx.mock
    def test_refuses_an_id_that_is_not_an_id(self):
        route = respx.route(host="3.basecampapi.com")
        for bad in (0, -1, "1049715915", None, True):
            with pytest.raises(UsageError, match="invalid mention person id"):
                _comments().expand_mentions(content="<div>x</div>", person_ids=[bad])
        assert not route.called


class TestCreateWithMentions:
    @respx.mock
    def test_reads_every_person_before_it_posts(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        create = _create_route()

        _comments().create_with_mentions(recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID])

        assert victor.called and create.called
        assert _posted_content(create) == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    def test_posts_nothing_when_a_mention_cannot_be_resolved(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(403, json={"error": "Denied"}))
        create = _create_route()

        with pytest.raises(ForbiddenError):
            _comments().create_with_mentions(
                recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
            )

        assert not create.called, "nothing is posted on a partial mention list"

    @respx.mock
    def test_posts_a_plain_comment_when_nobody_is_mentioned(self):
        create = _create_route()

        _comments().create_with_mentions(recording_id=RECORDING_ID, content="<div>On it.</div>")

        assert _posted_content(create) == "<div>On it.</div>"

    @respx.mock
    def test_refuses_empty_content_before_any_request(self):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(UsageError, match="content is required"):
            _comments().create_with_mentions(recording_id=RECORDING_ID, content="", mentions=[VICTOR_ID])
        assert not route.called

    @respx.mock
    def test_the_plain_create_is_still_reachable(self):
        # Rule 6 does not arise here -- the composite takes over no generated
        # method name -- and this is what says so.
        create = _create_route()
        _comments().create(recording_id=RECORDING_ID, content="<div>plain</div>")
        assert _posted_content(create) == "<div>plain</div>"


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_reads_every_person_before_it_posts(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        create = _create_route()

        await _async_comments().create_with_mentions(
            recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
        )

        assert victor.called
        assert _posted_content(create) == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    async def test_posts_nothing_when_a_mention_cannot_be_resolved(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(403, json={"error": "Denied"}))
        create = _create_route()

        with pytest.raises(ForbiddenError):
            await _async_comments().create_with_mentions(
                recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
            )

        assert not create.called

    @respx.mock
    async def test_expands_without_a_write(self):
        _person_route(VICTOR_ID, VICTOR_SGID)
        out = await _async_comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])
        assert VICTOR_SGID in out
