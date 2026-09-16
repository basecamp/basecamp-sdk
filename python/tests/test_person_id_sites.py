"""Go's typed decode of ``Person.id`` on the generated read path.

Go types ``Person.id`` as ``types.FlexibleInt64`` and every generated service
decodes through ``Parse<Op>Response``, so an untagged ``{"id": "7"}`` reads as
``7`` wherever a ``Person`` sits and a string at a plain-``int64`` person
(``UpcomingSchedulePerson``, ``MyAssignmentAssignee``,
``TemplateLibraryConfirmationPerson``) is left for Go to refuse. The sites come
from the generator's walk of the schema by the ``x-go-type`` marker; these tests
drive them through the real services, every page included.
"""

from __future__ import annotations

import importlib.util
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp._person_id import decode_person_id_sites
from basecamp.errors import ApiError
from basecamp.generated.services._person_id_sites import PERSON_ID_SITES
from tests.person_id_corpus import PERSON_ID_CORPUS

_BASE = "https://3.basecampapi.com/12345"

CORPUS_IDS = [repr(raw) for raw, _, _ in PERSON_ID_CORPUS]


def _load_generator():
    path = Path(__file__).parent.parent / "scripts" / "generate_services.py"
    spec = importlib.util.spec_from_file_location("generate_services_for_sites", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _account(client: Client):
    return client.for_account("12345")


class TestTheTableIsTheSchemasMarker:
    def test_the_walk_selects_on_the_marker_and_never_on_a_key_name(self):
        generator = _load_generator()
        flexible = {"type": "object", "properties": {"id": {"type": "integer", "x-go-type": "types.FlexibleInt64"}}}
        plain = {"type": "object", "properties": {"id": {"type": "integer", "format": "int64"}}}
        spec = {
            "components": {
                "schemas": {
                    "Person": flexible,
                    "PlainPerson": plain,
                    "Entry": {
                        "type": "object",
                        "properties": {
                            "creator": {"$ref": "#/components/schemas/PlainPerson"},
                            "watcher": {"$ref": "#/components/schemas/Person"},
                            "people": {"type": "array", "items": {"$ref": "#/components/schemas/Person"}},
                            "by_key": {
                                "type": "object",
                                "additionalProperties": {"$ref": "#/components/schemas/Person"},
                            },
                            "child": {"$ref": "#/components/schemas/Entry"},
                        },
                    },
                }
            },
            "paths": {
                "/entries": {
                    "get": {
                        "operationId": "ListEntries",
                        "responses": {
                            "200": {
                                "content": {
                                    "application/json": {
                                        "schema": {"type": "array", "items": {"$ref": "#/components/schemas/Entry"}}
                                    }
                                }
                            },
                            "422": {
                                "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Person"}}}
                            },
                        },
                    }
                },
                "/plain": {
                    "get": {
                        "operationId": "GetPlain",
                        "responses": {
                            "200": {
                                "content": {
                                    "application/json": {"schema": {"$ref": "#/components/schemas/PlainPerson"}}
                                }
                            }
                        },
                    }
                },
                "/person": {
                    "get": {
                        "operationId": "GetPerson",
                        "responses": {
                            "200": {
                                "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Person"}}}
                            }
                        },
                    }
                },
            },
        }

        assert generator.person_id_sites(spec) == {
            "GetPerson": [()],
            "ListEntries": [("[]", "by_key", "{}"), ("[]", "people", "[]"), ("[]", "watcher")],
        }

    @pytest.mark.parametrize(
        "operation",
        # Their people are `UpcomingSchedulePerson`, `MyAssignmentAssignee` and
        # `OutOfOfficePerson`: plain `int64` ids in Go, where a string is a
        # decode error. No `Person` sits anywhere in these bodies.
        ["GetUpcomingSchedule", "GetMyAssignments", "GetMyDueAssignments", "DisableOutOfOffice"],
    )
    def test_a_plain_int64_person_is_no_site(self, operation):
        assert operation not in PERSON_ID_SITES

    def test_the_template_library_confirmation_people_are_no_site(self):
        # `TemplateLibraryConfirmationPerson` rides a 409 body, never a 2xx one,
        # and its id is a plain int64.
        body = {
            "destination_todolist": {"creator": {"id": "7"}},
            "people": [{"id": "7", "name": "Ann", "avatar_url": ""}],
        }
        decode_person_id_sites(body, PERSON_ID_SITES, "CreateTemplateLibraryCopy")
        assert body == {
            "destination_todolist": {"creator": {"id": 7}},
            "people": [{"id": "7", "name": "Ann", "avatar_url": ""}],
        }


