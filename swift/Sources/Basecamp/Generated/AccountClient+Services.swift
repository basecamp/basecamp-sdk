// @generated from OpenAPI spec — do not edit directly
import Foundation

extension AccountClient {
    public var attachments: AttachmentsService { service("attachments") { AttachmentsService(accountClient: self) } }
    public var boosts: BoostsService { service("boosts") { BoostsService(accountClient: self) } }
    public var campfires: CampfiresService { service("campfires") { CampfiresService(accountClient: self) } }
    public var cardColumns: CardColumnsService { service("cardColumns") { CardColumnsService(accountClient: self) } }
    public var cardSteps: CardStepsService { service("cardSteps") { CardStepsService(accountClient: self) } }
    public var cardTables: CardTablesService { service("cardTables") { CardTablesService(accountClient: self) } }
    public var cards: CardsService { service("cards") { CardsService(accountClient: self) } }
    public var checkins: CheckinsService { service("checkins") { CheckinsService(accountClient: self) } }
    public var clientApprovals: ClientApprovalsService { service("clientApprovals") { ClientApprovalsService(accountClient: self) } }
    public var clientCorrespondences: ClientCorrespondencesService { service("clientCorrespondences") { ClientCorrespondencesService(accountClient: self) } }
    public var clientReplies: ClientRepliesService { service("clientReplies") { ClientRepliesService(accountClient: self) } }
    public var clientVisibility: ClientVisibilityService { service("clientVisibility") { ClientVisibilityService(accountClient: self) } }
    public var comments: CommentsService { service("comments") { CommentsService(accountClient: self) } }
    public var documents: DocumentsService { service("documents") { DocumentsService(accountClient: self) } }
    public var events: EventsService { service("events") { EventsService(accountClient: self) } }
    public var forwards: ForwardsService { service("forwards") { ForwardsService(accountClient: self) } }
    public var lineup: LineupService { service("lineup") { LineupService(accountClient: self) } }
    public var messageBoards: MessageBoardsService { service("messageBoards") { MessageBoardsService(accountClient: self) } }
    public var messageTypes: MessageTypesService { service("messageTypes") { MessageTypesService(accountClient: self) } }
    public var messages: MessagesService { service("messages") { MessagesService(accountClient: self) } }
    public var people: PeopleService { service("people") { PeopleService(accountClient: self) } }
    public var projects: ProjectsService { service("projects") { ProjectsService(accountClient: self) } }
    public var recordings: RecordingsService { service("recordings") { RecordingsService(accountClient: self) } }
    public var reports: ReportsService { service("reports") { ReportsService(accountClient: self) } }
    public var schedules: SchedulesService { service("schedules") { SchedulesService(accountClient: self) } }
    public var search: SearchService { service("search") { SearchService(accountClient: self) } }
    public var subscriptions: SubscriptionsService { service("subscriptions") { SubscriptionsService(accountClient: self) } }
    public var templates: TemplatesService { service("templates") { TemplatesService(accountClient: self) } }
    public var timeline: TimelineService { service("timeline") { TimelineService(accountClient: self) } }
    public var timesheets: TimesheetsService { service("timesheets") { TimesheetsService(accountClient: self) } }
    public var todolistGroups: TodolistGroupsService { service("todolistGroups") { TodolistGroupsService(accountClient: self) } }
    public var todolists: TodolistsService { service("todolists") { TodolistsService(accountClient: self) } }
    public var todos: TodosService { service("todos") { TodosService(accountClient: self) } }
    public var todosets: TodosetsService { service("todosets") { TodosetsService(accountClient: self) } }
    public var tools: ToolsService { service("tools") { ToolsService(accountClient: self) } }
    public var uploads: UploadsService { service("uploads") { UploadsService(accountClient: self) } }
    public var vaults: VaultsService { service("vaults") { VaultsService(accountClient: self) } }
    public var webhooks: WebhooksService { service("webhooks") { WebhooksService(accountClient: self) } }
}
