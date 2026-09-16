"""Tests for notification system actor normalization."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client


def _make_account(account_id: str = "12345"):
    c = Client(access_token="test-token")
    return c.for_account(account_id)


class TestSystemActorNormalization:
    @respx.mock
    def test_non_numeric_creator_id_normalized_to_zero_with_label(self):
        """LocalPerson creator.id: "basecamp" → id=0, system_label="basecamp"."""
        respx.get("https://3.basecampapi.com/12345/my/readings.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "unreads": [
                        {
                            "id": 42,
                            "title": "System notification",
                            "created_at": "2024-01-01T00:00:00Z",
                            "updated_at": "2024-01-01T00:00:00Z",
                            "creator": {
                                "id": "basecamp",
                                "name": "Basecamp",
                                "personable_type": "LocalPerson",
                            },
                        }
                    ],
                    "reads": [],
                    "memories": [],
                    "bubble_ups_count": 0,
                    "scheduled_bubble_ups_count": 0,
                },
            )
        )

        from basecamp.generated.services.my_notifications import MyNotificationsService

        result = MyNotificationsService(_make_account()).get_my_notifications()
        creator = result["unreads"][0]["creator"]

        assert creator["id"] == 0
        assert isinstance(creator["id"], int)
        assert creator["system_label"] == "basecamp"
        assert creator["personable_type"] == "LocalPerson"

    @respx.mock
    def test_numeric_string_creator_id_coerced_to_int(self):
        """Numeric string creator.id: "99999" → id=99999, no system_label."""
        respx.get("https://3.basecampapi.com/12345/my/readings.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "unreads": [
                        {
                            "id": 42,
                            "title": "Normal notification",
                            "created_at": "2024-01-01T00:00:00Z",
                            "updated_at": "2024-01-01T00:00:00Z",
                            "creator": {
                                "id": "99999",
                                "name": "Real Person",
                                "personable_type": "User",
                            },
                        }
                    ],
                    "reads": [],
                    "memories": [],
                    "bubble_ups_count": 0,
                    "scheduled_bubble_ups_count": 0,
                },
            )
        )

        from basecamp.generated.services.my_notifications import MyNotificationsService

        result = MyNotificationsService(_make_account()).get_my_notifications()
        creator = result["unreads"][0]["creator"]

        assert creator["id"] == 99999
        assert isinstance(creator["id"], int)
        assert "system_label" not in creator

    # The grammar itself is pinned row by row in `tests/test_person_id.py`
    # against the shared corpus. These four rows are here for what only a REAL
    # response can show: the normalizer runs inside `_request`, and every one of
    # them used to come out of that path wrong.
    @respx.mock
    @pytest.mark.parametrize(
        ("wire_id", "normalized"),
        [
            # `int()` strips whitespace and reads Unicode digits, so these two
            # arrived as a real person's id where Go names the system actor.
            (" 99999 ", {"id": 0, "system_label": " 99999 "}),
            ("９９", {"id": 0, "system_label": "９９"}),
            # A leading "+" and leading zeros ARE ParseInt's grammar, and the
            # normalizer writes the canonical digits back.
            ("+00099999", {"id": 99999}),
        ],
    )
    def test_the_id_grammar_is_parseint_and_not_pythons(self, wire_id, normalized):
        respx.get("https://3.basecampapi.com/12345/my/readings.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "unreads": [
                        {
                            "id": 42,
                            "title": "Notification",
                            "created_at": "2024-01-01T00:00:00Z",
                            "updated_at": "2024-01-01T00:00:00Z",
                            "creator": {"id": wire_id, "personable_type": "User"},
                        }
                    ],
                    "reads": [],
                    "memories": [],
                    "bubble_ups_count": 0,
                    "scheduled_bubble_ups_count": 0,
                },
            )
        )

        from basecamp.generated.services.my_notifications import MyNotificationsService

        result = MyNotificationsService(_make_account()).get_my_notifications()
        creator = result["unreads"][0]["creator"]

        assert creator == {"personable_type": "User", **normalized}

    # Past int64 the normalizer leaves the string, and `creator` is a `Person`
    # site on this operation, so the typed decode behind it refuses the read --
    # Go's `FlexibleInt64` returns "overflows int64" there
    # (`go/pkg/types/flexible_int64.go:44`), rather than a Python bigint standing
    # in for a wire int64.
    @respx.mock
    def test_a_person_id_past_int64_fails_the_read(self):
        respx.get("https://3.basecampapi.com/12345/my/readings.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "unreads": [{"id": 42, "creator": {"id": "18446744073709551616", "personable_type": "User"}}],
                    "reads": [],
                    "memories": [],
                },
            )
        )

        from basecamp.errors import ApiError
        from basecamp.generated.services.my_notifications import MyNotificationsService

        with pytest.raises(ApiError, match=r"GetMyNotifications: person id at unreads\.\[\]\.creator") as raised:
            MyNotificationsService(_make_account()).get_my_notifications()
        assert raised.value.retryable is False
