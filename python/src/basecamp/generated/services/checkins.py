# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CheckinsService(BaseService):
    def reminders(self) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="checkins", operation="reminders", is_mutation=False), "/my/question_reminders.json"
        )

    def get_answer(self, *, answer_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="get_answer", is_mutation=False, resource_id=answer_id),
            "GET",
            f"/question_answers/{answer_id}",
        )

    def update_answer(self, *, answer_id: int | str, content: str) -> None:
        self._request_void(
            OperationInfo(service="checkins", operation="update_answer", is_mutation=True, resource_id=answer_id),
            "PUT",
            f"/question_answers/{answer_id}",
            json_body=self._compact(content=content),
            operation="UpdateAnswer",
        )

    def get_questionnaire(self, *, questionnaire_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="checkins", operation="get_questionnaire", is_mutation=False, resource_id=questionnaire_id
            ),
            "GET",
            f"/questionnaires/{questionnaire_id}",
        )

    def list_questions(self, *, questionnaire_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(
                service="checkins", operation="list_questions", is_mutation=False, resource_id=questionnaire_id
            ),
            f"/questionnaires/{questionnaire_id}/questions.json",
        )

    def create_question(self, *, questionnaire_id: int | str, title: str, schedule: dict) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="checkins", operation="create_question", is_mutation=True, resource_id=questionnaire_id
            ),
            "POST",
            f"/questionnaires/{questionnaire_id}/questions.json",
            json_body=self._compact(title=title, schedule=schedule),
            operation="CreateQuestion",
        )

    def get_question(self, *, question_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="get_question", is_mutation=False, resource_id=question_id),
            "GET",
            f"/questions/{question_id}",
        )

    def update_question(
        self,
        *,
        question_id: int | str,
        title: str | None = None,
        schedule: dict | None = None,
        paused: bool | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="update_question", is_mutation=True, resource_id=question_id),
            "PUT",
            f"/questions/{question_id}",
            json_body=self._compact(title=title, schedule=schedule, paused=paused),
            operation="UpdateQuestion",
        )

    def list_answers(self, *, question_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="checkins", operation="list_answers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers.json",
        )

    def create_answer(self, *, question_id: int | str, content: str, group_on: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="create_answer", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/answers.json",
            json_body=self._compact(content=content, group_on=group_on),
            operation="CreateAnswer",
        )

    def answerers(self, *, question_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="checkins", operation="answerers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers/by.json",
        )

    def by_person(self, *, question_id: int | str, person_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="checkins", operation="by_person", is_mutation=False, resource_id=person_id),
            f"/questions/{question_id}/answers/by/{person_id}",
        )

    def update_notification_settings(
        self,
        *,
        question_id: int | str,
        notify_on_answer: bool | None = None,
        digest_include_unanswered: bool | None = None,
    ) -> dict[str, Any]:
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

    def pause(self, *, question_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="pause", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/pause.json",
            operation="PauseQuestion",
        )

    def resume(self, *, question_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="checkins", operation="resume", is_mutation=True, resource_id=question_id),
            "DELETE",
            f"/questions/{question_id}/pause.json",
            operation="ResumeQuestion",
        )


class AsyncCheckinsService(AsyncBaseService):
    async def reminders(self) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="reminders", is_mutation=False), "/my/question_reminders.json"
        )

    async def get_answer(self, *, answer_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="get_answer", is_mutation=False, resource_id=answer_id),
            "GET",
            f"/question_answers/{answer_id}",
        )

    async def update_answer(self, *, answer_id: int | str, content: str) -> None:
        await self._request_void(
            OperationInfo(service="checkins", operation="update_answer", is_mutation=True, resource_id=answer_id),
            "PUT",
            f"/question_answers/{answer_id}",
            json_body=self._compact(content=content),
            operation="UpdateAnswer",
        )

    async def get_questionnaire(self, *, questionnaire_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="checkins", operation="get_questionnaire", is_mutation=False, resource_id=questionnaire_id
            ),
            "GET",
            f"/questionnaires/{questionnaire_id}",
        )

    async def list_questions(self, *, questionnaire_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(
                service="checkins", operation="list_questions", is_mutation=False, resource_id=questionnaire_id
            ),
            f"/questionnaires/{questionnaire_id}/questions.json",
        )

    async def create_question(self, *, questionnaire_id: int | str, title: str, schedule: dict) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="checkins", operation="create_question", is_mutation=True, resource_id=questionnaire_id
            ),
            "POST",
            f"/questionnaires/{questionnaire_id}/questions.json",
            json_body=self._compact(title=title, schedule=schedule),
            operation="CreateQuestion",
        )

    async def get_question(self, *, question_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="get_question", is_mutation=False, resource_id=question_id),
            "GET",
            f"/questions/{question_id}",
        )

    async def update_question(
        self,
        *,
        question_id: int | str,
        title: str | None = None,
        schedule: dict | None = None,
        paused: bool | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="update_question", is_mutation=True, resource_id=question_id),
            "PUT",
            f"/questions/{question_id}",
            json_body=self._compact(title=title, schedule=schedule, paused=paused),
            operation="UpdateQuestion",
        )

    async def list_answers(self, *, question_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="list_answers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers.json",
        )

    async def create_answer(
        self, *, question_id: int | str, content: str, group_on: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="create_answer", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/answers.json",
            json_body=self._compact(content=content, group_on=group_on),
            operation="CreateAnswer",
        )

    async def answerers(self, *, question_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="answerers", is_mutation=False, resource_id=question_id),
            f"/questions/{question_id}/answers/by.json",
        )

    async def by_person(self, *, question_id: int | str, person_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="checkins", operation="by_person", is_mutation=False, resource_id=person_id),
            f"/questions/{question_id}/answers/by/{person_id}",
        )

    async def update_notification_settings(
        self,
        *,
        question_id: int | str,
        notify_on_answer: bool | None = None,
        digest_include_unanswered: bool | None = None,
    ) -> dict[str, Any]:
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

    async def pause(self, *, question_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="pause", is_mutation=True, resource_id=question_id),
            "POST",
            f"/questions/{question_id}/pause.json",
            operation="PauseQuestion",
        )

    async def resume(self, *, question_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="checkins", operation="resume", is_mutation=True, resource_id=question_id),
            "DELETE",
            f"/questions/{question_id}/pause.json",
            operation="ResumeQuestion",
        )
