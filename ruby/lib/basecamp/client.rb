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
    # @param token_provider [TokenProvider, nil] OAuth token provider (deprecated, use auth_strategy)
    # @param auth_strategy [AuthStrategy, nil] authentication strategy
    # @param hooks [Hooks, nil] observability hooks
    def initialize(config:, token_provider: nil, auth_strategy: nil, hooks: nil)
      raise ArgumentError, "provide either token_provider or auth_strategy, not both" if token_provider && auth_strategy
      raise ArgumentError, "provide token_provider or auth_strategy" if !token_provider && !auth_strategy

      @config = config
      @hooks = hooks || NoopHooks.new
      @http = Http.new(config: config, token_provider: token_provider, auth_strategy: auth_strategy, hooks: @hooks)
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
    def get(path, params: {}, operation: nil)
      @parent.http.get(account_path(path), params: params, operation: operation)
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

    # Performs a PUT request with raw binary data scoped to this account.
    # Used for multipart uploads (e.g., account logo).
    # @param path [String] URL path (without account prefix)
    # @param body [String, IO] raw binary data
    # @param content_type [String] MIME content type
    # @return [Response]
    def put_raw(path, body:, content_type:)
      @parent.http.put_raw(account_path(path), body: body, content_type: content_type)
    end

    # Fetches all pages of a paginated resource.
    # @param path [String] URL path (without account prefix)
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @yield [Hash] each item from the response
    # @return [ListEnumerator] metadata-carrying lazy enumerator
    def paginate(path, params: {}, operation: nil, max_items: nil, &)
      @parent.http.paginate(account_path(path), params: params, operation: operation, max_items: max_items, &)
    end

    # Fetches all pages of a paginated resource, extracting items from a key.
    # Use this for endpoints that return objects like { "events": [...] }.
    # @param path [String] URL path (without account prefix)
    # @param key [String] the key containing the array of items
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @yield [Hash] each item from the response
    # @return [ListEnumerator] metadata-carrying lazy enumerator
    def paginate_key(path, key:, params: {}, operation: nil, max_items: nil, &)
      @parent.http.paginate_key(account_path(path), key: key, params: params, operation: operation, \
        max_items: max_items, &)
    end

    # Fetches a wrapped paginated resource, returning wrapper fields + lazy paginated items.
    # @param path [String] URL path (without account prefix)
    # @param key [String] the key containing the array of paginated items
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @return [Hash] wrapper fields merged with key => ListEnumerator of all items
    def paginate_wrapped(path, key:, params: {}, operation: nil, max_items: nil)
      @parent.http.paginate_wrapped(account_path(path), key: key, params: params, operation: operation, \
        max_items: max_items)
    end

    # Downloads file content from any API-routable download URL.
    #
    # Handles the full download flow: URL rewriting to the configured API host,
    # authenticated first hop (which typically 302s to a signed download URL),
    # and unauthenticated second hop to fetch the actual file content. Neither
    # hop follows a redirect on its own: hop 1's is the dispatch to hop 2, and
    # a redirect on hop 2 is an error (SPEC §14 "Hop-2 Redirect Policy").
    #
    # @param raw_url [String] absolute download URL (e.g., from bc-attachment elements)
    # @return [DownloadResult] the download result with body, content_type, content_length, filename
    # @raise [UsageError] if raw_url is empty or not absolute
    # @raise [NetworkError] if a network error occurs
    # @raise [ApiError] if the API or download returns an error
    def download_url(raw_url)
      # Validation
      raise UsageError.new("download URL is required") if raw_url.nil? || raw_url.to_s.empty?

      begin
        parsed = URI.parse(raw_url)
      rescue URI::InvalidURIError
        raise UsageError.new("download URL must be an absolute URL")
      end
      raise UsageError.new("download URL must be an absolute URL") unless parsed.is_a?(URI::HTTP)

      # Operation hooks
      op = OperationInfo.new(
        service: "Account", operation: "DownloadURL",
        resource_type: "download", is_mutation: false
      )
      start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      safe_hook { hooks.on_operation_start(op) }

      begin
        # URL rewriting: replace scheme+host with config.base_url origin, preserve path+query+fragment
        base = URI.parse(config.base_url)
        rewritten = parsed.dup
        rewritten.scheme = base.scheme
        rewritten.host = base.host
        rewritten.port = base.port
        rewritten_url = rewritten.to_s

        # Hop 1: Authenticated API request under the SPEC §14 hop-1 retry
        # policy (captures redirect; every attempt is authenticated)
        response = http.get_download(rewritten_url)

        result = case response.status
        when 301, 302, 303, 307, 308
          # Redirect — extract Location, proceed to hop 2
          location = response.headers["Location"] || response.headers["location"]
          raise ApiError.new("redirect #{response.status} with no Location header") if location.nil? || location.empty?

          # Resolve relative Location against the rewritten API URL
          resolved_url = Security.resolve_url(rewritten_url, location)

          # Hop 2: fetch from signed URL (no auth, no hooks)
          signed_response = fetch_signed_download(resolved_url)

          DownloadResult.new(
            body: signed_response.body,
            content_type: signed_response["Content-Type"] || "",
            content_length: parse_content_length(signed_response["Content-Length"]),
            filename: Basecamp.filename_from_url(raw_url)
          )

        when 200..299
          # Direct download — no second hop
          DownloadResult.new(
            body: response.body,
            content_type: response.headers["Content-Type"] || response.headers["content-type"] || "",
            content_length: parse_content_length(response.headers["Content-Length"] || response.headers["content-length"]),
            filename: Basecamp.filename_from_url(raw_url)
          )

        else
          # This shouldn't happen because Faraday's raise_error middleware
          # handles 4xx/5xx, but handle it defensively
          raise Basecamp.error_from_response(response.status, response.body)
        end
      rescue => e
        duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
        safe_hook { hooks.on_operation_end(op, OperationResult.new(duration_ms: duration, error: e)) }
        raise
      else
        duration = ((Process.clock_gettime(Process::CLOCK_MONOTONIC) - start) * 1000).round
        safe_hook { hooks.on_operation_end(op, OperationResult.new(duration_ms: duration, error: nil)) }
        result
      end
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

    # @return [Services::HillChartsService]
    def hill_charts
      service(:hill_charts) { Services::HillChartsService.new(self) }
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

    # @return [Services::CloudFilesService]
    def cloud_files
      service(:cloud_files) { Services::CloudFilesService.new(self) }
    end

    # @return [Services::GoogleDocumentsService]
    def google_documents
      service(:google_documents) { Services::GoogleDocumentsService.new(self) }
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

    # @return [Services::WormholesService]
    def wormholes
      service(:wormholes) { Services::WormholesService.new(self) }
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

    # @return [Services::AutomationService]
    def automation
      service(:automation) { Services::AutomationService.new(self) }
    end

    # @return [Services::MessageTypesService]
    def message_types
      service(:message_types) { Services::MessageTypesService.new(self) }
    end

    # @return [Services::ToolsService]
    def tools
      service(:tools) { Services::ToolsService.new(self) }
    end

    # @return [Services::BookmarksService]
    def bookmarks
      service(:bookmarks) { Services::BookmarksService.new(self) }
    end

    # @return [Services::FoldersService]
    def folders
      service(:folders) { Services::FoldersService.new(self) }
    end

    # @return [Services::DraftsService]
    def drafts
      service(:drafts) { Services::DraftsService.new(self) }
    end

    # @return [Services::CalendarsService]
    def calendars
      service(:calendars) { Services::CalendarsService.new(self) }
    end

    # @return [Services::MyNotesService]
    def my_notes
      service(:my_notes) { Services::MyNotesService.new(self) }
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

    # @return [Services::EverythingService]
    def everything
      service(:everything) { Services::EverythingService.new(self) }
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

    # @return [Services::AccountService]
    def account
      service(:account) { Services::AccountService.new(self) }
    end

    # @return [Services::GaugesService]
    def gauges
      service(:gauges) { Services::GaugesService.new(self) }
    end

    # @return [Services::MyAssignmentsService]
    def my_assignments
      service(:my_assignments) { Services::MyAssignmentsService.new(self) }
    end

    # @return [Services::MyNotificationsService]
    def my_notifications
      service(:my_notifications) { Services::MyNotificationsService.new(self) }
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

    def fetch_signed_download(url)
      # The Location target is server-supplied, and this unauthenticated hop
      # has no same_origin? gate to refuse it (a signed URL is legitimately
      # cross-origin), so illegible targets must be refused here — as
      # ApiError, matching the legible refusals Go and TypeScript surface
      # when their dial rejects the scheme. One predicate states the whole
      # invariant — a dialable absolute HTTP(S) URL — because piecemeal
      # guards kept leaking: "it parses" admitted "ftp://", and "scheme is
      # http(s)" admitted hostless "http:foo", which Net::HTTP::Get rejects
      # with a raw ArgumentError before the dial (nothing is ever sent).
      # URI.parse types the scheme (URI::HTTPS < URI::HTTP), and a nil or
      # empty host ("http:foo", "http:///x") is undialable.
      uri = begin
        URI.parse(url)
      rescue URI::Error
        nil
      end
      unless uri.is_a?(URI::HTTP) && uri.host && !uri.host.empty?
        # SPEC §9: the signed URL is a credential — render its origin alone,
        # projected from the parse, or the fixed token when no complete origin
        # exists. Truncating the whole URL kept a short query intact.
        origin = if uri&.scheme && uri&.host && !uri.host.empty?
          port = uri.port && uri.port != uri.default_port ? ":#{uri.port}" : ""
          "#{uri.scheme}://#{uri.host}#{port}"
        else
          "unparsable"
        end
        raise ApiError.new("redirect to undialable download URL: #{origin}")
      end

      http_client = Net::HTTP.new(uri.host, uri.port)
      http_client.use_ssl = (uri.scheme == "https")
      http_client.open_timeout = config.timeout
      http_client.read_timeout = config.timeout

      request = Net::HTTP::Get.new(uri)

      begin
        # Net::HTTP#request never follows a redirect, which is the policy:
        # the signed URL is the one destination the API host named, and a
        # redirect from it is refused below, not dialled (SPEC §14 "Hop-2
        # Redirect Policy"). Stated here so a move to a following client
        # (Faraday, Net::HTTP.get_response's callers) has to argue with it.
        response = http_client.request(request)
      rescue StandardError
        # SPEC §9: the transport error renders the signed URL, so neither its
        # message nor the exception itself survives. cause: nil at the raise
        # site — MRI sets the built-in cause at raise time past the class's
        # stored cause: keyword (same pattern as oauth/exchange.rb).
        raise NetworkError.new("Download failed"), cause: nil
      end

      # The exact set hop 1 dispatches on, not Net::HTTPRedirection — that
      # class also covers 304, which is a cache answer, not a redirect.
      if [ 301, 302, 303, 307, 308 ].include?(response.code.to_i)
        raise ApiError.new("redirect #{response.code} on the signed download hop is not followed", http_status: response.code.to_i)
      end

      unless response.is_a?(Net::HTTPSuccess)
        raise ApiError.new("download failed with status #{response.code}", http_status: response.code.to_i)
      end

      response
    end

    def safe_hook
      yield
    rescue => e
      warn "Basecamp hook error: #{e.class}: #{e.message}"
    end

    def parse_content_length(value)
      return -1 if value.nil? || value.to_s.empty?

      parsed = value.to_i
      parsed >= 0 ? parsed : -1
    end
  end
end
