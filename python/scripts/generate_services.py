#!/usr/bin/env python3
"""Generates Python service classes from OpenAPI spec.

Usage: python scripts/generate_services.py [--openapi ../openapi.json] [--output src/basecamp/generated/services]

This generator:
1. Parses openapi.json
2. Groups operations by tag
3. Maps operationIds to method names
4. Generates Python service files with sync and async classes
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

METHODS = ("get", "post", "put", "patch", "delete")

# Tag to service name mapping overrides
TAG_TO_SERVICE = {
    "Card Tables": "CardTables",
    "Campfire": "Campfires",
    "Todos": "Todos",
    "Messages": "Messages",
    "Files": "Files",
    "Forwards": "Forwards",
    "Schedule": "Schedules",
    "People": "People",
    "Projects": "Projects",
    "Automation": "Automation",
    "ClientFeatures": "ClientFeatures",
    "Boosts": "Boosts",
    "Untagged": "Miscellaneous",
}

# Service splits - some tags map to multiple services
SERVICE_SPLITS: dict[str, dict[str, list[str]]] = {
    "Campfire": {
        "Campfires": [
            "GetCampfire", "ListCampfires",
            "ListChatbots", "CreateChatbot", "GetChatbot", "UpdateChatbot", "DeleteChatbot",
            "ListCampfireLines", "CreateCampfireLine", "GetCampfireLine", "DeleteCampfireLine",
            "ListCampfireUploads", "CreateCampfireUpload",
        ],
    },
    "Card Tables": {
        "CardTables": ["GetCardTable"],
        "Cards": ["GetCard", "UpdateCard", "MoveCard", "CreateCard", "ListCards"],
        "CardColumns": [
            "GetCardColumn", "UpdateCardColumn", "SetCardColumnColor",
            "EnableCardColumnOnHold", "DisableCardColumnOnHold",
            "CreateCardColumn", "MoveCardColumn",
        ],
        "CardSteps": [
            "GetCardStep", "CreateCardStep", "UpdateCardStep", "SetCardStepCompletion",
            "RepositionCardStep",
        ],
    },
    "Files": {
        "Attachments": ["CreateAttachment"],
        "Uploads": ["GetUpload", "UpdateUpload", "ListUploads", "CreateUpload", "ListUploadVersions"],
        "Vaults": ["GetVault", "UpdateVault", "ListVaults", "CreateVault"],
        "Documents": ["GetDocument", "UpdateDocument", "ListDocuments", "CreateDocument"],
    },
    "Automation": {
        "Tools": ["GetTool", "UpdateTool", "DeleteTool", "CloneTool", "EnableTool", "DisableTool", "RepositionTool"],
        "Recordings": ["GetRecording", "ArchiveRecording", "UnarchiveRecording", "TrashRecording", "ListRecordings"],
        "Webhooks": ["ListWebhooks", "CreateWebhook", "GetWebhook", "UpdateWebhook", "DeleteWebhook"],
        "Events": ["ListEvents"],
        "Lineup": ["CreateLineupMarker", "UpdateLineupMarker", "DeleteLineupMarker"],
        "Search": ["Search", "GetSearchMetadata"],
        "Templates": [
            "ListTemplates", "CreateTemplate", "GetTemplate", "UpdateTemplate",
            "DeleteTemplate", "CreateProjectFromTemplate", "GetProjectConstruction",
        ],
        "Checkins": [
            "GetQuestionnaire", "ListQuestions", "CreateQuestion", "GetQuestion",
            "UpdateQuestion", "ListAnswers", "CreateAnswer", "GetAnswer", "UpdateAnswer",
        ],
    },
    "Messages": {
        "Messages": ["GetMessage", "UpdateMessage", "CreateMessage", "ListMessages", "PinMessage", "UnpinMessage"],
        "MessageBoards": ["GetMessageBoard"],
        "MessageTypes": [
            "ListMessageTypes", "CreateMessageType", "GetMessageType",
            "UpdateMessageType", "DeleteMessageType",
        ],
        "Comments": ["GetComment", "UpdateComment", "ListComments", "CreateComment"],
    },
    "People": {
        "People": [
            "GetMyProfile", "ListPeople", "GetPerson", "ListProjectPeople",
            "UpdateProjectAccess", "ListPingablePeople",
        ],
        "Subscriptions": ["GetSubscription", "Subscribe", "Unsubscribe", "UpdateSubscription"],
    },
    "Schedule": {
        "Schedules": [
            "GetSchedule", "UpdateScheduleSettings", "ListScheduleEntries",
            "CreateScheduleEntry", "GetScheduleEntry", "UpdateScheduleEntry",
            "GetScheduleEntryOccurrence",
        ],
        "Timesheets": [
            "GetRecordingTimesheet", "GetProjectTimesheet", "GetTimesheetReport",
            "GetTimesheetEntry", "CreateTimesheetEntry", "UpdateTimesheetEntry",
        ],
    },
    "ClientFeatures": {
        "ClientApprovals": ["ListClientApprovals", "GetClientApproval"],
        "ClientCorrespondences": ["ListClientCorrespondences", "GetClientCorrespondence"],
        "ClientReplies": ["ListClientReplies", "GetClientReply"],
        "ClientVisibility": ["SetClientVisibility"],
    },
    "Todos": {
        "Todos": ["ListTodos", "CreateTodo", "GetTodo", "UpdateTodo", "CompleteTodo", "UncompleteTodo", "TrashTodo"],
        "Todolists": ["GetTodolistOrGroup", "UpdateTodolistOrGroup", "ListTodolists", "CreateTodolist"],
        "Todosets": ["GetTodoset"],
        "TodolistGroups": ["ListTodolistGroups", "CreateTodolistGroup", "RepositionTodolistGroup"],
        "HillCharts": ["GetHillChart", "UpdateHillChartSettings"],
    },
    "Untagged": {
        "Timeline": ["GetProjectTimeline"],
        "Reports": ["GetProgressReport", "GetUpcomingSchedule", "GetAssignedTodos", "GetOverdueTodos", "GetPersonProgress"],
        "Checkins": [
            "GetQuestionReminders", "ListQuestionAnswerers", "GetAnswersByPerson",
            "UpdateQuestionNotificationSettings", "PauseQuestion", "ResumeQuestion",
        ],
        "Todos": ["RepositionTodo"],
        "People": ["ListAssignablePeople"],
        "CardColumns": ["SubscribeToCardColumn", "UnsubscribeFromCardColumn"],
    },
}

# Method name overrides
METHOD_NAME_OVERRIDES = {
    "GetMyProfile": "my_profile",
    "GetTodolistOrGroup": "get",
    "UpdateTodolistOrGroup": "update",
    "SetCardColumnColor": "set_color",
    "EnableCardColumnOnHold": "enable_on_hold",
    "DisableCardColumnOnHold": "disable_on_hold",
    "RepositionCardStep": "reposition",
    "CreateCardStep": "create",
    "UpdateCardStep": "update",
    "SetCardStepCompletion": "set_completion",
    "GetQuestionnaire": "get_questionnaire",
    "GetQuestion": "get_question",
    "GetAnswer": "get_answer",
    "ListQuestions": "list_questions",
    "ListAnswers": "list_answers",
    "CreateQuestion": "create_question",
    "CreateAnswer": "create_answer",
    "UpdateQuestion": "update_question",
    "UpdateAnswer": "update_answer",
    "GetQuestionReminders": "reminders",
    "GetAnswersByPerson": "by_person",
    "ListQuestionAnswerers": "answerers",
    "UpdateQuestionNotificationSettings": "update_notification_settings",
    "PauseQuestion": "pause",
    "ResumeQuestion": "resume",
    "GetSearchMetadata": "metadata",
    "Search": "search",
    "CreateProjectFromTemplate": "create_project",
    "GetProjectConstruction": "get_construction",
    "GetRecordingTimesheet": "for_recording",
    "GetProjectTimesheet": "for_project",
    "GetTimesheetReport": "report",
    "GetTimesheetEntry": "get",
    "CreateTimesheetEntry": "create",
    "UpdateTimesheetEntry": "update",
    "GetProgressReport": "progress",
    "GetUpcomingSchedule": "upcoming",
    "GetAssignedTodos": "assigned",
    "GetOverdueTodos": "overdue",
    "GetPersonProgress": "person_progress",
    "SubscribeToCardColumn": "subscribe_to_column",
    "UnsubscribeFromCardColumn": "unsubscribe_from_column",
    "SetClientVisibility": "set_visibility",
    # Campfires
    "GetCampfire": "get",
    "ListCampfires": "list",
    "ListChatbots": "list_chatbots",
    "CreateChatbot": "create_chatbot",
    "GetChatbot": "get_chatbot",
    "UpdateChatbot": "update_chatbot",
    "DeleteChatbot": "delete_chatbot",
    "ListCampfireLines": "list_lines",
    "CreateCampfireLine": "create_line",
    "GetCampfireLine": "get_line",
    "DeleteCampfireLine": "delete_line",
    "ListCampfireUploads": "list_uploads",
    "CreateCampfireUpload": "create_upload",
    # Forwards
    "GetForward": "get",
    "ListForwards": "list",
    "GetForwardReply": "get_reply",
    "ListForwardReplies": "list_replies",
    "CreateForwardReply": "create_reply",
    "GetInbox": "get_inbox",
    # Uploads
    "GetUpload": "get",
    "UpdateUpload": "update",
    "ListUploads": "list",
    "CreateUpload": "create",
    "ListUploadVersions": "list_versions",
    "GetMessage": "get",
    "UpdateMessage": "update",
    "CreateMessage": "create",
    "ListMessages": "list",
    "PinMessage": "pin",
    "UnpinMessage": "unpin",
    "GetMessageBoard": "get",
    "GetMessageType": "get",
    "UpdateMessageType": "update",
    "CreateMessageType": "create",
    "ListMessageTypes": "list",
    "DeleteMessageType": "delete",
    "GetComment": "get",
    "UpdateComment": "update",
    "CreateComment": "create",
    "ListComments": "list",
    "ListProjectPeople": "list_for_project",
    "ListPingablePeople": "list_pingable",
    "ListAssignablePeople": "list_assignable",
    "GetSchedule": "get",
    "UpdateScheduleSettings": "update_settings",
    "GetScheduleEntry": "get_entry",
    "UpdateScheduleEntry": "update_entry",
    "CreateScheduleEntry": "create_entry",
    "ListScheduleEntries": "list_entries",
    "GetScheduleEntryOccurrence": "get_entry_occurrence",
    # Hill Charts
    "GetHillChart": "get",
    "UpdateHillChartSettings": "update_settings",
}

# Verb patterns for extracting method names
VERB_PATTERNS = [
    ("Subscribe", "subscribe"),
    ("Unsubscribe", "unsubscribe"),
    ("List", "list"),
    ("Get", "get"),
    ("Create", "create"),
    ("Update", "update"),
    ("Delete", "delete"),
    ("Trash", "trash"),
    ("Archive", "archive"),
    ("Unarchive", "unarchive"),
    ("Complete", "complete"),
    ("Uncomplete", "uncomplete"),
    ("Enable", "enable"),
    ("Disable", "disable"),
    ("Reposition", "reposition"),
    ("Move", "move"),
    ("Clone", "clone"),
    ("Set", "set"),
    ("Pin", "pin"),
    ("Unpin", "unpin"),
    ("Pause", "pause"),
    ("Resume", "resume"),
    ("Search", "search"),
]

SIMPLE_RESOURCES = {
    "todo", "todos", "todolist", "todolists", "todoset", "message", "messages",
    "comment", "comments", "card", "cards", "cardtable", "cardcolumn", "cardstep",
    "column", "step", "project", "projects", "person", "people", "campfire",
    "campfires", "chatbot", "chatbots", "webhook", "webhooks", "vault", "vaults",
    "document", "documents", "upload", "uploads", "schedule", "scheduleentry",
    "scheduleentries", "event", "events", "recording", "recordings", "template",
    "templates", "attachment", "question", "questions", "answer", "answers",
    "questionnaire", "subscription", "forward", "forwards", "inbox", "messageboard",
    "messagetype", "messagetypes", "tool", "lineupmarker", "clientapproval",
    "clientapprovals", "clientcorrespondence", "clientcorrespondences", "clientreply",
    "clientreplies", "forwardreply", "forwardreplies", "campfireline", "campfirelines",
    "todolistgroup", "todolistgroups", "todolistorgroup", "uploadversions",
}


PYTHON_KEYWORDS = frozenset({
    "False", "None", "True", "and", "as", "assert", "async", "await",
    "break", "class", "continue", "def", "del", "elif", "else", "except",
    "finally", "for", "from", "global", "if", "import", "in", "is",
    "lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try",
    "while", "with", "yield",
})


def to_snake_case(name: str) -> str:
    s = re.sub(r"([a-z\d])([A-Z])", r"\1_\2", name)
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", s)
    return s.lower()


def safe_python_name(snake_name: str) -> str:
    """Append trailing underscore if name is a Python keyword (PEP 8 convention)."""
    if snake_name in PYTHON_KEYWORDS:
        return snake_name + "_"
    return snake_name


def is_simple_resource(resource: str) -> bool:
    return resource.lower().replace("_", "") in SIMPLE_RESOURCES


def extract_method_name(operation_id: str) -> str:
    if operation_id in METHOD_NAME_OVERRIDES:
        return METHOD_NAME_OVERRIDES[operation_id]

    for prefix, method in VERB_PATTERNS:
        if operation_id.startswith(prefix):
            remainder = operation_id[len(prefix):]
            if not remainder:
                return method
            resource = to_snake_case(remainder)
            if is_simple_resource(resource):
                return method
            return f"{method}_{resource}"

    return to_snake_case(operation_id)


def schema_to_python_type(schema: dict | None) -> str:
    if not schema:
        return "str"
    if "$ref" in schema:
        return "dict"
    t = schema.get("type", "")
    if t == "integer":
        return "int"
    elif t == "boolean":
        return "bool"
    elif t == "array":
        return "list"
    elif t == "object":
        return "dict"
    return "str"


def convert_path(path: str) -> str:
    """Remove /{accountId} prefix and convert {camelCaseParam} to {snake_case_param}."""
    path = re.sub(r"^/\{accountId\}", "", path)

    def _replace(m: re.Match) -> str:
        return "{" + to_snake_case(m.group(1)) + "}"

    return re.sub(r"\{(\w+)\}", _replace, path)


def resolve_schema_ref(ref: dict, schemas: dict) -> dict | None:
    if "$ref" not in ref:
        return ref
    ref_path = ref["$ref"]
    if ref_path.startswith("#/components/schemas/"):
        schema_name = ref_path.rsplit("/", 1)[-1]
        return schemas.get(schema_name)
    return None


def extract_body_params(
    schema_ref: dict | None, schemas: dict,
) -> list[dict]:
    if not schema_ref:
        return []
    schema = resolve_schema_ref(schema_ref, schemas)
    if not schema or not schema.get("properties"):
        return []

    required_fields = set(schema.get("required", []))
    params = []
    for name, prop in schema["properties"].items():
        params.append({
            "name": name,
            "python_name": safe_python_name(to_snake_case(name)),
            "type": schema_to_python_type(prop),
            "required": name in required_fields,
        })
    return params


def find_service_for_operation(tag: str, operation_id: str) -> str:
    if tag in SERVICE_SPLITS:
        for svc, op_ids in SERVICE_SPLITS[tag].items():
            if operation_id in op_ids:
                return svc
    return TAG_TO_SERVICE.get(tag, tag.replace(" ", ""))


def parse_operation(
    path: str, method: str, operation: dict, schemas: dict,
) -> dict:
    operation_id = operation["operationId"]
    method_name = extract_method_name(operation_id)
    http_method = method.upper()

    # Path params (excluding accountId)
    path_params = []
    for p in operation.get("parameters", []):
        if p["in"] == "path" and p["name"] != "accountId":
            path_params.append({
                "name": p["name"],
                "python_name": to_snake_case(p["name"]),
                "type": schema_to_python_type(p.get("schema")),
            })

    # Query params
    query_params = []
    for p in operation.get("parameters", []):
        if p["in"] == "query":
            snake = to_snake_case(p["name"])
            query_params.append({
                "name": p["name"],
                "python_name": safe_python_name(snake),
                "type": schema_to_python_type(p.get("schema")),
                "required": p.get("required", False),
            })

    # Body params
    body_schema_ref = (operation.get("requestBody") or {}).get("content", {}).get(
        "application/json", {},
    ).get("schema")
    has_binary_body = bool(
        (operation.get("requestBody") or {}).get("content", {}).get("application/octet-stream"),
    )
    body_params = extract_body_params(body_schema_ref, schemas)

    # Response type
    success = operation.get("responses", {}).get("200") or operation.get("responses", {}).get("201")
    response_schema = (success or {}).get("content", {}).get("application/json", {}).get("schema")
    returns_void = response_schema is None
    returns_array = (response_schema or {}).get("type") == "array"

    # Pagination
    pagination = operation.get("x-basecamp-pagination")
    has_pagination = pagination is not None
    pagination_key = (pagination or {}).get("key")

    return {
        "operation_id": operation_id,
        "method_name": method_name,
        "http_method": http_method,
        "path": convert_path(path),
        "path_params": path_params,
        "query_params": query_params,
        "body_params": body_params,
        "has_body": len(body_params) > 0,
        "has_binary_body": has_binary_body,
        "returns_void": returns_void,
        "returns_array": returns_array,
        "is_mutation": http_method != "GET",
        "has_pagination": has_pagination,
        "pagination_key": pagination_key,
    }


def group_operations(spec: dict) -> dict[str, dict]:
    schemas = spec.get("components", {}).get("schemas", {})
    services: dict[str, dict] = {}

    for path, path_item in spec["paths"].items():
        for method in METHODS:
            operation = path_item.get(method)
            if not operation:
                continue

            tag = (operation.get("tags") or ["Untagged"])[0]
            parsed = parse_operation(path, method, operation, schemas)
            service_name = find_service_for_operation(tag, operation["operationId"])

            if service_name not in services:
                services[service_name] = {
                    "name": service_name,
                    "operations": [],
                }
            services[service_name]["operations"].append(parsed)

    return services


def python_type_hint(param_type: str) -> str:
    """Map a schema type string to a Python type hint for signatures."""
    return {
        "int": "int",
        "bool": "bool",
        "str": "str",
        "list": "list",
        "dict": "dict",
    }.get(param_type, "str")


def build_params(op: dict) -> list[str]:
    """Build keyword-only parameter list for a method."""
    params: list[str] = []

    # Path params
    for p in op["path_params"]:
        params.append(f"{p['python_name']}: int | str")

    # Binary upload params
    if op["has_binary_body"]:
        params.append("content: bytes")
        params.append("content_type: str")
    elif op["has_body"]:
        required = [b for b in op["body_params"] if b["required"]]
        optional = [b for b in op["body_params"] if not b["required"]]
        for b in required:
            hint = python_type_hint(b["type"])
            params.append(f"{b['python_name']}: {hint}")
        for b in optional:
            hint = python_type_hint(b["type"])
            params.append(f"{b['python_name']}: {hint} | None = None")

    # Query params
    required_qp = [q for q in op["query_params"] if q["required"]]
    optional_qp = [q for q in op["query_params"] if not q["required"]]
    for q in required_qp:
        hint = python_type_hint(q["type"])
        params.append(f"{q['python_name']}: {hint}")
    for q in optional_qp:
        hint = python_type_hint(q["type"])
        params.append(f"{q['python_name']}: {hint} | None = None")

    return params


def build_info_kwargs(op: dict, service_name: str) -> str:
    """Build OperationInfo constructor kwargs."""
    parts = [
        f'service="{service_name.lower()}"',
        f'operation="{op["method_name"]}"',
        f'is_mutation={op["is_mutation"]}',
    ]

    project_param = next((p for p in op["path_params"] if p["name"] == "projectId"), None)
    resource_param = next(
        (p for p in reversed(op["path_params"]) if p["name"] != "projectId"),
        None,
    )

    if project_param:
        parts.append(f"project_id={project_param['python_name']}")
    if resource_param:
        parts.append(f"resource_id={resource_param['python_name']}")

    return ", ".join(parts)


def build_path_expr(op: dict) -> str:
    """Build the f-string path expression."""
    path = op["path"]
    # If path contains interpolation vars, use f-string
    if "{" in path:
        return 'f"' + path + '"'
    return '"' + path + '"'


def _has_keyword_collision(params: list[dict]) -> bool:
    return any(p["name"] != p["python_name"] for p in params)


def _build_compact_or_dict(params: list[dict]) -> str:
    """Build self._compact(...) or a dict literal with inline None-stripping.

    Uses _compact() when all API names are valid Python identifiers,
    falls back to a dict comprehension when a name like 'from' collides
    with a Python keyword.
    """
    if not _has_keyword_collision(params):
        mappings = [f"{p['name']}={p['python_name']}" for p in params]
        return f"self._compact({', '.join(mappings)})"
    # Build {k: v for k, v in {...}.items() if v is not None}
    pairs = [f'"{p["name"]}": {p["python_name"]}' for p in params]
    return "{{k: v for k, v in {{{}}}.items() if v is not None}}".format(", ".join(pairs))


def build_body_expr(op: dict) -> str:
    """Build self._compact(...) expression for body params."""
    if not op["body_params"]:
        return "{}"
    return _build_compact_or_dict(op["body_params"])


def build_query_params_expr(op: dict) -> str:
    """Build self._compact(...) expression for query params."""
    return _build_compact_or_dict(op["query_params"])


def operation_kwarg(op: dict) -> str:
    """Return the operation= kwarg string for mutations, empty string for GETs."""
    if op["is_mutation"]:
        return f', operation="{op["operation_id"]}"'
    return ""


def is_paginated_list(op: dict) -> bool:
    return (op["returns_array"] or op["has_pagination"]) and not op["pagination_key"]


def is_wrapped_paginated(op: dict) -> bool:
    return op["has_pagination"] and op["pagination_key"] is not None


def return_type(op: dict) -> str:
    if op["returns_void"]:
        return "None"
    if is_wrapped_paginated(op):
        return "dict[str, Any]"
    if is_paginated_list(op):
        return "ListResult"
    return "dict[str, Any]"


def generate_method_body(op: dict, service_name: str, *, is_async: bool) -> list[str]:
    """Generate the method body lines (no signature, no def)."""
    lines: list[str] = []
    info_kwargs = build_info_kwargs(op, service_name)
    path_expr = build_path_expr(op)

    if is_wrapped_paginated(op):
        key = op["pagination_key"]
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_paginated_wrapped(")
            lines.append(f'            OperationInfo({info_kwargs}), {path_expr}, "{key}",')
            lines.append(f"            params={build_query_params_expr(op)},")
            lines.append("        )")
        else:
            lines.append(f'        return {_await(is_async)}self._request_paginated_wrapped(OperationInfo({info_kwargs}), {path_expr}, "{key}")')
    elif is_paginated_list(op):
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_paginated(")
            lines.append(f"            OperationInfo({info_kwargs}), {path_expr},")
            lines.append(f"            params={build_query_params_expr(op)},")
            lines.append("        )")
        else:
            lines.append(f"        return {_await(is_async)}self._request_paginated(OperationInfo({info_kwargs}), {path_expr})")
    elif op["has_binary_body"]:
        # Binary upload
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_raw(OperationInfo({info_kwargs}), {path_expr}, content=content, content_type=content_type, params={build_query_params_expr(op)}{operation_kwarg(op)})")
        else:
            lines.append(f"        return {_await(is_async)}self._request_raw(OperationInfo({info_kwargs}), {path_expr}, content=content, content_type=content_type{operation_kwarg(op)})")
    elif op["returns_void"]:
        if op["has_body"]:
            lines.append(f"        {_await(is_async)}self._request_void(OperationInfo({info_kwargs}), \"{op['http_method']}\", {path_expr}, json_body={build_body_expr(op)}{operation_kwarg(op)})")
        else:
            lines.append(f"        {_await(is_async)}self._request_void(OperationInfo({info_kwargs}), \"{op['http_method']}\", {path_expr}{operation_kwarg(op)})")
    else:
        # Standard request
        extra_kwargs = ""
        if op["has_body"]:
            extra_kwargs += f", json_body={build_body_expr(op)}"
        if op["query_params"]:
            extra_kwargs += f", params={build_query_params_expr(op)}"
        extra_kwargs += operation_kwarg(op)
        lines.append(f"        return {_await(is_async)}self._request(OperationInfo({info_kwargs}), \"{op['http_method']}\", {path_expr}{extra_kwargs})")

    return lines


def _await(is_async: bool) -> str:
    return "await " if is_async else ""


def generate_service_file(service: dict) -> str:
    """Generate complete Python file content for a service."""
    name = service["name"]
    sync_class = f"{name}Service"
    async_class = f"Async{name}Service"

    lines = [
        "# @generated from OpenAPI spec — do not edit manually",
        "",
        "from __future__ import annotations",
        "",
        "from typing import Any",
        "",
        "from basecamp.generated.services._base import BaseService",
        "from basecamp.generated.services._async_base import AsyncBaseService",
        "from basecamp._pagination import ListResult",
        "from basecamp.hooks import OperationInfo",
        "",
        "",
        f"class {sync_class}(BaseService):",
    ]

    for op in service["operations"]:
        lines.append("")
        params = build_params(op)
        ret = return_type(op)
        sig_params = ", ".join(["self"] + ([f"*, {', '.join(params)}"] if params else []))
        lines.append(f"    def {op['method_name']}({sig_params}) -> {ret}:")
        body = generate_method_body(op, name, is_async=False)
        lines.extend(body)

    lines.append("")
    lines.append("")
    lines.append(f"class {async_class}(AsyncBaseService):")

    for op in service["operations"]:
        lines.append("")
        params = build_params(op)
        ret = return_type(op)
        sig_params = ", ".join(["self"] + ([f"*, {', '.join(params)}"] if params else []))
        lines.append(f"    async def {op['method_name']}({sig_params}) -> {ret}:")
        body = generate_method_body(op, name, is_async=True)
        lines.extend(body)

    lines.append("")
    return "\n".join(lines)


def service_filename(name: str) -> str:
    """Convert service name to filename. Special case webhooks to avoid clash with webhooks/ package."""
    snake = to_snake_case(name)
    if snake == "webhooks":
        return "webhooks_service.py"
    return f"{snake}.py"


def generate_init_file(services: dict[str, dict]) -> str:
    """Generate __init__.py with all service imports."""
    lines = [
        "# @generated from OpenAPI spec — do not edit manually",
        "",
    ]

    imports: list[tuple[str, str, str]] = []
    for name in sorted(services):
        fname = service_filename(name)
        module = fname.removesuffix(".py")
        sync_class = f"{name}Service"
        async_class = f"Async{name}Service"
        imports.append((module, sync_class, async_class))

    for module, sync_cls, async_cls in imports:
        lines.append(f"from basecamp.generated.services.{module} import {sync_cls}, {async_cls}")

    all_names = []
    for _, sync_cls, async_cls in imports:
        all_names.extend([f'    "{sync_cls}"', f'    "{async_cls}"'])

    lines.append("")
    lines.append("__all__ = [")
    for entry in all_names:
        lines.append(f"{entry},")
    lines.append("]")
    lines.append("")

    return "\n".join(lines)


def main() -> None:
    import argparse
    parser = argparse.ArgumentParser(description="Generate Python service classes from OpenAPI spec")
    parser.add_argument("--openapi", default=str(Path(__file__).parent.parent.parent / "openapi.json"))
    parser.add_argument("--output", default=str(Path(__file__).parent.parent / "src" / "basecamp" / "generated" / "services"))
    args = parser.parse_args()

    openapi_path = Path(args.openapi)
    output_dir = Path(args.output)

    if not openapi_path.exists():
        print(f"Error: OpenAPI file not found: {openapi_path}", file=sys.stderr)
        sys.exit(1)

    with open(openapi_path, encoding="utf-8") as f:
        spec = json.load(f)

    services = group_operations(spec)
    output_dir.mkdir(parents=True, exist_ok=True)

    total_ops = 0
    generated_files: list[str] = []

    for name, service in sorted(services.items()):
        code = generate_service_file(service)
        fname = service_filename(name)
        filepath = output_dir / fname
        filepath.write_text(code, encoding="utf-8")
        op_count = len(service["operations"])
        total_ops += op_count
        generated_files.append(fname)
        print(f"Generated {fname} ({op_count} operations)")

    # Generate __init__.py
    init_code = generate_init_file(services)
    init_path = output_dir / "__init__.py"
    init_path.write_text(init_code, encoding="utf-8")
    print(f"Generated __init__.py")

    print(f"\nGenerated {len(services)} services with {total_ops} operations total.")


if __name__ == "__main__":
    main()
