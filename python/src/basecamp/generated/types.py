# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any, NotRequired, TypedDict

WebhookHeadersMap = dict[str, str]


class Account(TypedDict):
    active: NotRequired[bool]
    created_at: str
    frozen: NotRequired[bool]
    id: int
    limits: NotRequired[AccountLimits]
    logo: NotRequired[str]
    name: str
    owner_name: NotRequired[str]
    paused: NotRequired[bool]
    settings: NotRequired[AccountSettings]
    subscription: NotRequired[AccountSubscription]
    trial: NotRequired[bool]
    trial_ends_on: NotRequired[str]
    updated_at: str


class AccountLimits(TypedDict):
    can_create_projects: NotRequired[bool]
    can_create_users: NotRequired[bool]
    can_pin_projects: NotRequired[bool]
    can_upload_files: NotRequired[bool]


class AccountSettings(TypedDict):
    company_hq_enabled: NotRequired[bool]
    projects_enabled: NotRequired[bool]
    teams_enabled: NotRequired[bool]


class AccountSubscription(TypedDict):
    clients: NotRequired[bool]
    logo: NotRequired[bool]
    project_limit: NotRequired[int]
    proper_name: NotRequired[str]
    short_name: NotRequired[str]
    teams: NotRequired[bool]
    templates: NotRequired[bool]
    timesheet: NotRequired[bool]


class Assignable(TypedDict):
    app_url: NotRequired[str]
    assignees: NotRequired[list[Person]]
    bucket: NotRequired[TodoBucket]
    due_on: NotRequired[str]
    id: NotRequired[int]
    parent: NotRequired[TodoParent]
    starts_on: NotRequired[str]
    title: NotRequired[str]
    type: NotRequired[str]
    url: NotRequired[str]


class BadRequestErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class Boost(TypedDict):
    booster: NotRequired[Person]
    content: NotRequired[str]
    created_at: str
    id: int
    recording: NotRequired[RecordingParent]