class TestAnIdReadsAsFlexibleInt64:
    TABLE = {"GetThing": (("creator",), ("people", "[]"))}

    @pytest.mark.parametrize(("raw", "kind", "value"), PERSON_ID_CORPUS, ids=CORPUS_IDS)
    def test_a_string_id_reads_as_go_reads_it(self, raw, kind, value):
        body = {"creator": {"id": raw, "name": "Ann"}, "people": [{"id": raw}]}
        if kind == "refuse":
            with pytest.raises(ApiError, match=r"GetThing: person id at creator is not an int64") as raised:
                decode_person_id_sites(body, self.TABLE, "GetThing")
            assert raised.value.retryable is False
            return
        decode_person_id_sites(body, self.TABLE, "GetThing")
        expected = value if kind == "value" else 0
        # A syntax refusal is 0 with NO `system_label`: the decoder writes none.
        assert body == {"creator": {"id": expected, "name": "Ann"}, "people": [{"id": expected}]}

    @pytest.mark.parametrize("raw", [7, 0, -(2**63), 2**63 - 1])
    def test_an_int64_number_is_left(self, raw):
        body = {"creator": {"id": raw}}
        decode_person_id_sites(body, self.TABLE, "GetThing")
        assert body == {"creator": {"id": raw}}

    @pytest.mark.parametrize("raw", [10.5, 10.0, 1e3, 2**63, -(2**63) - 1, None, True, False, [], {}, [7], {"id": 7}])
    def test_any_other_json_value_fails_the_read(self, raw):
        # `True` is an `int` in Python and a boolean in Go; `10.0` is integral and
        # `json.Number("10.0").Int64()` still refuses it.
        with pytest.raises(ApiError, match=r"person id at people\.\[\] is not an int64"):
            decode_person_id_sites({"people": [{"id": 1}, {"id": raw}]}, self.TABLE, "GetThing")

    @pytest.mark.parametrize(
        "body",
        [
            {"creator": None, "people": None},
            {"creator": "7", "people": "7"},
            {"creator": {"name": "no id"}, "people": [None, "7", {"name": "no id"}]},
            {"creator": ["7"], "people": {"id": "7"}},
            {},
            [],
            None,
        ],
    )
    def test_a_person_that_is_not_an_object_with_an_id_is_left_alone(self, body):
        # Go zero-fills or refuses these as part of a whole-body typed decode this
        # SDK does not do for any field: a declared residual, not a loosening.
        import copy

        before = copy.deepcopy(body)
        decode_person_id_sites(body, self.TABLE, "GetThing")
        assert body == before

    def test_an_operation_with_no_sites_is_untouched(self):
        body = {"creator": {"id": "7"}}
        decode_person_id_sites(body, self.TABLE, "GetOther")
        decode_person_id_sites(body, self.TABLE, None)
        assert body == {"creator": {"id": "7"}}

    def test_the_decode_is_idempotent(self):
        body = {"creator": {"id": "+007"}, "people": [{"id": "basecamp"}]}
        decode_person_id_sites(body, self.TABLE, "GetThing")
        decode_person_id_sites(body, self.TABLE, "GetThing")
        assert body == {"creator": {"id": 7}, "people": [{"id": 0}]}


def _serve_pages(pattern: str, pages: list, *, base: str) -> None:
    """Serve ``pages`` in order behind Link headers, keyed on the ``page`` param."""

    def respond(request: httpx.Request) -> httpx.Response:
        index = int(request.url.params.get("page", "1")) - 1
        headers = {"Link": f'<{base}?page={index + 2}>; rel="next"'} if index + 1 < len(pages) else {}
        return httpx.Response(200, json=pages[index], headers=headers)

    respx.get(url__regex=pattern).mock(side_effect=respond)


