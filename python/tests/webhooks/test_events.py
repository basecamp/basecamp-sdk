from __future__ import annotations

from basecamp.webhooks.events import parse_event_kind


class TestParseEventKind:
    def test_simple(self):
        assert parse_event_kind("todo_created") == ("todo", "created")

    def test_compound_type(self):
        assert parse_event_kind("question_answer_created") == ("question_answer", "created")

    def test_multiple_underscores(self):
        assert parse_event_kind("schedule_entry_changed") == ("schedule_entry", "changed")

    def test_no_underscore(self):
        assert parse_event_kind("unknown") == ("unknown", "")

    def test_single_char_action(self):
        assert parse_event_kind("todo_x") == ("todo", "x")

    def test_trailing_underscore(self):
        assert parse_event_kind("todo_") == ("todo", "")
