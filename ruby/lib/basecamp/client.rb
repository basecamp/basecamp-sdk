# frozen_string_literal: true

module Basecamp
  # Main client for the Basecamp API.
  #
  # Client holds shared resources and is used to create AccountClient instances
  # for specific Basecamp accounts via the {#for_account} method.
  #
  # @example Basic usage
  #   config = Basecamp::Config.from_env
  #   token_provider = Basecamp::StaticTokenProvider.new(ENV["BASECAMP_ACCESS_TOKEN"])
  #   client = Basecamp::Client.new(config: config, token_provider: token_provider)
  #
  #   # Get authorization info (account-independent)
  #   auth = client.authorization.get
  #
  #   # Work with a specific account
  #   account = client.for_account("12345")
  #   projects = account.projects.list
  #
  # @example With custom hooks
  #   require "logger"
  #   logger = Logger.new($stdout)
  #   hooks = Basecamp::LoggerHooks.new(logger)
  #
  #   client = Basecamp::Client.new(
  #     config: config,
  #     token_provider: token_provider,
  #     hooks: hooks
  #   )
  class Client
    # @return [Config] client configuration
    attr_reader :config

    # Creates a new Basecamp API client.
    #
    # @param config [Config] configuration settings
    # @param token_provider [TokenProvider] OAuth token provider
    # @param hooks [Hooks, nil] observability hooks
    def initialize(config:, token_provider:, hooks: nil)
      @config = config
      @token_provider = token_provider
      @hooks = hooks || NoopHooks.new
      @http = Http.new(config: config, token_provider: token_provider, hooks: @hooks)
      @mutex = Mutex.new
    end

    # Returns an AccountClient bound to the specified Basecamp account.
    #
    # The Basecamp API requires an account ID in the URL path
    # (e.g., https://3.basecampapi.com/12345/projects.json).
    #
    # @param account_id [String, Integer] the Basecamp account ID
    # @return [AccountClient]
    # @raise [ArgumentError] if account_id is empty or non-numeric
    #
    # @example
    #   account = client.for_account("12345")
    #   projects = account.projects.list
    def for_account(account_id)
      account_id = account_id.to_s
      raise ArgumentError, "account_id cannot be empty" if account_id.empty?
      raise ArgumentError, "account_id must be numeric, got: #{account_id}" unless account_id.match?(/\A\d+\z/)

      AccountClient.new(parent: self, account_id: account_id)
    end

    # Returns the AuthorizationService for authorization operations.
    # This is the only service available directly on Client, as it doesn't require
    # an account context. All other services require an AccountClient via {#for_account}.
    #
    # @return [Services::AuthorizationService]
    def authorization
      @mutex.synchronize do
        @authorization ||= Services::AuthorizationService.new(self)
      end
    end

    # @api private
    # Returns the HTTP client for making requests.
    # @return [Http]
    attr_reader :http

    # @api private
    # Returns the observability hooks.
    # @return [Hooks]
    attr_reader :hooks

    # @api private
    # Returns nil since Client is not bound to an account.
    # @return [nil]
    def account_id
      nil
    end
  end

  # HTTP client bound to a specific Basecamp account.
  #
  # Create an AccountClient using {Client#for_account}.
  # All API operations that require an account context use this class.
  #
  # @example
  #   account = client.for_account("12345")
  #
  #   # List projects
  #   account.projects.list.each do |project|
  #     puts project["name"]
  #   end
  #
  #   # Create a todo
  #   account.todos.create(
  #     project_id: 123,
  #     todolist_id: 456,
  #     content: "New task"
  #   )
  class AccountClient
    # @return [String] the account ID this client is bound to
    attr_reader :account_id

    # @api private
    # @param parent [Client] the parent client
    # @param account_id [String] the account ID
    def initialize(parent:, account_id:)
      @parent = parent
      @account_id = account_id
      @services = {}
      @mutex = Mutex.new
    end

    # @return [Config] client configuration
    def config
      @parent.config
    end

    # @api private
    # @return [Http] the HTTP client
    def http
      @parent.http
    end

    # @api private
    # @return [Hooks] the observability hooks
    def hooks
      @parent.hooks
    end

    # Performs a GET request scoped to this account.
    # @param path [String] URL path (without account prefix)
    # @param params [Hash] query parameters
    # @return [Response]
    def get(path, params: {})
      @parent.http.get(account_path(path), params: params)
    end

    # Performs a POST request scoped to this account.
    # @param path [String] URL path (without account prefix)
    # @param body [Hash, nil] request body
    # @return [Response]
    def post(path, body: nil)
      @parent.http.post(account_path(path), body: body)
    end

    # Performs a PUT request scoped to this account.
    # @param path [String] URL path (without account prefix)
    # @param body [Hash, nil] request body
    # @return [Response]
    def put(path, body: nil)
      @parent.http.put(account_path(path), body: body)
    end

    # Performs a DELETE request scoped to this account.
    # @param path [String] URL path (without account prefix)
    # @return [Response]
    def delete(path)
      @parent.http.delete(account_path(path))
    end

    # Performs a POST request with raw binary data scoped to this account.
    # Used for file uploads (attachments).
    # @param path [String] URL path (without account prefix)
    # @param body [String, IO] raw binary data
    # @param content_type [String] MIME content type
    # @return [Response]
    def post_raw(path, body:, content_type:)
      @parent.http.post_raw(account_path(path), body: body, content_type: content_type)
    end

    # Fetches all pages of a paginated resource.
    # @param path [String] URL path (without account prefix)
    # @param params [Hash] query parameters
    # @yield [Hash] each item from the response
    # @return [Enumerator] if no block given
    def paginate(path, params: {}, &)
      @parent.http.paginate(account_path(path), params: params, &)
    end

    # Fetches all pages of a paginated resource, extracting items from a key.
    # Use this for endpoints that return objects like { "events": [...] }.
    # @param path [String] URL path (without account prefix)
    # @param key [String] the key containing the array of items
    # @param params [Hash] query parameters
    # @yield [Hash] each item from the response
    # @return [Enumerator] if no block given
    def paginate_key(path, key:, params: {}, &)
      @parent.http.paginate_key(account_path(path), key: key, params: params, &)
    end

    # @!group Services

    # @return [Services::ProjectsService]
    def projects
      service(:projects) { Services::ProjectsService.new(self) }
    end

    # @return [Services::TodosService]
    def todos
      service(:todos) { Services::TodosService.new(self) }
    end

    # @return [Services::TodosetsService]
    def todosets
      service(:todosets) { Services::TodosetsService.new(self) }
    end

    # @return [Services::TodolistsService]
    def todolists
      service(:todolists) { Services::TodolistsService.new(self) }
    end

    # @return [Services::PeopleService]
    def people
      service(:people) { Services::PeopleService.new(self) }
    end

    # @return [Services::CommentsService]
    def comments
      service(:comments) { Services::CommentsService.new(self) }
    end

    # @return [Services::MessagesService]
    def messages
      service(:messages) { Services::MessagesService.new(self) }
    end

    # @return [Services::MessageBoardsService]
    def message_boards
      service(:message_boards) { Services::MessageBoardsService.new(self) }
    end

    # @return [Services::WebhooksService]
    def webhooks
      service(:webhooks) { Services::WebhooksService.new(self) }
    end

    # @return [Services::CampfiresService]
    def campfires
      service(:campfires) { Services::CampfiresService.new(self) }
    end

    # @return [Services::SchedulesService]
    def schedules
      service(:schedules) { Services::SchedulesService.new(self) }
    end

    # @return [Services::VaultsService]
    def vaults
      service(:vaults) { Services::VaultsService.new(self) }
    end

    # @return [Services::RecordingsService]
    def recordings
      service(:recordings) { Services::RecordingsService.new(self) }
    end

    # @return [Services::DocumentsService]
    def documents
      service(:documents) { Services::DocumentsService.new(self) }
    end

    # @return [Services::UploadsService]
    def uploads
      service(:uploads) { Services::UploadsService.new(self) }
    end

    # @return [Services::AttachmentsService]
    def attachments
      service(:attachments) { Services::AttachmentsService.new(self) }
    end

    # @return [Services::CheckinsService]
    def checkins
      service(:checkins) { Services::CheckinsService.new(self) }
    end

    # @return [Services::ForwardsService]
    def forwards
      service(:forwards) { Services::ForwardsService.new(self) }
    end

    # @return [Services::CardTablesService]
    def card_tables
      service(:card_tables) { Services::CardTablesService.new(self) }
    end

    # @return [Services::CardsService]
    def cards
      service(:cards) { Services::CardsService.new(self) }
    end

    # @return [Services::CardColumnsService]
    def card_columns
      service(:card_columns) { Services::CardColumnsService.new(self) }
    end

    # @return [Services::CardStepsService]
    def card_steps
      service(:card_steps) { Services::CardStepsService.new(self) }
    end

    # @return [Services::TemplatesService]
    def templates
      service(:templates) { Services::TemplatesService.new(self) }
    end

    # @return [Services::EventsService]
    def events
      service(:events) { Services::EventsService.new(self) }
    end

    # @return [Services::ClientApprovalsService]
    def client_approvals
      service(:client_approvals) { Services::ClientApprovalsService.new(self) }
    end

    # @return [Services::ClientCorrespondencesService]
    def client_correspondences
      service(:client_correspondences) { Services::ClientCorrespondencesService.new(self) }
    end

    # @return [Services::ClientRepliesService]
    def client_replies
      service(:client_replies) { Services::ClientRepliesService.new(self) }
    end

    # @return [Services::LineupService]
    def lineup
      service(:lineup) { Services::LineupService.new(self) }
    end

    # @return [Services::MessageTypesService]
    def message_types
      service(:message_types) { Services::MessageTypesService.new(self) }
    end

    # @return [Services::ToolsService]
    def tools
      service(:tools) { Services::ToolsService.new(self) }
    end

    # @return [Services::SubscriptionsService]
    def subscriptions
      service(:subscriptions) { Services::SubscriptionsService.new(self) }
    end

    # @return [Services::SearchService]
    def search
      service(:search) { Services::SearchService.new(self) }
    end

    # @return [Services::ReportsService]
    def reports
      service(:reports) { Services::ReportsService.new(self) }
    end

    # @return [Services::TimelineService]
    def timeline
      service(:timeline) { Services::TimelineService.new(self) }
    end

    # @return [Services::TimesheetsService]
    def timesheets
      service(:timesheets) { Services::TimesheetsService.new(self) }
    end

    # @return [Services::ClientVisibilityService]
    def client_visibility
      service(:client_visibility) { Services::ClientVisibilityService.new(self) }
    end

    # @return [Services::TodolistGroupsService]
    def todolist_groups
      service(:todolist_groups) { Services::TodolistGroupsService.new(self) }
    end

    # @return [Services::BoostsService]
    def boosts
      service(:boosts) { Services::BoostsService.new(self) }
    end

    # @!endgroup

    private

    def account_path(path)
      return path if path.start_with?("http://", "https://")

      path = "/#{path}" unless path.start_with?("/")

      # Guard against double-prefixing
      prefix = "/#{@account_id}"
      if path.start_with?(prefix)
        rest = path[prefix.length..]
        return path if rest.empty? || rest.start_with?("/", "?")
      end

      "/#{@account_id}#{path}"
    end

    def service(name)
      @mutex.synchronize do
        @services[name] ||= yield
      end
    end
  end
end