class TestTheGeneratedReadPathDecodes:
    @respx.mock
    def test_a_single_object_read(self):
        respx.get(f"{_BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "creator": {"id": "7", "name": "Ann"}})
        )
        with Client(access_token="t") as client:
            comment = _account(client).comments.get(comment_id=1)
        assert comment["creator"] == {"id": 7, "name": "Ann"}

    @respx.mock
    def test_a_syntax_refusal_is_zero_without_a_label_off_the_positional_surfaces(self):
        respx.get(f"{_BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "creator": {"id": "basecamp", "name": "Basecamp"}})
        )
        with Client(access_token="t") as client:
            comment = _account(client).comments.get(comment_id=1)
        assert comment["creator"] == {"id": 0, "name": "Basecamp"}

    @respx.mock
    def test_a_range_refusal_fails_the_read(self):
        respx.get(f"{_BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "creator": {"id": "9223372036854775808"}})
        )
        with Client(access_token="t") as client, pytest.raises(ApiError, match="GetComment: person id at creator"):
            _account(client).comments.get(comment_id=1)

    @respx.mock
    def test_every_followed_page_of_a_bare_array_list(self):
        base = f"{_BASE}/todolists/3/todos.json"
        _serve_pages(
            r".*/todolists/3/todos\.json.*",
            [
                [{"id": 1, "creator": {"id": "7"}, "assignees": [{"id": "8"}]}],
                [
                    {
                        "id": 2,
                        "creator": {"id": "+9"},
                        "steps": [{"assignees": [{"id": "010"}], "completer": {"id": "11"}}],
                    }
                ],
            ],
            base=base,
        )
        with Client(access_token="t") as client:
            todos = list(_account(client).todos.list(todolist_id=3))
        assert todos[0]["creator"]["id"] == 7
        assert todos[0]["assignees"][0]["id"] == 8
        assert todos[1]["creator"]["id"] == 9
        assert todos[1]["steps"][0]["assignees"][0]["id"] == 10
        assert todos[1]["steps"][0]["completer"]["id"] == 11

    @respx.mock
    def test_a_range_refusal_on_a_followed_page_fails_the_read(self):
        base = f"{_BASE}/recordings/4/comments.json"
        _serve_pages(
            r".*/recordings/4/comments\.json.*",
            [[{"id": 1, "creator": {"id": "7"}}], [{"id": 2, "creator": {"id": "18446744073709551616x"}}]],
            base=base,
        )
        with (
            Client(access_token="t") as client,
            pytest.raises(ApiError, match=r"ListComments: person id at \[\]\.creator"),
        ):
            _account(client).comments.list(recording_id=4)

    @respx.mock
    def test_every_page_of_a_wrapped_paginated_response(self):
        base = f"{_BASE}/reports/users/progress/5.json"
        _serve_pages(
            r".*/reports/users/progress/5\.json.*",
            [
                {"person": {"id": "5"}, "events": [{"id": 1, "creator": {"id": "7"}}]},
                {"events": [{"id": 2, "creator": {"id": "8"}, "attachments": [{"creator": {"id": "9"}}]}]},
            ],
            base=base,
        )
        with Client(access_token="t") as client:
            progress = _account(client).reports.person_progress(person_id=5)
        assert progress["person"]["id"] == 5
        events = list(progress["events"])
        assert [e["creator"]["id"] for e in events] == [7, 8]
        assert events[1]["attachments"][0]["creator"]["id"] == 9

    @respx.mock
    def test_an_unpaginated_list(self):
        respx.get(f"{_BASE}/reports/todos/assigned.json").mock(
            return_value=httpx.Response(200, json=[{"id": "7", "name": "Ann"}])
        )
        with Client(access_token="t") as client:
            people = list(_account(client).people.list_assignable())
        assert people == [{"id": 7, "name": "Ann"}]

    @respx.mock
    def test_a_nested_step_site(self):
        respx.get(f"{_BASE}/card_tables/cards/6").mock(
            return_value=httpx.Response(
                200,
                json={"id": 6, "steps": [{"id": 1, "assignees": [{"id": "7"}, {"id": "8"}], "creator": {"id": "9"}}]},
            )
        )
        with Client(access_token="t") as client:
            card = _account(client).cards.get(card_id=6)
        assert [a["id"] for a in card["steps"][0]["assignees"]] == [7, 8]
        assert card["steps"][0]["creator"]["id"] == 9

    @respx.mock
    def test_a_positional_surface_keeps_its_label_and_decodes_once(self):
        # The normalizer runs first on gauges: its system actor keeps the label
        # and the decode behind it finds an `int` and leaves it.
        base = f"{_BASE}/projects/7/gauge/needles.json"
        _serve_pages(
            r".*/projects/7/gauge/needles\.json.*",
            [[{"id": 1, "creator": {"id": "basecamp"}}], [{"id": 2, "creator": {"id": "007"}}]],
            base=base,
        )
        with Client(access_token="t") as client:
            needles = list(_account(client).gauges.list_gauge_needles(project_id=7))
        assert [n["creator"] for n in needles] == [{"id": 0, "system_label": "basecamp"}, {"id": 7}]


