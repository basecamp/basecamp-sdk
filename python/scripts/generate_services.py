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

import ast
import json
import re
import sys
import textwrap
from pathlib import Path

# Make the shared generator helper importable whether this file is run as a
# script (its dir is already sys.path[0]) or loaded via importlib in tests.
sys.path.insert(0, str(Path(__file__).parent))
from gen_common import escape_py_string  # noqa: E402

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
            "ListCampfireLines", "CreateCampfireLine", "GetCampfireLine", "UpdateCampfireLine", "DeleteCampfireLine",
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
        "Wormholes": ["CreateWormhole", "UpdateWormhole", "DeleteWormhole"],
    },
    "Files": {
        "Attachments": ["CreateAttachment"],
        "Uploads": ["GetUpload", "UpdateUpload", "ListUploads", "CreateUpload", "ListUploadVersions", "CreateUploadVersion"],
        "Vaults": ["GetVault", "UpdateVault", "ListVaults", "CreateVault"],
        "Documents": ["GetDocument", "ReplaceDocument", "ListDocuments", "CreateDocument"],
        "CloudFiles": ["GetCloudFile", "CreateCloudFile", "UpdateCloudFile"],
        "GoogleDocuments": ["GetGoogleDocument", "CreateGoogleDocument", "UpdateGoogleDocument"],
    },
    "Automation": {
        "Tools": ["GetTool", "UpdateTool", "DeleteTool", "CreateTool", "EnableTool", "DisableTool", "RepositionTool"],
        "Recordings": ["ArchiveRecording", "UnarchiveRecording", "TrashRecording", "ListRecordings", "SpotlightRecording", "UnspotlightRecording"],
        "Webhooks": ["ListWebhooks", "CreateWebhook", "GetWebhook", "UpdateWebhook", "DeleteWebhook"],
        "Events": ["ListEvents"],
        "Lineup": ["CreateLineupMarker", "UpdateLineupMarker", "DeleteLineupMarker"],
        "Search": ["Search", "GetSearchMetadata"],
        "Templates": [
            "ListTemplates", "CreateTemplate", "GetTemplate", "UpdateTemplate",
            "DeleteTemplate", "CreateProjectFromTemplate", "GetProjectConstruction",
            "GetTemplateLibrary", "CreateTemplateLibraryCopy", "GetTemplateLibraryCopy",
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
            "CreateScheduleEntry", "GetScheduleEntry", "ReplaceScheduleEntry",
            "GetScheduleEntryOccurrence",
        ],
        "Timesheets": [
            "GetRecordingTimesheet", "GetProjectTimesheet", "GetTimesheetReport",
            "GetTimesheetEntry", "CreateTimesheetEntry", "UpdateTimesheetEntry",
            "DestroyTimesheetEntry",
        ],
    },
    "ClientFeatures": {
        "ClientApprovals": ["ListClientApprovals", "GetClientApproval"],
        "ClientCorrespondences": ["ListClientCorrespondences", "GetClientCorrespondence"],
        "ClientReplies": ["ListClientReplies", "GetClientReply"],
        "ClientVisibility": ["SetClientVisibility"],
    },
    "Todos": {
        "Todos": ["ListTodos", "CreateTodo", "CreateTodosetTodo", "GetTodo", "ReplaceTodo", "CompleteTodo", "UncompleteTodo"],
        "Todolists": ["GetTodolistOrGroup", "UpdateTodolistOrGroup", "ListTodolists", "CreateTodolist", "RepositionTodolist"],
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
    "SpotlightRecording": "spotlight",
    "UnspotlightRecording": "unspotlight",
    "GetMyProfile": "my_profile",
    "GetTodolistOrGroup": "get",
    # The plain `update` name belongs to the merge-safe composite; the raw
    # single-PUT path keeps a name that says what it does. BC3 rebuilds the
    # todolist from the permitted params, so omission clears. See #374.
    "UpdateTodolistOrGroup": "replace",
    "SetCardColumnColor": "set_color",
    "EnableCardColumnOnHold": "enable_on_hold",
    "DisableCardColumnOnHold": "disable_on_hold",
    "RepositionCardStep": "reposition",
    "CreateCardStep": "create",
    "UpdateCardStep": "update",
    # The plain `update` name belongs to the composite; the raw single-PUT
    # path keeps a distinct name. The composite defended against BC3 clearing an
    # omitted due_on (#467); basecamp/bc3#12521 made that representation
    # presence-aware, so the two now behave identically.
    "UpdateCard": "update_verbatim",
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
    "GetTemplateLibrary": "get_library",
    "CreateTemplateLibraryCopy": "create_library_copy",
    "GetTemplateLibraryCopy": "get_library_copy",
    "GetRecordingTimesheet": "for_recording",
    "GetProjectTimesheet": "for_project",
    "GetTimesheetReport": "report",
    "GetTimesheetEntry": "get",
    "CreateTimesheetEntry": "create",
    "UpdateTimesheetEntry": "update",
    "DestroyTimesheetEntry": "destroy",
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
    "UpdateCampfireLine": "update_line",
    "DeleteCampfireLine": "delete_line",
    "ListCampfireUploads": "list_uploads",
    "CreateCampfireUpload": "create_upload",
    # Forwards
    "GetForward": "get",
    "ListForwards": "list",
    "GetForwardReply": "get_reply",
    "ListForwardReplies": "list_replies",
    "GetInbox": "get_inbox",
    # Uploads
    "GetUpload": "get",
    "UpdateUpload": "update",
    "ListUploads": "list",
    "CreateUpload": "create",
    "ListUploadVersions": "list_versions",
    "CreateUploadVersion": "create_version",
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
    # The plain `update_entry` name belongs to the merge-safe composite; the raw
    # single-PUT path keeps a name that says what it does. Without the override
    # the algorithm yields a bare `replace` (scheduleentry is a SIMPLE_RESOURCE),
    # which reads as "replace the schedule". See #547.
    "ReplaceScheduleEntry": "replace_entry",
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
    ("Replace", "replace"),
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
    "wormhole", "wormholes",
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
        inner = schema_to_python_type(schema.get("items"))
        return f"list[{inner}]"
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
        # A bare `$ref` property carries no description of its own — read it
        # off the referenced schema (single-level: the spec has no ref-to-ref
        # chains). A sibling description on the property itself still wins.
        description = prop.get("description")
        if description is None and "$ref" in prop:
            target = resolve_schema_ref(prop, schemas)
            if target:
                description = target.get("description")
        params.append({
            "name": name,
            "python_name": safe_python_name(to_snake_case(name)),
            "type": schema_to_python_type(prop),
            "required": name in required_fields,
            "description": description,
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
                "description": p.get("description"),
            })

    # Query params
    query_params = []
    for p in operation.get("parameters", []):
        if p["in"] == "query":
            # Bracketed array wire names (e.g. `bucket_ids[]`) keep the `[]` on
            # the raw `name` (used as the params-dict wire key so httpx sends
            # `bucket_ids%5B%5D=…`), but strip it for the public `python_name`
            # kwarg identifier (`bucket_ids[]` is not a valid identifier).
            snake = to_snake_case(re.sub(r"\[\]$", "", p["name"]))
            schema = p.get("schema") or {}
            query_params.append({
                "name": p["name"],
                "python_name": safe_python_name(snake),
                "type": schema_to_python_type(p.get("schema")),
                "required": p.get("required", False),
                "deprecated": bool(p.get("deprecated") or schema.get("deprecated")),
                "deprecation_reason": p.get("x-deprecated-reason") or schema.get("x-deprecated-reason"),
                "description": p.get("description"),
            })

    # Body params
    body_schema_ref = (operation.get("requestBody") or {}).get("content", {}).get(
        "application/json", {},
    ).get("schema")
    has_binary_body = bool(
        (operation.get("requestBody") or {}).get("content", {}).get("application/octet-stream"),
    )
    multipart_content = (operation.get("requestBody") or {}).get("content", {}).get("multipart/form-data")
    has_multipart_body = bool(multipart_content)
    multipart_field = (
        (operation.get("x-basecamp-multipart") or {}).get("field", "file")
        if has_multipart_body
        else None
    )
    body_params = extract_body_params(body_schema_ref, schemas)

    # Response type
    success = operation.get("responses", {}).get("200") or operation.get("responses", {}).get("201")
    response_schema = (success or {}).get("content", {}).get("application/json", {}).get("schema")
    returns_void = response_schema is None
    # Resolve through a $ref so bare-array ResponseContent aliases (e.g. the
    # unpaginated overdue lists, whose response is a $ref to `Todo[]`) are
    # detected as arrays rather than falling through to a dict return type.
    resolved_response = response_schema
    if isinstance(response_schema, dict) and "$ref" in response_schema:
        resolved_response = resolve_schema_ref(response_schema, schemas) or response_schema
    returns_array = (resolved_response or {}).get("type") == "array"

    # Pagination
    pagination = operation.get("x-basecamp-pagination")
    has_pagination = pagination is not None
    pagination_key = (pagination or {}).get("key")

    return {
        "operation_id": operation_id,
        "description": operation.get("description") or "",
        "method_name": method_name,
        "http_method": http_method,
        "path": convert_path(path),
        "path_params": path_params,
        "query_params": query_params,
        "body_params": body_params,
        "has_body": len(body_params) > 0 or has_multipart_body,
        "has_binary_body": has_binary_body,
        "has_multipart_body": has_multipart_body,
        "multipart_field": multipart_field,
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
    # Parametrized list types (e.g. `list[int]`, `list[str]`) pass through.
    if param_type.startswith("list"):
        return param_type
    return {
        "int": "int",
        "bool": "bool",
        "str": "str",
        "dict": "dict",
    }.get(param_type, "str")


def escape_docstring_text(text: str) -> str:
    """Escape multi-line text for interpolation into a triple-quoted docstring.

    Unlike ``escape_py_string`` (which flattens to a single line for plain
    string literals), this keeps real newlines: it escapes backslashes (so no
    accidental escape sequences form), neutralizes any embedded triple-quote
    that would close the docstring early, and normalizes the control characters
    that would confuse the emitted source. Any remaining C0 control or DEL is
    dropped — a literal NUL in particular makes the whole module uncompilable
    ("source code string cannot contain null bytes"). The closing delimiter is
    always emitted on its own line, so a trailing double-quote in the text is
    safe.
    """
    text = (
        text.replace("\\", "\\\\")
        .replace('"""', '\\"\\"\\"')
        .replace("\r\n", "\n")
        .replace("\r", "\n")
        .replace("\t", "    ")
    )
    return re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", "", text)


def _split_paragraphs(text: str) -> list[str]:
    return [p.strip("\n") for p in re.split(r"\n\s*\n", text.strip()) if p.strip()]


def _fallback_param_doc(python_name: str) -> str:
    """Derived Args line for a parameter the spec leaves undescribed.

    Path params carry no descriptions in the OpenAPI spec at all, so this
    humanized fallback ("project_id" -> "The project id.") is what documents
    them — matching the Ruby and TypeScript generators' fallback convention.
    The trailing underscore a Python-keyword collision appends (``from_``) is
    stripped before humanizing, so it reads "The from." not "The from .".
    """
    return f"The {python_name.rstrip('_').replace('_', ' ')}."


def build_params(op: dict) -> list[dict]:
    """Build the keyword-only parameter list for a method.

    Returns one dict per parameter, in signature order: ``sig`` is the
    signature fragment, ``python_name``/``description`` feed the docstring's
    Args section — a single ordered list so the two can never disagree.
    """
    params: list[dict] = []

    def add(sig: str, python_name: str, description: str | None) -> None:
        params.append({"sig": sig, "python_name": python_name, "description": description})

    # Path params — use the schema type, not a blanket int | str
    for p in op["path_params"]:
        add(f"{p['python_name']}: {p['type']}", p["python_name"], p.get("description"))

    # Binary upload params
    if op["has_binary_body"]:
        add("content: bytes", "content", "Raw bytes of the file to upload.")
        add("content_type: str", "content_type", 'MIME content type of the upload (e.g. "image/png").')
    elif op.get("has_multipart_body"):
        add("content: bytes", "content", "Raw bytes of the file to upload.")
        add("filename: str", "filename", "Filename for the uploaded file.")
        add("content_type: str", "content_type", 'MIME content type of the upload (e.g. "image/png").')
    elif op["has_body"]:
        required = [b for b in op["body_params"] if b["required"]]
        optional = [b for b in op["body_params"] if not b["required"]]
        for b in required:
            hint = python_type_hint(b["type"])
            add(f"{b['python_name']}: {hint}", b["python_name"], b.get("description"))
        for b in optional:
            hint = python_type_hint(b["type"])
            add(f"{b['python_name']}: {hint} | None = None", b["python_name"], b.get("description"))

    # Query params
    required_qp = [q for q in op["query_params"] if q["required"]]
    optional_qp = [q for q in op["query_params"] if not q["required"]]
    for q in required_qp:
        hint = python_type_hint(q["type"])
        add(f"{q['python_name']}: {hint}", q["python_name"], q.get("description"))
    for q in optional_qp:
        hint = python_type_hint(q["type"])
        add(f"{q['python_name']}: {hint} | None = None", q["python_name"], q.get("description"))

    # Paginated operations take a client-side cap on collected items,
    # matching the maxItems pagination option in the other SDKs.
    if op["has_pagination"]:
        max_items_doc = (
            "Client-side cap on the number of items collected across pages; "
            "None or a non-positive value means no item cap. "
            "Collection is always bounded by config.max_pages."
        )
        # SPEC section 8: a positive `page` pins a single page, so the
        # follow loop stops after one request. Only worth saying on the
        # operations that actually take a `page` param.
        if any(q["python_name"] == "page" for q in op["query_params"]):
            max_items_doc += " A positive page argument fetches exactly that one page."
        add("max_items: int | None = None", "max_items", max_items_doc)

    return params


def _wrap_args_entry(python_name: str, description: str) -> list[str]:
    """Wrap one Args entry to Google style: entry at 12 spaces, continuations at 16.

    Long tokens and hyphenated words stay whole (descriptions carry URLs,
    ``x-header-…`` names, and ``Module::Class`` references that must not be
    split mid-token); an over-long token overflows its line instead.
    """
    text = " ".join(escape_docstring_text(description).split())
    wrapped = textwrap.wrap(
        f"{python_name}: {text}",
        width=88,
        subsequent_indent="    ",
        break_long_words=False,
        break_on_hyphens=False,
    ) or [f"{python_name}:"]
    return [f"            {line}" for line in wrapped]


def _deprecation_section(op: dict) -> list[str]:
    """Docstring section flagging deprecated query params.

    Documentation-only (see #406): a TypedDict/kwarg has no per-parameter
    deprecation directive, and an RST ``.. deprecated::`` inside a ``:param:``
    is malformed, so this is real prose listing each deprecated parameter and
    its replacement. Emitted only when the operation has at least one
    deprecated param, keyed on that flag rather than a specific method name.
    """
    deprecated = [q for q in op["query_params"] if q.get("deprecated")]
    if not deprecated:
        return []
    lines = ["        Deprecated parameters (prefer the replacement):", ""]
    for q in deprecated:
        reason = escape_py_string(q.get("deprecation_reason") or "deprecated")
        lines.append(f"        - {q['name']}: {reason}")
    return lines


def method_docstring(op: dict, params: list[dict]) -> list[str]:
    """Emit the method docstring: summary, extended description, deprecation
    note, and a Google-style Args section.

    The summary is the operation description's whole first paragraph (never
    the Ruby generators' first-line truncation — first lines here are usually
    unterminated summary phrases), with a period appended when the paragraph
    lacks terminal punctuation. Remaining paragraphs follow verbatim, except
    the "**Pagination**" boilerplate on paginated operations: it instructs
    manual Link-header following, which these methods do automatically —
    ``max_items`` in Args documents the collected-pages behavior instead.
    """
    description = escape_docstring_text(op["description"]).strip() or f"{op['operation_id']} operation."
    paragraphs = _split_paragraphs(description)
    if op["has_pagination"]:
        paragraphs = [p for p in paragraphs if not p.lstrip().startswith("**Pagination**")]

    summary = paragraphs[0] if paragraphs else f"{op['operation_id']} operation."
    if summary[-1] not in ".!?":
        summary += "."

    body: list[str] = []
    for paragraph in [summary] + paragraphs[1:]:
        if body:
            body.append("")
        body.extend(f"        {line.rstrip()}".rstrip() for line in paragraph.split("\n"))

    deprecation = _deprecation_section(op)
    if deprecation:
        body.append("")
        body.extend(deprecation)

    if params:
        body.append("")
        body.append("        Args:")
        for p in params:
            body.extend(_wrap_args_entry(p["python_name"], p["description"] or _fallback_param_doc(p["python_name"])))

    if len(body) == 1:
        return [f'        """{body[0].strip()}"""']
    lines = [f'        """{body[0].strip()}']
    lines.extend(body[1:])
    lines.append('        """')
    return lines


def build_info_kwargs(op: dict, service_name: str) -> str:
    """Build OperationInfo constructor kwargs."""
    parts = [
        f'service="{service_name.lower()}"',
        f'operation="{op["method_name"]}"',
        f'is_mutation={op["is_mutation"]}',
    ]

    project_param = next((p for p in op["path_params"] if p["name"] in ("projectId", "bucketId")), None)
    resource_param = next(
        (
            p
            for p in reversed(op["path_params"])
            if p["name"] not in ("projectId", "bucketId")
            and (p["name"].endswith("Id") or p["name"] == "id")
        ),
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
    """Return the ``operation=`` kwarg string carrying the canonical (PascalCase)
    operationId. The transport uses it to look up per-operation metadata — the
    idempotency gate for mutations and the retry.max ceiling for every retryable
    request (GET/list/pagination included). OperationInfo.operation is the
    snake_case display name and is NOT a metadata key, so it must not be used for
    lookups."""
    return f', operation="{op["operation_id"]}"'


def is_paginated_list(op: dict) -> bool:
    # Link-header paginated: the operation is explicitly marked paginated. A bare
    # array without that marker is a complete, unpaginated collection (see
    # is_unpaginated_array) and must NOT follow Link headers.
    return op["has_pagination"] and not op["pagination_key"]


def is_unpaginated_array(op: dict) -> bool:
    # Returns a bare array but is not marked paginated: the whole collection comes
    # back in a single response (e.g. the overdue todo/card feeds). Fetched with a
    # single request, no Link-following — matching the other SDKs' plain decode.
    return op["returns_array"] and not op["has_pagination"] and not op["pagination_key"]


def is_wrapped_paginated(op: dict) -> bool:
    return op["has_pagination"] and op["pagination_key"] is not None


def return_type(op: dict) -> str:
    if op["returns_void"]:
        return "None"
    if is_wrapped_paginated(op):
        return "dict[str, Any]"
    if is_paginated_list(op) or is_unpaginated_array(op):
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
            lines.append(f"            params={build_query_params_expr(op)}, max_items=max_items{operation_kwarg(op)},")
            lines.append("        )")
        else:
            lines.append(f'        return {_await(is_async)}self._request_paginated_wrapped(OperationInfo({info_kwargs}), {path_expr}, "{key}", max_items=max_items{operation_kwarg(op)})')
    elif is_paginated_list(op):
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_paginated(")
            lines.append(f"            OperationInfo({info_kwargs}), {path_expr},")
            lines.append(f"            params={build_query_params_expr(op)}, max_items=max_items{operation_kwarg(op)},")
            lines.append("        )")
        else:
            lines.append(f"        return {_await(is_async)}self._request_paginated(OperationInfo({info_kwargs}), {path_expr}, max_items=max_items{operation_kwarg(op)})")
    elif is_unpaginated_array(op):
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_list(")
            lines.append(f"            OperationInfo({info_kwargs}), {path_expr},")
            lines.append(f"            params={build_query_params_expr(op)}{operation_kwarg(op)},")
            lines.append("        )")
        else:
            lines.append(f"        return {_await(is_async)}self._request_list(OperationInfo({info_kwargs}), {path_expr}{operation_kwarg(op)})")
    elif op["has_binary_body"]:
        # Binary upload
        if op["query_params"]:
            lines.append(f"        return {_await(is_async)}self._request_raw(OperationInfo({info_kwargs}), {path_expr}, content=content, content_type=content_type, params={build_query_params_expr(op)}{operation_kwarg(op)})")
        else:
            lines.append(f"        return {_await(is_async)}self._request_raw(OperationInfo({info_kwargs}), {path_expr}, content=content, content_type=content_type{operation_kwarg(op)})")
    elif op.get("has_multipart_body"):
        # Multipart form-data upload
        field = op["multipart_field"]
        lines.append(f'        {_await(is_async)}self._request_multipart_void(OperationInfo({info_kwargs}), "{op["http_method"]}", {path_expr}, field="{field}", content=content, filename=filename, content_type=content_type{operation_kwarg(op)})')
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


# SENTINEL. The import package every generated service module lives in, and the
# one string the stale-file sweep needs to survive unchanged between two runs.
# The barrel this generator writes imports its siblings from here; the next run
# reads those imports back out (`previously_emitted_modules`) to learn what the
# last run emitted. So this is not merely a code path — it is the format of a
# record written by one run and read by the next.
#
# Change it and the run that lands the change reads an unparseable-to-it barrel,
# finds nothing, and sweeps nothing: any module the same change drops must be
# removed by hand, once. That is the whole residual, and it is why the record is
# a package path rather than a comment. Renaming this package rewrites the
# imports of every generated module and of every consumer that imports a service
# directly — it is not a thing that happens as a drive-by edit to a preamble,
# which is exactly what the previous content-fingerprint predicate was.
SERVICE_PACKAGE = "basecamp.generated.services"

SERVICE_MODULE_MARKER = "# @generated from OpenAPI spec — do not edit manually"
SERVICE_MODULE_BASE_IMPORT = f"from {SERVICE_PACKAGE}._base import BaseService"

# The barrel's filename, named once because three things spell it: the writer,
# the reader that recovers the previous run's roster from it, and `main`, which
# has to count it as this run's output *before* it has been written.
SERVICE_BARREL = "__init__.py"


def generate_service_file(service: dict) -> str:
    """Generate complete Python file content for a service."""
    name = service["name"]
    sync_class = f"{name}Service"
    async_class = f"Async{name}Service"

    lines = [
        SERVICE_MODULE_MARKER,
        "",
        "from __future__ import annotations",
        "",
        "from typing import Any",
        "",
        SERVICE_MODULE_BASE_IMPORT,
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
        sig_fragments = [p["sig"] for p in params]
        sig_params = ", ".join(["self"] + ([f"*, {', '.join(sig_fragments)}"] if params else []))
        lines.append(f"    def {op['method_name']}({sig_params}) -> {ret}:")
        lines.extend(method_docstring(op, params))
        body = generate_method_body(op, name, is_async=False)
        lines.extend(body)

    lines.append("")
    lines.append("")
    lines.append(f"class {async_class}(AsyncBaseService):")

    for op in service["operations"]:
        lines.append("")
        params = build_params(op)
        ret = return_type(op)
        sig_fragments = [p["sig"] for p in params]
        sig_params = ", ".join(["self"] + ([f"*, {', '.join(sig_fragments)}"] if params else []))
        lines.append(f"    async def {op['method_name']}({sig_params}) -> {ret}:")
        lines.extend(method_docstring(op, params))
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
    """Generate __init__.py with all service imports.

    Doubles as this generator's record of what it emitted: `remove_stale_files`
    reads the *previous* run's barrel to learn which modules to sweep, so the
    import statements below are read as well as written. Emitted from
    `SERVICE_PACKAGE`, which is where that coupling is documented.

    Written last, after the sweep, so that the record it replaces outlives every
    deletion it authorises — see `main`.
    """
    lines = [
        SERVICE_MODULE_MARKER,
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
        lines.append(f"from {SERVICE_PACKAGE}.{module} import {sync_cls}, {async_cls}")

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


def previously_emitted_modules(output_dir: Path) -> set[str]:
    """The service modules the barrel already on disk says the last run emitted.

    `__init__.py` is this generator's own committed record of its own output:
    every run rewrites it whole from the current mapping, importing exactly the
    modules it just wrote. Read while the *previous* run's copy is still on disk,
    it answers the only question the sweep has — which modules did we produce
    last time — from a record we wrote, rather than by guessing from the contents
    of files we find. `main` does not overwrite it until the sweep has finished,
    so this read and every deletion it feeds happen against the same record.

    That is the whole reason this replaced a content predicate. Recognising past
    output by the text in its preamble couples deletion to strings that move: a
    change that renames a service *and* touches the `@generated` line or the base
    import in the same commit leaves the dropped module carrying the old preamble,
    unrecognised, unswept, and unfixable by rerunning the generator. Names in a
    barrel do not move when a preamble does.

    It also removes the reason the content predicate needed a second clause. Point
    `--output` at `.../generated` instead of `.../generated/services` and the
    barrel there is the empty package marker: no imports, empty drop list, nothing
    deleted, `types.py` untouched. The wrong-directory slip is answered by the
    mechanism rather than by a guard bolted to it.

    Read via `ast` rather than a regex so line wrapping, parenthesised imports and
    import ordering are all beneath notice; only the package path itself is load
    bearing, and `SERVICE_PACKAGE` carries that warning. A barrel that is missing,
    unreadable or unparseable yields an empty set: this run sweeps nothing, which
    is the direction to fail in when the alternative is unlinking on a guess.
    """
    barrel = output_dir / SERVICE_BARREL
    try:
        tree = ast.parse(barrel.read_text(encoding="utf-8"))
    except (OSError, SyntaxError, ValueError):
        return set()

    prefix = f"{SERVICE_PACKAGE}."
    modules = set()
    for node in tree.body:
        if isinstance(node, ast.ImportFrom) and node.level == 0 and node.module and node.module.startswith(prefix):
            tail = node.module[len(prefix):]
            if "." not in tail:
                modules.add(tail)
    return modules


def remove_stale_files(output_dir: Path, previous_modules: set[str], emitted_files: list[str]) -> None:
    """Delete modules this generator used to emit and no longer does.

    Without this, removing or renaming a service in the mapping leaves the old
    module on disk forever: `__init__.py` is rewritten whole and stops importing
    it, but nothing deletes it, so `git status` has nothing to report and a CI
    step that regenerates in place cannot see the corpse (#757).

    The sweep set is exactly `previous_modules - this run's output`. Nothing is
    inspected, nothing is pattern-matched, and no file this generator did not
    record emitting is a candidate — a hand-written module in this directory is
    safe because it was never in the barrel, not because it failed a test.

    Called by `main` while the barrel that produced `previous_modules` is still
    the one on disk, which is what makes a failed run retryable: this loop can
    stop anywhere — an unlink that raises, a process killed between two of them —
    and the record naming everything it had not got to yet is still intact, so
    the next run nominates the remainder and finishes. Sweeping after the barrel
    was replaced would destroy that record first, and a module left behind by an
    interrupted sweep would never be nominated again by any run.

    What that does not cover, stated plainly: a module the barrel never named is
    never swept. Reachable two ways — a barrel hand-deleted or truncated between
    runs, and a module dropped while this sweep was absent or broken (that is,
    every corpse that predates the sweep). Both still need one manual `rm`. The
    trade is deliberate: a drop list can only be too short, where a content
    predicate can be wrong in the other direction, and this generator has already
    demonstrated once what it costs to delete another generator's output.

    `_base.py` and `_async_base.py` are hand-written infrastructure living under
    `generated/` by exception (AGENTS.md Hard Rule 1), and they are the only files
    here that a regeneration could not put back. The barrel does not import them,
    so they are already outside the drop list; the `_` prefix check is kept anyway
    because the drop list is read off disk, and the one irreversible loss in this
    directory is worth not making contingent on a file's contents being intact.
    """
    emitted = {Path(name).stem for name in emitted_files}

    for module in sorted(previous_modules - emitted):
        if module.startswith("_"):
            continue
        stale = output_dir / f"{module}.py"
        if stale.is_file():
            stale.unlink()
            print(f"Removed stale {stale.name}")


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

    # The barrel on disk right now is the last run's record of what it emitted,
    # and this run replaces it. Read it first; nothing below writes it until the
    # sweep has finished with it.
    previous_modules = previously_emitted_modules(output_dir)

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

    # This run's output in full, named at the point the sweep needs it rather
    # than accumulated by the writes: the barrel is written last now, so a list
    # that grew as a side effect of writing would be short by exactly the barrel
    # at the moment the sweep subtracts it from the previous record. Naming it
    # here says what `emitted_files` means, which appending it after the fact did
    # not.
    #
    # It is belt and braces, and measured so rather than assumed: drop the barrel
    # from this list and no case in the fixture moves. A barrel never imports
    # itself, so `__init__` cannot be in `previous_modules`, and the `_` prefix
    # guard in the sweep would decline it anyway. Both of those are properties of
    # other code; this is the statement local to the sweep.
    emitted_files = [*generated_files, SERVICE_BARREL]

    # Sweep while the previous barrel is still on disk, and only then replace it.
    # That record is the sole thing that can nominate a stale module, so it has
    # to outlive the deletions it authorises: an interrupted run — a raising
    # unlink, a killed process — then leaves the record intact and the next clean
    # run finishes the job. Rewriting the barrel first would erase the record
    # before the work it describes was done, and anything the interrupted sweep
    # missed would be invisible to every later run and removable only by hand.
    #
    # The trade, stated rather than left to be discovered: between the first
    # unlink and the barrel write the package on disk imports a module that is
    # gone, so a run killed in that window leaves it unimportable. That is loud
    # and one rerun away from fixed, where the ordering it replaces left a
    # package that imported cleanly and was quietly wrong forever.
    remove_stale_files(output_dir, previous_modules, emitted_files)

    init_path = output_dir / SERVICE_BARREL
    init_path.write_text(generate_init_file(services), encoding="utf-8")
    print(f"Generated {SERVICE_BARREL}")

    print(f"\nGenerated {len(services)} services with {total_ops} operations total.")


if __name__ == "__main__":
    main()
