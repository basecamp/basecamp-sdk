"""Tests for notification system actor normalization."""

from __future__ import annotations

import httpx
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
                },
            )
        )

        from basecamp.generated.services.my_notifications import MyNotificationsService

        result = MyNotificationsService(_make_account()).get_my_notifications()
        creator = result["unreads"][0]["creator"]

        assert creator["id"] == 99999
        assert isinstance(creator["id"], int)
        assert "system_label" not in creator