class TestAPlainInt64PersonStaysStrict:
    @respx.mock
    def test_the_upcoming_schedule(self):
        respx.get(url__regex=r".*/reports/schedules/upcoming\.json.*").mock(
            return_value=httpx.Response(
                200,
                json={"schedule_entries": [{"id": 1, "creator": {"id": "7"}, "participants": [{"id": "8"}]}]},
            )
        )
        with Client(access_token="t") as client:
            report = _account(client).reports.upcoming(window_starts_on="2024-01-01", window_ends_on="2024-01-31")
        entry = report["schedule_entries"][0]
        assert entry["creator"] == {"id": "7"}
        assert entry["participants"] == [{"id": "8"}]

    @respx.mock
    def test_my_assignments(self):
        respx.get(f"{_BASE}/my/assignments.json").mock(
            return_value=httpx.Response(
                200, json={"priorities": [{"id": 1, "assignees": [{"id": "7"}]}], "non_priorities": []}
            )
        )
        with Client(access_token="t") as client:
            assignments = _account(client).my_assignments.get_my_assignments()
        assert assignments["priorities"][0]["assignees"] == [{"id": "7"}]


class TestTheAsyncBaseDecodesTheSame:
    @respx.mock
    @pytest.mark.asyncio
    async def test_a_single_object_read(self):
        respx.get(f"{_BASE}/comments/1").mock(return_value=httpx.Response(200, json={"id": 1, "creator": {"id": "7"}}))
        client = AsyncClient(access_token="t")
        try:
            comment = await client.for_account("12345").comments.get(comment_id=1)
        finally:
            await client.close()
        assert comment["creator"] == {"id": 7}

    @respx.mock
    @pytest.mark.asyncio
    async def test_every_page_of_a_wrapped_paginated_response(self):
        base = f"{_BASE}/reports/users/progress/5.json"
        _serve_pages(
            r".*/reports/users/progress/5\.json.*",
            [
                {"person": {"id": "5"}, "events": [{"id": 1, "creator": {"id": "7"}}]},
                {"events": [{"id": 2, "creator": {"id": "8"}}]},
            ],
            base=base,
        )
        client = AsyncClient(access_token="t")
        try:
            progress = await client.for_account("12345").reports.person_progress(person_id=5)
        finally:
            await client.close()
        assert progress["person"]["id"] == 5
        assert [e["creator"]["id"] for e in progress["events"]] == [7, 8]

    @respx.mock
    @pytest.mark.asyncio
    async def test_every_followed_page_of_a_bare_array_list(self):
        base = f"{_BASE}/todolists/3/todos.json"
        _serve_pages(
            r".*/todolists/3/todos\.json.*",
            [[{"id": 1, "creator": {"id": "7"}}], [{"id": 2, "assignees": [{"id": "8"}]}]],
            base=base,
        )
        client = AsyncClient(access_token="t")
        try:
            todos = list(await client.for_account("12345").todos.list(todolist_id=3))
        finally:
            await client.close()
        assert todos[0]["creator"]["id"] == 7
        assert todos[1]["assignees"][0]["id"] == 8

    @respx.mock
    @pytest.mark.asyncio
    async def test_an_unpaginated_list(self):
        respx.get(f"{_BASE}/reports/todos/assigned.json").mock(return_value=httpx.Response(200, json=[{"id": "7"}]))
        client = AsyncClient(access_token="t")
        try:
            people = list(await client.for_account("12345").people.list_assignable())
        finally:
            await client.close()
        assert people == [{"id": 7}]
