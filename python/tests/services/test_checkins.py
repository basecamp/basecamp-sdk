from __future__ import annotations

import httpx
import respx

from basecamp.client import Client


def make_account():
    c = Client(access_token="test-token")
    return c, c.for_account("12345")


class TestUpdateAnswer:
    @respx.mock
    def test_update_answer_with_group_on(self):
        route = respx.put("https://3.basecampapi.com/12345/question_answers/200").mock(return_value=httpx.Response(204))

        client, account = make_account()
        account.checkins.update_answer(
            answer_id=200,
            content="<p>Updated answer</p>",
            group_on="2025-03-01",
        )
        client.close()

        assert route.called
        req = route.calls.last.request
        body = req.content.decode()
        assert '"content": "<p>Updated answer</p>"' in body or '"content":"<p>Updated answer</p>"' in body
        assert '"group_on": "2025-03-01"' in body or '"group_on":"2025-03-01"' in body

    @respx.mock
    def test_update_answer_omits_group_on_when_none(self):
        route = respx.put("https://3.basecampapi.com/12345/question_answers/200").mock(return_value=httpx.Response(204))

        client, account = make_account()
        account.checkins.update_answer(
            answer_id=200,
            content="<p>Updated answer</p>",
        )
        client.close()

        assert route.called
        req = route.calls.last.request
        body = req.content.decode()
        assert "group_on" not in body
