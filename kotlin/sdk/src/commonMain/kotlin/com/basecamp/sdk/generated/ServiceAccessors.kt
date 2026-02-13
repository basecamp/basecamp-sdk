package com.basecamp.sdk.generated

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.generated.services.*

/**
 * Generated service accessor extensions for [AccountClient].
 *
 * These properties provide lazy, cached access to all Basecamp API services.
 *
 * @generated from OpenAPI spec — do not edit directly
 */

/** Attachments operations. */
val AccountClient.attachments: AttachmentsService
    get() = service("Attachments") { AttachmentsService(this) }

/** Boosts operations. */
val AccountClient.boosts: BoostsService
    get() = service("Boosts") { BoostsService(this) }

/** Campfires operations. */
val AccountClient.campfires: CampfiresService
    get() = service("Campfires") { CampfiresService(this) }

/** CardColumns operations. */
val AccountClient.cardColumns: CardColumnsService
    get() = service("CardColumns") { CardColumnsService(this) }

/** CardSteps operations. */
val AccountClient.cardSteps: CardStepsService
    get() = service("CardSteps") { CardStepsService(this) }

/** CardTables operations. */
val AccountClient.cardTables: CardTablesService
    get() = service("CardTables") { CardTablesService(this) }

/** Cards operations. */
val AccountClient.cards: CardsService
    get() = service("Cards") { CardsService(this) }

/** Checkins operations. */
val AccountClient.checkins: CheckinsService
    get() = service("Checkins") { CheckinsService(this) }

/** ClientApprovals operations. */
val AccountClient.clientApprovals: ClientApprovalsService
    get() = service("ClientApprovals") { ClientApprovalsService(this) }

/** ClientCorrespondences operations. */
val AccountClient.clientCorrespondences: ClientCorrespondencesService
    get() = service("ClientCorrespondences") { ClientCorrespondencesService(this) }

/** ClientReplies operations. */
val AccountClient.clientReplies: ClientRepliesService
    get() = service("ClientReplies") { ClientRepliesService(this) }

/** ClientVisibility operations. */
val AccountClient.clientVisibility: ClientVisibilityService
    get() = service("ClientVisibility") { ClientVisibilityService(this) }

/** Comments operations. */
val AccountClient.comments: CommentsService
    get() = service("Comments") { CommentsService(this) }

/** Documents operations. */
val AccountClient.documents: DocumentsService
    get() = service("Documents") { DocumentsService(this) }

/** Events operations. */
val AccountClient.events: EventsService
    get() = service("Events") { EventsService(this) }

/** Forwards operations. */
val AccountClient.forwards: ForwardsService
    get() = service("Forwards") { ForwardsService(this) }

/** Lineup operations. */
val AccountClient.lineup: LineupService
    get() = service("Lineup") { LineupService(this) }

/** MessageBoards operations. */
val AccountClient.messageBoards: MessageBoardsService
    get() = service("MessageBoards") { MessageBoardsService(this) }

/** MessageTypes operations. */
val AccountClient.messageTypes: MessageTypesService
    get() = service("MessageTypes") { MessageTypesService(this) }

/** Messages operations. */
val AccountClient.messages: MessagesService
    get() = service("Messages") { MessagesService(this) }

/** People operations. */
val AccountClient.people: PeopleService
    get() = service("People") { PeopleService(this) }

/** Projects operations. */
val AccountClient.projects: ProjectsService
    get() = service("Projects") { ProjectsService(this) }

/** Recordings operations. */
val AccountClient.recordings: RecordingsService
    get() = service("Recordings") { RecordingsService(this) }

/** Reports operations. */
val AccountClient.reports: ReportsService
    get() = service("Reports") { ReportsService(this) }

/** Schedules operations. */
val AccountClient.schedules: SchedulesService
    get() = service("Schedules") { SchedulesService(this) }

/** Search operations. */
val AccountClient.search: SearchService
    get() = service("Search") { SearchService(this) }

/** Subscriptions operations. */
val AccountClient.subscriptions: SubscriptionsService
    get() = service("Subscriptions") { SubscriptionsService(this) }

/** Templates operations. */
val AccountClient.templates: TemplatesService
    get() = service("Templates") { TemplatesService(this) }

/** Timeline operations. */
val AccountClient.timeline: TimelineService
    get() = service("Timeline") { TimelineService(this) }

/** Timesheets operations. */
val AccountClient.timesheets: TimesheetsService
    get() = service("Timesheets") { TimesheetsService(this) }

/** TodolistGroups operations. */
val AccountClient.todolistGroups: TodolistGroupsService
    get() = service("TodolistGroups") { TodolistGroupsService(this) }

/** Todolists operations. */
val AccountClient.todolists: TodolistsService
    get() = service("Todolists") { TodolistsService(this) }

/** Todos operations. */
val AccountClient.todos: TodosService
    get() = service("Todos") { TodosService(this) }

/** Todosets operations. */
val AccountClient.todosets: TodosetsService
    get() = service("Todosets") { TodosetsService(this) }

/** Tools operations. */
val AccountClient.tools: ToolsService
    get() = service("Tools") { ToolsService(this) }

/** Uploads operations. */
val AccountClient.uploads: UploadsService
    get() = service("Uploads") { UploadsService(this) }

/** Vaults operations. */
val AccountClient.vaults: VaultsService
    get() = service("Vaults") { VaultsService(this) }

/** Webhooks operations. */
val AccountClient.webhooks: WebhooksService
    get() = service("Webhooks") { WebhooksService(this) }

