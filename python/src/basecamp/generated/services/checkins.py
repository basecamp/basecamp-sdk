# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CheckinsService(BaseService):
    def reminders(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get pending check-in reminders for the current user.

        Returns questions that are pending a response from the authenticated user.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="checkins", operation="reminders", is_mutation=False),
            "/my/question_reminders.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetQuestionReminders",
        )

    def get_answer(self, *, answer_id: int) -> dict[str, Any]:
        """Get a single answer by id.

        Args:
            answer_id: The answer id.
        """
        return self._request(
            OperationInfo(service="checkins", operation="get_answer", is_mutation=False, resource_id=answer_id),
            "GET",
            f"/question_answers/{answer_id}",
            operation="GetAnswer",
        )

    def update_answer(self, *, answer_id: int, content: str, group_on: str | None = None) -> None:
        """Update an existing answer.

        Args:
            answer_id: The answer id.
            content: The content.
            group_on: The group on.
        """
        self._request_void(
            OperationInfo(service="checkins", operation="update_answer", is_mutation=True, resource_id=answer_id),
            "PUT",
            f"/question_answers/{answer_id}",
            json_body=self._compact(content=content, group_on=group_on),
            operation="UpdateAnswer",
        )

    def get_questionnaire(self, *, questionnaire_id: int) -> dict[str, Any]:
        """Get a questionnaire (automatic check-ins container) by id.

        Args:
            questionnaire_id: The questionnaire id.
        """
        return self._request(
            OperationInfo(
                service="checkins", operation="get_questionnaire", is_mutation=False, resource_id=questionnaire_id
            ),
            "GET",
            f"/questionnaires/{questionnaire_id}",
            operation="GetQuestionnaire",
        )

    def list_questions(
        self, *, questionnaire_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all questions in a questionnaire.

        Args:
            questionnaire_id: The questionnaire id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(
                service="checkins", operation="list_questions", is_mutation=False, resource_id=questionnaire_id
            ),
            f"/questionnaires/{questionnaire_id}/questions.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListQuestions",
        )

    def create_question(
        self, *, questionnaire_id: int, title: str, schedule: dict, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a new question in a questionnaire.

        Args:
            questionnaire_id: The questionnaire id.
            title: The title.
            schedule: The schedule.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(
                service="checkins", operation="create_question", is_mutation=True, resource_id=questionnaire_id
            ),
            "POST",
            f"/questionnaires/{questionnaire_id}/questions.json",
            json_body=self._compact(title=title, schedule=schedule, visible_to_clients=visible_to_clients),
            operation="CreateQuestion",
        )

    def get_question(self, *, question_id: int) -> dict[str, Any]:
        """Get a single question by id.

        Args:
            question_id: The question id.
        """
        return self._request(
            OperationInfo(service="checkins", operation="get_question", is_mutation=False, resource_id=question_id),
            "GET",
            f"/questions/{question_id}",
            operation="GetQuestion",
        )

    def update_question(
        self, *, question_id: int, title: str | None = None, schedule: dict | None = None, paused: bool | None = None
    ) -> dict[str, Any]:
        """Update an existing question.

        Args:
            question_id: The question id.
            title: The title.
            schedule: The schedule.
            paused: The paused.
        """
        return self._request(
            OperationInfo(service="checkins", operation="update_question", is_mutation=True, resource_id=question_id),
            "PUT",
            f"/questions/{question_id}",
            json_body=self._compact(title=title, schedule=schedule, paused=paused),
            operation="UpdateQuestion",
        )

    def list_answers(self, *, question_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all answers for a question.

        Args:
            question_id: The question id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="checkins", operation="list_answers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListAnswers",
        )

    def create_answer(self, *, question_id: int, content: str, group_on: str | None = None) -> dict[str, Any]:
        """Create a new answer for a question.

        Args:
            question_id: The question id.
            content: The content.
            group_on: The group on.
        """
        return self._request(
            OperationInfo(service="checkins", operation="create_answer", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/answers.json",
            json_body=self._compact(content=content, group_on=group_on),
            operation="CreateAnswer",
        )

    def answerers(self, *, question_id: int, max_items: int | None = None) -> ListResult:
        """List all people who have answered a question (answerers).

        Args:
            question_id: The question id.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages.
        """
        return self._request_paginated(
            OperationInfo(service="checkins", operation="answerers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers/by.json",
            max_items=max_items,
            operation="ListQuestionAnswerers",
        )

    def by_person(
        self, *, question_id: int, person_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """Get all answers from a specific person for a question.

        Args:
            question_id: The question id.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="checkins", operation="by_person", is_mutation=False, resource_id=person_id),
            f"/questions/{question_id}/answers/by/{person_id}",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetAnswersByPerson",
        )

    def update_notification_settings(
        self, *, question_id: int, notify_on_answer: bool | None = None, digest_include_unanswered: bool | None = None
    ) -> dict[str, Any]:
        """Update notification settings for a check-in question.

        Args:
            question_id: The question id.
            notify_on_answer: Notify when someone answers
            digest_include_unanswered: Include unanswered in digest
        """
        return self._request(
            OperationInfo(
                service="checkins", operation="update_notification_settings", is_mutation=True, resource_id=question_id
            ),
            "PUT",
            f"/questions/{question_id}/notification_settings.json",
            json_body=self._compact(
                notify_on_answer=notify_on_answer, digest_include_unanswered=digest_include_unanswered
            ),
            operation="UpdateQuestionNotificationSettings",
        )

    def pause(self, *, question_id: int) -> dict[str, Any]:
        """Pause a check-in question (stops sending reminders).

        Args:
            question_id: The question id.
        """
        return self._request(
            OperationInfo(service="checkins", operation="pause", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/pause.json",
            operation="PauseQuestion",
        )

    def resume(self, *, question_id: int) -> dict[str, Any]:
        """Resume a paused check-in question (resumes sending reminders).

        Args:
            question_id: The question id.
        """
        return self._request(
            OperationInfo(service="checkins", operation="resume", is_mutation=True, resource_id=question_id),
            "DELETE",
            f"/questions/{question_id}/pause.json",
            operation="ResumeQuestion",
        )


class AsyncCheckinsService(AsyncBaseService):
    async def reminders(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get pending check-in reminders for the current user.

        Returns questions that are pending a response from the authenticated user.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="reminders", is_mutation=False),
            "/my/question_reminders.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetQuestionReminders",
        )

    async def get_answer(self, *, answer_id: int) -> dict[str, Any]:
        """Get a single answer by id.

        Args:
            answer_id: The answer id.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="get_answer", is_mutation=False, resource_id=answer_id),
            "GET",
            f"/question_answers/{answer_id}",
            operation="GetAnswer",
        )

    async def update_answer(self, *, answer_id: int, content: str, group_on: str | None = None) -> None:
        """Update an existing answer.

        Args:
            answer_id: The answer id.
            content: The content.
            group_on: The group on.
        """
        await self._request_void(
            OperationInfo(service="checkins", operation="update_answer", is_mutation=True, resource_id=answer_id),
            "PUT",
            f"/question_answers/{answer_id}",
            json_body=self._compact(content=content, group_on=group_on),
            operation="UpdateAnswer",
        )

    async def get_questionnaire(self, *, questionnaire_id: int) -> dict[str, Any]:
        """Get a questionnaire (automatic check-ins container) by id.

        Args:
            questionnaire_id: The questionnaire id.
        """
        return await self._request(
            OperationInfo(
                service="checkins", operation="get_questionnaire", is_mutation=False, resource_id=questionnaire_id
            ),
            "GET",
            f"/questionnaires/{questionnaire_id}",
            operation="GetQuestionnaire",
        )

    async def list_questions(
        self, *, questionnaire_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all questions in a questionnaire.

        Args:
            questionnaire_id: The questionnaire id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(
                service="checkins", operation="list_questions", is_mutation=False, resource_id=questionnaire_id
            ),
            f"/questionnaires/{questionnaire_id}/questions.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListQuestions",
        )

    async def create_question(
        self, *, questionnaire_id: int, title: str, schedule: dict, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a new question in a questionnaire.

        Args:
            questionnaire_id: The questionnaire id.
            title: The title.
            schedule: The schedule.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(
                service="checkins", operation="create_question", is_mutation=True, resource_id=questionnaire_id
            ),
            "POST",
            f"/questionnaires/{questionnaire_id}/questions.json",
            json_body=self._compact(title=title, schedule=schedule, visible_to_clients=visible_to_clients),
            operation="CreateQuestion",
        )

    async def get_question(self, *, question_id: int) -> dict[str, Any]:
        """Get a single question by id.

        Args:
            question_id: The question id.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="get_question", is_mutation=False, resource_id=question_id),
            "GET",
            f"/questions/{question_id}",
            operation="GetQuestion",
        )

    async def update_question(
        self, *, question_id: int, title: str | None = None, schedule: dict | None = None, paused: bool | None = None
    ) -> dict[str, Any]:
        """Update an existing question.

        Args:
            question_id: The question id.
            title: The title.
            schedule: The schedule.
            paused: The paused.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="update_question", is_mutation=True, resource_id=question_id),
            "PUT",
            f"/questions/{question_id}",
            json_body=self._compact(title=title, schedule=schedule, paused=paused),
            operation="UpdateQuestion",
        )

    async def list_answers(
        self, *, question_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all answers for a question.

        Args:
            question_id: The question id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="list_answers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListAnswers",
        )

    async def create_answer(self, *, question_id: int, content: str, group_on: str | None = None) -> dict[str, Any]:
        """Create a new answer for a question.

        Args:
            question_id: The question id.
            content: The content.
            group_on: The group on.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="create_answer", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/answers.json",
            json_body=self._compact(content=content, group_on=group_on),
            operation="CreateAnswer",
        )

    async def answerers(self, *, question_id: int, max_items: int | None = None) -> ListResult:
        """List all people who have answered a question (answerers).

        Args:
            question_id: The question id.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages.
        """
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="answerers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers/by.json",
            max_items=max_items,
            operation="ListQuestionAnswerers",
        )

    async def by_person(
        self, *, question_id: int, person_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """Get all answers from a specific person for a question.

        Args:
            question_id: The question id.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="by_person", is_mutation=False, resource_id=person_id),
            f"/questions/{question_id}/answers/by/{person_id}",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetAnswersByPerson",
        )

    async def update_notification_settings(
        self, *, question_id: int, notify_on_answer: bool | None = None, digest_include_unanswered: bool | None = None
    ) -> dict[str, Any]:
        """Update notification settings for a check-in question.

        Args:
            question_id: The question id.
            notify_on_answer: Notify when someone answers
            digest_include_unanswered: Include unanswered in digest
        """
        return await self._request(
            OperationInfo(
                service="checkins", operation="update_notification_settings", is_mutation=True, resource_id=question_id
            ),
            "PUT",
            f"/questions/{question_id}/notification_settings.json",
            json_body=self._compact(
                notify_on_answer=notify_on_answer, digest_include_unanswered=digest_include_unanswered
            ),
            operation="UpdateQuestionNotificationSettings",
        )

    async def pause(self, *, question_id: int) -> dict[str, Any]:
        """Pause a check-in question (stops sending reminders).

        Args:
            question_id: The question id.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="pause", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/pause.json",
            operation="PauseQuestion",
        )

    async def resume(self, *, question_id: int) -> dict[str, Any]:
        """Resume a paused check-in question (resumes sending reminders).

        Args:
            question_id: The question id.
        """
        return await self._request(
            OperationInfo(service="checkins", operation="resume", is_mutation=True, resource_id=question_id),
            "DELETE",
            f"/questions/{question_id}/pause.json",
            operation="ResumeQuestion",
        )
