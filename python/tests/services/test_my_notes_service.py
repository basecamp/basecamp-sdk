"""Tests for the MyNotesService (the per-user scratchpad note singleton)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import AuthError, ValidationError

BASE = "https://3.basecampapi.com/12345"

_WRITTEN = {
    "id": 5,
    "type": "Notebook::Note",
    "created_at": "2026-07-21T00:02:30.308Z",
    "updated_at": "2026-07-21T00:02:30.308Z",
    "content": '<div dir="auto">Things to remember…</div>',
    "content_attachments": [],
    "url": "https://3.basecampapi.com/12345/my/notes.json",
    "app_url": "https://3.basecamp.com/12345/my/navigation/notes",
}


def _my_notes():
    return Client(access_token="test-token").for_account("12345").my_notes


class TestGetMyNote:
    @respx.mock
    def test_returns_written_note(self):
        respx.get(f"{BASE}/my/notes.json").mock(return_value=httpx.Response(200, json=_WRITTEN))

        note = _my_notes().get_my_note()

        assert note["id"] == 5
        assert note["type"] == "Notebook::Note"

    @respx.mock
    def test_pre_first_write_shape_is_null(self):
        respx.get(f"{BASE}/my/notes.json").mock(
            return_value=httpx.Response(
                200, json={**_WRITTEN, "id": None, "created_at": None, "updated_at": None, "content": ""}
            )
        )

        note = _my_notes().get_my_note()

        assert note["id"] is None
        assert note["created_at"] is None
        assert note["updated_at"] is None
        assert note["content"] == ""

    @respx.mock
    def test_401_surfaces_as_auth_error(self):
        respx.get(f"{BASE}/my/notes.json").mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError):
            _my_notes().get_my_note()


class TestUpdateMyNote:
    @respx.mock
    def test_sends_nested_envelope(self):
        route = respx.put(f"{BASE}/my/notes.json").mock(
            return_value=httpx.Response(200, json={**_WRITTEN, "content": "<div>Updated</div>"})
        )

        note = _my_notes().update_my_note(note={"content": "<div>Updated</div>"})

        assert json.loads(route.calls[-1].request.content) == {"note": {"content": "<div>Updated</div>"}}
        assert note["content"] == "<div>Updated</div>"

    # my/notes_controller.rb:19 renders the field-keyed shape, which is what the
    # operation now declares (FieldValidationError, not ValidationError).
    @respx.mock
    def test_field_keyed_422_surfaces_as_validation_error(self):
        respx.put(f"{BASE}/my/notes.json").mock(
            return_value=httpx.Response(422, json={"errors": {"content": ["can't be blank"]}})
        )

        with pytest.raises(ValidationError) as excinfo:
            _my_notes().update_my_note(note={"content": "x"})

        assert str(excinfo.value) == "content: can't be blank"
        assert excinfo.value.field_errors == {"content": ["can't be blank"]}