class Campfire(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    files_url: NotRequired[str]
    id: int
    inherits_status: bool
    lines_url: NotRequired[str]
    position: NotRequired[int]
    status: str
    subscription_url: NotRequired[str]
    title: str
    topic: NotRequired[str]
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CampfireLine(TypedDict):
    app_url: str
    attachments: NotRequired[list[CampfireLineAttachment]]
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    content: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CampfireLineAttachment(TypedDict):
    byte_size: NotRequired[int]
    content_type: NotRequired[str]
    download_url: NotRequired[str]
    filename: NotRequired[str]
    title: NotRequired[str]
    url: NotRequired[str]


class Card(TypedDict):
    app_url: str
    assignees: NotRequired[list[Person]]
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    completed: NotRequired[bool]
    completed_at: NotRequired[str]
    completer: NotRequired[Person]
    completion_subscribers: NotRequired[list[Person]]
    completion_url: NotRequired[str]
    content: NotRequired[str]
    created_at: str
    creator: Person
    description: NotRequired[str]
    due_on: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    position: NotRequired[int]
    status: str
    steps: NotRequired[list[CardStep]]
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CardColumn(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    cards_count: NotRequired[int]
    cards_url: NotRequired[str]
    color: NotRequired[str]
    comments_count: NotRequired[int]
    created_at: str
    creator: Person
    description: NotRequired[str]
    id: int
    inherits_status: bool
    on_hold: NotRequired[CardColumnOnHold]
    parent: RecordingParent
    position: NotRequired[int]
    status: str
    subscribers: NotRequired[list[Person]]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CardColumnOnHold(TypedDict):
    cards_count: int
    cards_url: str
    created_at: str
    id: int
    inherits_status: bool
    status: str
    title: str
    updated_at: str


class CardStep(TypedDict):
    app_url: str
    assignees: NotRequired[list[Person]]
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    completed: NotRequired[bool]
    completed_at: NotRequired[str]
    completer: NotRequired[Person]
    completion_url: NotRequired[str]
    created_at: str
    creator: Person
    due_on: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    position: NotRequired[int]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CardTable(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    lists: NotRequired[list[CardColumn]]
    status: str
    subscribers: NotRequired[list[Person]]
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class Chatbot(TypedDict):
    app_url: NotRequired[str]
    command_url: NotRequired[str]
    created_at: str
    id: int
    lines_url: NotRequired[str]
    service_name: str
    updated_at: str
    url: NotRequired[str]


class ClientApproval(TypedDict):
    app_url: str
    approval_status: NotRequired[str]
    approver: NotRequired[Person]
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    content: NotRequired[str]
    created_at: str
    creator: Person
    due_on: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    replies_count: NotRequired[int]
    replies_url: NotRequired[str]
    responses: NotRequired[list[ClientApprovalResponse]]
    status: str
    subject: NotRequired[str]
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ClientApprovalResponse(TypedDict):
    app_url: NotRequired[str]
    approved: NotRequired[bool]
    bookmark_url: NotRequired[str]
    bucket: NotRequired[RecordingBucket]
    content: NotRequired[str]
    created_at: NotRequired[str]
    creator: NotRequired[Person]
    id: NotRequired[int]
    inherits_status: NotRequired[bool]
    parent: NotRequired[RecordingParent]
    status: NotRequired[str]
    title: NotRequired[str]
    type: NotRequired[str]
    updated_at: NotRequired[str]
    visible_to_clients: NotRequired[bool]


class ClientCompany(TypedDict):
    id: int
    name: str


class ClientCorrespondence(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    content: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    replies_count: NotRequired[int]
    replies_url: NotRequired[str]
    status: str
    subject: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ClientReply(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    content: str
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ClientSide(TypedDict):
    app_url: NotRequired[str]
    url: NotRequired[str]


class CloneToolRequestContent(TypedDict):
    source_recording_id: int
    title: NotRequired[str]


class Comment(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    content: str
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class CreateAttachmentResponseContent(TypedDict):
    attachable_sgid: NotRequired[str]


class CreateCampfireLineRequestContent(TypedDict):
    content: str
    content_type: NotRequired[str]


class CreateCardColumnRequestContent(TypedDict):
    description: NotRequired[str]
    title: str


class CreateCardRequestContent(TypedDict):
    content: NotRequired[str]
    due_on: NotRequired[str]
    notify: NotRequired[bool]
    title: str


class CreateCardStepRequestContent(TypedDict):
    assignees: NotRequired[list[int]]
    due_on: NotRequired[str]
    title: str


class CreateChatbotRequestContent(TypedDict):
    command_url: NotRequired[str]
    service_name: str


class CreateCommentRequestContent(TypedDict):
    content: str


class CreateDocumentRequestContent(TypedDict):
    content: NotRequired[str]
    status: NotRequired[str]
    subscriptions: NotRequired[list[int]]
    title: str


class CreateEventBoostRequestContent(TypedDict):
    content: str


class CreateForwardReplyRequestContent(TypedDict):
    content: str


class CreateGaugeNeedleRequestContent(TypedDict):
    gauge_needle: GaugeNeedlePayload
    notify: NotRequired[str]
    subscriptions: NotRequired[list[int]]


class CreateLineupMarkerRequestContent(TypedDict):
    date: str
    name: str


class CreateMessageRequestContent(TypedDict):
    category_id: NotRequired[int]
    content: NotRequired[str]
    status: NotRequired[str]
    subject: str
    subscriptions: NotRequired[list[int]]


class CreateMessageTypeRequestContent(TypedDict):
    icon: str
    name: str


class CreatePersonRequest(TypedDict):
    company_name: NotRequired[str]
    email_address: str
    name: str
    title: NotRequired[str]


class CreateProjectFromTemplateRequestContent(TypedDict):
    description: NotRequired[str]
    name: str


class CreateProjectRequestContent(TypedDict):
    description: NotRequired[str]
    name: str


class CreateQuestionRequestContent(TypedDict):
    schedule: QuestionSchedule
    title: str


class CreateRecordingBoostRequestContent(TypedDict):
    content: str


class CreateScheduleEntryRequestContent(TypedDict):
    all_day: NotRequired[bool]
    description: NotRequired[str]
    ends_at: str
    notify: NotRequired[bool]
    participant_ids: NotRequired[list[int]]
    starts_at: str
    subscriptions: NotRequired[list[int]]
    summary: str


class CreateTemplateRequestContent(TypedDict):
    description: NotRequired[str]
    name: str


class CreateTimesheetEntryRequestContent(TypedDict):
    date: str
    description: NotRequired[str]
    hours: str
    person_id: NotRequired[int]


class CreateTodoRequestContent(TypedDict):
    assignee_ids: NotRequired[list[int]]
    completion_subscriber_ids: NotRequired[list[int]]
    content: str
    description: NotRequired[str]
    due_on: NotRequired[str]
    notify: NotRequired[bool]
    starts_on: NotRequired[str]


class CreateTodolistGroupRequestContent(TypedDict):
    name: str


class CreateTodolistRequestContent(TypedDict):
    description: NotRequired[str]
    name: str


class CreateUploadRequestContent(TypedDict):
    attachable_sgid: str
    base_name: NotRequired[str]
    description: NotRequired[str]
    subscriptions: NotRequired[list[int]]


class CreateVaultRequestContent(TypedDict):
    title: str


class CreateWebhookRequestContent(TypedDict):
    active: NotRequired[bool]
    payload_url: str
    types: list[str]


class DockItem(TypedDict):
    app_url: str
    enabled: bool
    id: int
    name: str
    position: NotRequired[int]
    title: str
    url: str


class Document(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    content: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    position: NotRequired[int]
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class EnableOutOfOfficeRequestContent(TypedDict):
    out_of_office: OutOfOfficePayload


class Event(TypedDict):
    action: str
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    created_at: str
    creator: Person
    details: NotRequired[EventDetails]
    id: int
    recording_id: int


class EventDetails(TypedDict):
    added_person_ids: NotRequired[list[int]]
    notified_recipient_ids: NotRequired[list[int]]
    removed_person_ids: NotRequired[list[int]]


class ForbiddenErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class Forward(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    content: NotRequired[str]
    created_at: str
    creator: Person
    from_: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    replies_count: NotRequired[int]
    replies_url: NotRequired[str]
    status: str
    subject: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ForwardReply(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    content: str
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class Gauge(TypedDict):
    app_url: NotRequired[str]
    bookmark_url: NotRequired[str]
    bucket: NotRequired[RecordingBucket]
    created_at: str
    creator: NotRequired[Person]
    description: NotRequired[str]
    enabled: NotRequired[bool]
    id: int
    inherits_status: NotRequired[bool]
    last_needle_color: NotRequired[str]
    last_needle_position: NotRequired[int]
    previous_needle_position: NotRequired[int]
    status: NotRequired[str]
    title: NotRequired[str]
    type: NotRequired[str]
    updated_at: str
    url: NotRequired[str]
    visible_to_clients: NotRequired[bool]


class GaugeNeedle(TypedDict):
    app_url: NotRequired[str]
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: NotRequired[RecordingBucket]
    color: NotRequired[str]
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    created_at: str
    creator: NotRequired[Person]
    description: NotRequired[str]
    id: int
    inherits_status: NotRequired[bool]
    parent: NotRequired[RecordingParent]
    position: NotRequired[int]
    status: NotRequired[str]
    subscription_url: NotRequired[str]
    title: NotRequired[str]
    type: NotRequired[str]
    updated_at: str
    url: NotRequired[str]
    visible_to_clients: NotRequired[bool]


class GaugeNeedlePayload(TypedDict):
    color: NotRequired[str]
    description: NotRequired[str]
    position: int


class GaugeNeedleUpdatePayload(TypedDict):
    description: NotRequired[str]


class GaugeTogglePayload(TypedDict):
    enabled: bool


class GetAssignedTodosResponseContent(TypedDict):
    grouped_by: NotRequired[str]
    person: NotRequired[Person]
    todos: NotRequired[list[Todo]]


class GetMyAssignmentsResponseContent(TypedDict):
    non_priorities: NotRequired[list[MyAssignment]]
    priorities: NotRequired[list[MyAssignment]]


class GetMyNotificationsResponseContent(TypedDict):
    memories: NotRequired[list[Notification]]
    reads: NotRequired[list[Notification]]
    unreads: NotRequired[list[Notification]]


class GetOverdueTodosResponseContent(TypedDict):
    over_a_month_late: NotRequired[list[Todo]]
    over_a_week_late: NotRequired[list[Todo]]
    over_three_months_late: NotRequired[list[Todo]]
    under_a_week_late: NotRequired[list[Todo]]


class GetPersonProgressResponseContent(TypedDict):
    events: NotRequired[list[TimelineEvent]]
    person: NotRequired[Person]


class GetUpcomingScheduleResponseContent(TypedDict):
    assignables: NotRequired[list[Assignable]]
    recurring_schedule_entry_occurrences: NotRequired[list[ScheduleEntry]]
    schedule_entries: NotRequired[list[ScheduleEntry]]


class HillChart(TypedDict):
    app_update_url: NotRequired[str]
    app_versions_url: NotRequired[str]
    dots: NotRequired[list[HillChartDot]]
    enabled: bool
    stale: bool
    updated_at: NotRequired[str]


class HillChartDot(TypedDict):
    app_url: NotRequired[str]
    color: str
    id: int
    label: str
    position: int
    url: NotRequired[str]


class Inbox(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    forwards_count: NotRequired[int]
    forwards_url: NotRequired[str]
    id: int
    inherits_status: bool
    position: NotRequired[int]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class InternalServerErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class LineupMarker(TypedDict):
    created_at: str
    date: str
    id: int
    name: str
    updated_at: str


class MarkAsReadRequestContent(TypedDict):
    readables: list[str]


class Message(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    category: NotRequired[MessageType]
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    content: str
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    subject: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class MessageBoard(TypedDict):
    app_messages_url: NotRequired[str]
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    messages_count: NotRequired[int]
    messages_url: NotRequired[str]
    position: NotRequired[int]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class MessageType(TypedDict):
    created_at: str
    icon: str
    id: int
    name: str
    updated_at: str


class MoveCardColumnRequestContent(TypedDict):
    position: NotRequired[int]
    source_id: int
    target_id: int


class MoveCardRequestContent(TypedDict):
    column_id: int
    position: NotRequired[int]


class MyAssignment(TypedDict):
    app_url: NotRequired[str]
    assignees: NotRequired[list[MyAssignmentAssignee]]
    bucket: NotRequired[MyAssignmentBucket]
    children: NotRequired[list[MyAssignment]]
    comments_count: NotRequired[int]
    completed: NotRequired[bool]
    content: NotRequired[str]
    due_on: NotRequired[str]
    has_description: NotRequired[bool]
    id: int
    parent: NotRequired[MyAssignmentParent]
    priority_recording_id: NotRequired[int]
    starts_on: NotRequired[str]
    type: NotRequired[str]


class MyAssignmentAssignee(TypedDict):
    avatar_url: NotRequired[str]
    id: int
    name: NotRequired[str]


class MyAssignmentBucket(TypedDict):
    app_url: NotRequired[str]
    id: int
    name: NotRequired[str]


class MyAssignmentParent(TypedDict):
    app_url: NotRequired[str]
    id: int
    title: NotRequired[str]


class NotFoundErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class Notification(TypedDict):
    app_url: NotRequired[str]
    bookmark_url: NotRequired[str]
    bucket_name: NotRequired[str]
    content_excerpt: NotRequired[str]
    created_at: str
    creator: NotRequired[Person]
    id: int
    image_url: NotRequired[str]
    memory_url: NotRequired[str]
    named: NotRequired[bool]
    participants: NotRequired[list[Person]]
    previewable_attachments: NotRequired[list[PreviewableAttachment]]
    read_at: NotRequired[str]
    readable_identifier: NotRequired[str]
    readable_sgid: NotRequired[str]
    section: NotRequired[str]
    subscribed: NotRequired[bool]
    subscription_url: NotRequired[str]
    title: NotRequired[str]
    type: NotRequired[str]
    unread_at: NotRequired[str]
    unread_count: NotRequired[int]
    unread_url: NotRequired[str]
    updated_at: str


class OutOfOffice(TypedDict):
    enabled: NotRequired[bool]
    end_date: NotRequired[str]
    ongoing: NotRequired[bool]
    person: NotRequired[OutOfOfficePerson]
    start_date: NotRequired[str]


class OutOfOfficePayload(TypedDict):
    end_date: str
    start_date: str


class OutOfOfficePerson(TypedDict):
    id: int
    name: NotRequired[str]


class PauseQuestionResponseContent(TypedDict):
    paused: NotRequired[bool]


class Person(TypedDict):
    admin: NotRequired[bool]
    attachable_sgid: NotRequired[str]
    avatar_url: NotRequired[str]
    bio: NotRequired[str]
    can_access_hill_charts: NotRequired[bool]
    can_access_timesheet: NotRequired[bool]
    can_manage_people: NotRequired[bool]
    can_manage_projects: NotRequired[bool]
    can_ping: NotRequired[bool]
    client: NotRequired[bool]
    company: NotRequired[PersonCompany]
    created_at: NotRequired[str]
    email_address: NotRequired[str]
    employee: NotRequired[bool]
    id: int
    location: NotRequired[str]
    name: str
    owner: NotRequired[bool]
    personable_type: NotRequired[str]
    time_zone: NotRequired[str]
    title: NotRequired[str]
    updated_at: NotRequired[str]


class PersonCompany(TypedDict):
    id: int
    name: str


class Preferences(TypedDict):
    app_url: NotRequired[str]
    first_week_day: NotRequired[str]
    time_format: NotRequired[str]
    time_zone_name: NotRequired[str]
    url: NotRequired[str]


class PreferencesPayload(TypedDict):
    first_week_day: NotRequired[str]
    time_format: NotRequired[str]
    time_zone_name: NotRequired[str]


class PreviewableAttachment(TypedDict):
    app_url: NotRequired[str]
    content_type: NotRequired[str]
    filename: NotRequired[str]
    filesize: NotRequired[int]
    height: NotRequired[int]
    id: NotRequired[int]
    url: NotRequired[str]
    width: NotRequired[int]


class Project(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bookmarked: NotRequired[bool]
    client_company: NotRequired[ClientCompany]
    clients_enabled: NotRequired[bool]
    clientside: NotRequired[ClientSide]
    created_at: str
    description: NotRequired[str]
    dock: NotRequired[list[DockItem]]
    id: int
    name: str
    purpose: NotRequired[str]
    status: str
    updated_at: str
    url: str


class ProjectAccessResult(TypedDict):
    granted: NotRequired[list[Person]]
    revoked: NotRequired[list[Person]]


class ProjectConstruction(TypedDict):
    id: int
    project: NotRequired[Project]
    status: str
    url: NotRequired[str]


class Question(TypedDict):
    answers_count: NotRequired[int]
    answers_url: NotRequired[str]
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    paused: NotRequired[bool]
    schedule: NotRequired[QuestionSchedule]
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class QuestionAnswer(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: RecordingBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    content: str
    created_at: str
    creator: Person
    group_on: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class QuestionAnswerPayload(TypedDict):
    content: str
    group_on: NotRequired[str]


class QuestionAnswerUpdatePayload(TypedDict):
    content: str
    group_on: NotRequired[str]


class QuestionReminder(TypedDict):
    group_on: NotRequired[str]
    question: NotRequired[Question]
    remind_at: NotRequired[str]
    reminder_id: NotRequired[int]


class QuestionSchedule(TypedDict):
    days: NotRequired[list[int]]
    end_date: NotRequired[str]
    frequency: NotRequired[str]
    hour: NotRequired[int]
    minute: NotRequired[int]
    month_interval: NotRequired[int]
    start_date: NotRequired[str]
    week_instance: NotRequired[int]
    week_interval: NotRequired[int]


class Questionnaire(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    name: str
    questions_count: NotRequired[int]
    questions_url: NotRequired[str]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class RateLimitErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]
    retry_after: NotRequired[int]


class Recording(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: RecordingBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    content: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    parent: RecordingParent
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class RecordingBucket(TypedDict):
    id: int
    name: str
    type: str


class RecordingParent(TypedDict):
    app_url: str
    id: int
    title: str
    type: str
    url: str


class RepositionCardStepRequestContent(TypedDict):
    position: int
    source_id: int


class RepositionTodoRequestContent(TypedDict):
    parent_id: NotRequired[int]
    position: int


class RepositionTodolistGroupRequestContent(TypedDict):
    position: int


class RepositionToolRequestContent(TypedDict):
    position: int


class ResumeQuestionResponseContent(TypedDict):
    paused: NotRequired[bool]


class Schedule(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    entries_count: NotRequired[int]
    entries_url: NotRequired[str]
    id: int
    include_due_assignments: NotRequired[bool]
    inherits_status: bool
    position: NotRequired[int]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ScheduleAttributes(TypedDict):
    end_date: NotRequired[str]
    start_date: NotRequired[str]


class ScheduleEntry(TypedDict):
    all_day: NotRequired[bool]
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    created_at: str
    creator: Person
    description: NotRequired[str]
    ends_at: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    participants: NotRequired[list[Person]]
    starts_at: NotRequired[str]
    status: str
    subscription_url: NotRequired[str]
    summary: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class SearchMetadata(TypedDict):
    projects: NotRequired[list[SearchProject]]


class SearchProject(TypedDict):
    id: NotRequired[int]
    name: NotRequired[str]


class SearchResult(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: NotRequired[RecordingBucket]
    content: NotRequired[str]
    created_at: NotRequired[str]
    creator: NotRequired[Person]
    description: NotRequired[str]
    id: int
    inherits_status: NotRequired[bool]
    parent: NotRequired[RecordingParent]
    status: NotRequired[str]
    subject: NotRequired[str]
    title: str
    type: str
    updated_at: NotRequired[str]
    url: str
    visible_to_clients: NotRequired[bool]


class SetCardColumnColorRequestContent(TypedDict):
    color: str


class SetCardStepCompletionRequestContent(TypedDict):
    completion: str


class SetClientVisibilityRequestContent(TypedDict):
    visible_to_clients: bool


class Subscription(TypedDict):
    count: int
    subscribed: bool
    subscribers: NotRequired[list[Person]]
    url: str


class Template(TypedDict):
    app_url: NotRequired[str]
    created_at: str
    description: NotRequired[str]
    dock: NotRequired[list[DockItem]]
    id: int
    name: str
    status: NotRequired[str]
    updated_at: str
    url: NotRequired[str]


class TimelineEvent(TypedDict):
    action: NotRequired[str]
    app_url: NotRequired[str]
    bucket: NotRequired[TodoBucket]
    created_at: NotRequired[str]
    creator: NotRequired[Person]
    id: NotRequired[int]
    kind: NotRequired[str]
    parent_recording_id: NotRequired[int]
    summary_excerpt: NotRequired[str]
    target: NotRequired[str]
    title: NotRequired[str]
    url: NotRequired[str]


class TimesheetEntry(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    date: NotRequired[str]
    description: NotRequired[str]
    hours: NotRequired[str]
    id: int
    inherits_status: bool
    parent: RecordingParent
    person: NotRequired[Person]
    status: str
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class Todo(TypedDict):
    app_url: str
    assignees: NotRequired[list[Person]]
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    completed: NotRequired[bool]
    completion_subscribers: NotRequired[list[Person]]
    completion_url: NotRequired[str]
    content: str
    created_at: str
    creator: Person
    description: NotRequired[str]
    due_on: NotRequired[str]
    id: int
    inherits_status: bool
    parent: TodoParent
    position: NotRequired[int]
    starts_on: NotRequired[str]
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class TodoBucket(TypedDict):
    id: int
    name: str
    type: str


class TodoParent(TypedDict):
    app_url: str
    id: int
    title: str
    type: str
    url: str


class Todolist(TypedDict):
    app_todos_url: NotRequired[str]
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    completed: NotRequired[bool]
    completed_ratio: NotRequired[str]
    created_at: str
    creator: Person
    description: NotRequired[str]
    groups_url: NotRequired[str]
    id: int
    inherits_status: bool
    name: str
    parent: TodoParent
    position: NotRequired[int]
    status: str
    subscription_url: NotRequired[str]
    title: str
    todos_url: NotRequired[str]
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class TodolistGroup(TypedDict):
    app_todos_url: NotRequired[str]
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    completed: NotRequired[bool]
    completed_ratio: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    name: str
    parent: TodoParent
    position: NotRequired[int]
    status: str
    subscription_url: NotRequired[str]
    title: str
    todos_url: NotRequired[str]
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class Todoset(TypedDict):
    app_todolists_url: NotRequired[str]
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    completed: NotRequired[bool]
    completed_ratio: NotRequired[str]
    created_at: str
    creator: Person
    id: int
    inherits_status: bool
    name: str
    position: NotRequired[int]
    status: str
    title: str
    todolists_count: NotRequired[int]
    todolists_url: NotRequired[str]
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool


class ToggleGaugeRequestContent(TypedDict):
    gauge: GaugeTogglePayload


class Tool(TypedDict):
    app_url: NotRequired[str]
    bucket: NotRequired[RecordingBucket]
    created_at: str
    enabled: bool
    id: int
    name: str
    position: NotRequired[int]
    status: NotRequired[str]
    title: str
    updated_at: str
    url: NotRequired[str]


class UnauthorizedErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class UpdateAccountNameRequestContent(TypedDict):
    name: str


class UpdateCardColumnRequestContent(TypedDict):
    description: NotRequired[str]
    title: NotRequired[str]


class UpdateCardRequestContent(TypedDict):
    assignee_ids: NotRequired[list[int]]
    content: NotRequired[str]
    due_on: NotRequired[str]
    title: NotRequired[str]


class UpdateCardStepRequestContent(TypedDict):
    assignees: NotRequired[list[int]]
    due_on: NotRequired[str]
    title: NotRequired[str]


class UpdateChatbotRequestContent(TypedDict):
    command_url: NotRequired[str]
    service_name: str


class UpdateCommentRequestContent(TypedDict):
    content: str


class UpdateDocumentRequestContent(TypedDict):
    content: NotRequired[str]
    title: NotRequired[str]


class UpdateGaugeNeedleRequestContent(TypedDict):
    gauge_needle: NotRequired[GaugeNeedleUpdatePayload]


class UpdateHillChartSettingsRequestContent(TypedDict):
    tracked: NotRequired[list[int]]
    untracked: NotRequired[list[int]]


class UpdateLineupMarkerRequestContent(TypedDict):
    date: NotRequired[str]
    name: NotRequired[str]


class UpdateMessageRequestContent(TypedDict):
    category_id: NotRequired[int]
    content: NotRequired[str]
    status: NotRequired[str]
    subject: NotRequired[str]


class UpdateMessageTypeRequestContent(TypedDict):
    icon: NotRequired[str]
    name: NotRequired[str]


class UpdateMyPreferencesRequestContent(TypedDict):
    person: PreferencesPayload


class UpdateMyProfileRequestContent(TypedDict):
    bio: NotRequired[str]
    email_address: NotRequired[str]
    first_week_day: NotRequired[str]
    location: NotRequired[str]
    name: NotRequired[str]
    time_format: NotRequired[str]
    time_zone_name: NotRequired[str]
    title: NotRequired[str]


class UpdateProjectAccessRequestContent(TypedDict):
    create: NotRequired[list[CreatePersonRequest]]
    grant: NotRequired[list[int]]
    revoke: NotRequired[list[int]]


class UpdateProjectRequestContent(TypedDict):
    admissions: NotRequired[str]
    description: NotRequired[str]
    name: str
    schedule_attributes: NotRequired[ScheduleAttributes]


class UpdateQuestionNotificationSettingsRequestContent(TypedDict):
    digest_include_unanswered: NotRequired[bool]
    notify_on_answer: NotRequired[bool]


class UpdateQuestionNotificationSettingsResponseContent(TypedDict):
    responding: NotRequired[bool]
    subscribed: NotRequired[bool]


class UpdateQuestionRequestContent(TypedDict):
    paused: NotRequired[bool]
    schedule: NotRequired[QuestionSchedule]
    title: NotRequired[str]


class UpdateScheduleEntryRequestContent(TypedDict):
    all_day: NotRequired[bool]
    description: NotRequired[str]
    ends_at: NotRequired[str]
    notify: NotRequired[bool]
    participant_ids: NotRequired[list[int]]
    starts_at: NotRequired[str]
    summary: NotRequired[str]


class UpdateScheduleSettingsRequestContent(TypedDict):
    include_due_assignments: bool


class UpdateSubscriptionRequestContent(TypedDict):
    subscriptions: NotRequired[list[int]]
    unsubscriptions: NotRequired[list[int]]


class UpdateTemplateRequestContent(TypedDict):
    description: NotRequired[str]
    name: NotRequired[str]


class UpdateTimesheetEntryRequestContent(TypedDict):
    date: NotRequired[str]
    description: NotRequired[str]
    hours: NotRequired[str]
    person_id: NotRequired[int]


class UpdateTodoRequestContent(TypedDict):
    assignee_ids: NotRequired[list[int]]
    completion_subscriber_ids: NotRequired[list[int]]
    content: NotRequired[str]
    description: NotRequired[str]
    due_on: NotRequired[str]
    notify: NotRequired[bool]
    starts_on: NotRequired[str]


class UpdateTodolistOrGroupRequestContent(TypedDict):
    description: NotRequired[str]
    name: NotRequired[str]


class UpdateToolRequestContent(TypedDict):
    title: str


class UpdateUploadRequestContent(TypedDict):
    base_name: NotRequired[str]
    description: NotRequired[str]


class UpdateVaultRequestContent(TypedDict):
    title: NotRequired[str]


class UpdateWebhookRequestContent(TypedDict):
    active: NotRequired[bool]
    payload_url: NotRequired[str]
    types: NotRequired[list[str]]


class Upload(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    boosts_count: NotRequired[int]
    boosts_url: NotRequired[str]
    bucket: TodoBucket
    byte_size: NotRequired[int]
    comments_count: NotRequired[int]
    comments_url: NotRequired[str]
    content_type: NotRequired[str]
    created_at: str
    creator: Person
    description: NotRequired[str]
    download_url: NotRequired[str]
    filename: NotRequired[str]
    height: NotRequired[int]
    id: int
    inherits_status: bool
    parent: RecordingParent
    position: NotRequired[int]
    status: str
    subscription_url: NotRequired[str]
    title: str
    type: str
    updated_at: str
    url: str
    visible_to_clients: bool
    width: NotRequired[int]


class ValidationErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]


class Vault(TypedDict):
    app_url: str
    bookmark_url: NotRequired[str]
    bucket: TodoBucket
    created_at: str
    creator: Person
    documents_count: NotRequired[int]
    documents_url: NotRequired[str]
    id: int
    inherits_status: bool
    parent: NotRequired[RecordingParent]
    position: NotRequired[int]
    status: str
    title: str
    type: str
    updated_at: str
    uploads_count: NotRequired[int]
    uploads_url: NotRequired[str]
    url: str
    vaults_count: NotRequired[int]
    vaults_url: NotRequired[str]
    visible_to_clients: bool


class Webhook(TypedDict):
    active: NotRequired[bool]
    app_url: str
    created_at: str
    id: int
    payload_url: str
    recent_deliveries: NotRequired[list[WebhookDelivery]]
    types: NotRequired[list[str]]
    updated_at: str
    url: str


class WebhookCopy(TypedDict):
    app_url: NotRequired[str]
    bucket: NotRequired[WebhookCopyBucket]
    id: NotRequired[int]
    url: NotRequired[str]


class WebhookCopyBucket(TypedDict):
    id: NotRequired[int]


class WebhookDelivery(TypedDict):
    created_at: NotRequired[str]
    id: NotRequired[int]
    request: NotRequired[WebhookDeliveryRequest]
    response: NotRequired[WebhookDeliveryResponse]


class WebhookDeliveryRequest(TypedDict):
    body: NotRequired[WebhookEvent]
    headers: NotRequired[WebhookHeadersMap]


class WebhookDeliveryResponse(TypedDict):
    code: NotRequired[int]
    headers: NotRequired[WebhookHeadersMap]
    message: NotRequired[str]


class WebhookEvent(TypedDict):
    copy: NotRequired[WebhookCopy]
    created_at: NotRequired[str]
    creator: NotRequired[Person]
    details: NotRequired[Any]
    id: NotRequired[int]
    kind: NotRequired[str]
    recording: NotRequired[Recording]


class WebhookLimitErrorResponseContent(TypedDict):
    error: str
    message: NotRequired[str]
