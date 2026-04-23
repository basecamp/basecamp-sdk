#!/usr/bin/env ruby
# frozen_string_literal: true

# Conformance test runner for the Ruby SDK.
#
# Reads JSON test definitions from conformance/tests/ and executes
# them against the SDK using WebMock for HTTP stubbing.

require "bundler/setup"
require "basecamp"
require "webmock"
require "json"
require "set"
require "time"

WebMock.enable!
WebMock.disable_net_connect!

# Test execution tracking
class TestTracker
  attr_reader :requests

  def initialize
    @requests = []
    @mutex = Mutex.new
  end

  def record_request(time:, method:, uri:, headers: {})
    @mutex.synchronize do
      @requests << { time: time, method: method, uri: uri.to_s, headers: headers }
    end
  end

  def reset!
    @mutex.synchronize { @requests.clear }
  end

  def request_count
    @requests.size
  end

  def delays_between_requests
    return [] if @requests.size < 2

    @requests.each_cons(2).map do |a, b|
      ((b[:time] - a[:time]) * 1000).to_i # milliseconds
    end
  end
end

# Stub mapper that always raises a pre-caught error.
# Used when client construction fails (e.g., HTTPS enforcement).
class ErrorMapper
  def initialize(error)
    @error = error
  end

  def call(*, **)
    raise @error
  end
end

# Maps operation names to SDK method calls
class OperationMapper
  def initialize(account_client)
    @account = account_client
  end

  def call(operation, path_params: {}, query_params: {}, body: nil)
    case operation
    when "ListProjects"
      @account.projects.list.to_a
    when "GetProject"
      @account.projects.get(project_id: path_params["projectId"])
    when "CreateProject"
      @account.projects.create(name: body["name"])
    when "ListTodos"
      @account.todos.list(
        todolist_id: path_params["todolistId"]
      ).to_a
    when "GetTodo"
      @account.todos.get(
        todo_id: path_params["todoId"]
      )
    when "CreateTodo"
      @account.todos.create(
        todolist_id: path_params["todolistId"],
        content: body["content"]
      )
    when "GetTimesheetEntry"
      @account.timesheets.get(
        entry_id: path_params["entryId"]
      )
    when "GetProjectTimeline"
      @account.timeline.get_project_timeline(
        project_id: path_params["projectId"]
      ).to_a
    when "UpdateProject"
      @account.projects.update(
        project_id: path_params["projectId"],
        name: body["name"]
      )
    when "TrashProject"
      @account.projects.trash(
        project_id: path_params["projectId"]
      )
    when "GetProjectTimesheet"
      @account.timesheets.for_project(
        project_id: path_params["projectId"]
      ).to_a
    when "UpdateTimesheetEntry"
      @account.timesheets.update(
        entry_id: path_params["entryId"],
        date: body&.dig("date"),
        hours: body&.dig("hours"),
        description: body&.dig("description")
      )
    when "ListWebhooks"
      @account.webhooks.list(
        bucket_id: path_params["bucketId"]
      ).to_a
    when "CreateWebhook"
      @account.webhooks.create(
        bucket_id: path_params["bucketId"],
        payload_url: body["payload_url"],
        types: body["types"]
      )
    when "GetProgressReport"
      @account.reports.progress.to_a
    when "GetPersonProgress"
      @account.reports.person_progress(
        person_id: path_params["personId"]
      )
    when "GetTool"
      @account.tools.get(
        tool_id: path_params["toolId"]
      )
    when "CloneTool"
      @account.tools.clone(
        source_recording_id: body["source_recording_id"],
        title: body["title"]
      )
    when "EnableTool"
      @account.tools.enable(
        tool_id: path_params["toolId"]
      )
    else
      raise "Unknown operation: #{operation}"
    end
  end
end

# Test result
TestResult = Struct.new(:name, :passed, :message)

# Tests where the Ruby SDK's behavior intentionally differs.
#
# The Ruby SDK only retries GET requests (see Http#request). PUT and DELETE
# are sent once even though they're naturally idempotent. Tests asserting
# mutation-retry behavior are skipped.
#
# The Ruby SDK's paginate Enumerator does not expose X-Total-Count metadata,
# so responseMeta.totalCount assertions are skipped.
RUBY_SKIPS = Set.new([
  "PUT operation is naturally idempotent",
  "DELETE operation is naturally idempotent",
  "Total count header is accessible",
  "Missing X-Total-Count returns zero",
  "Pagination stops at maxPages safety cap",
  "maxItems caps results across pages",
  "DownloadURL auth'd first hop 302s to signed URL",
  "DownloadURL direct 2xx body",
  "DownloadURL retries on 503 at the auth'd first hop",
  "DownloadURL honors Retry-After on 429 at the auth'd first hop",
  "DownloadURL surfaces redirect with no Location",
].freeze)

DOWNLOAD_SKIP = "Ruby runner does not yet dispatch DownloadURL (tracked as follow-up)".freeze
RUBY_SKIP_REASONS = {
  "PUT operation is naturally idempotent" => "Ruby SDK only retries GET",
  "DELETE operation is naturally idempotent" => "Ruby SDK only retries GET",
  "Total count header is accessible" => "Ruby SDK paginate doesn't expose X-Total-Count metadata",
  "Missing X-Total-Count returns zero" => "Ruby SDK paginate doesn't expose X-Total-Count metadata",
  "Pagination stops at maxPages safety cap" => "Ruby SDK paginate doesn't expose truncation metadata",
  "maxItems caps results across pages" => "Ruby SDK paginate doesn't support maxItems",
  "DownloadURL auth'd first hop 302s to signed URL" => DOWNLOAD_SKIP,
  "DownloadURL direct 2xx body" => DOWNLOAD_SKIP,
  "DownloadURL retries on 503 at the auth'd first hop" => DOWNLOAD_SKIP,
  "DownloadURL honors Retry-After on 429 at the auth'd first hop" => DOWNLOAD_SKIP,
  "DownloadURL surfaces redirect with no Location" => DOWNLOAD_SKIP,
}.freeze

# Single test case
class TestRunner
  def initialize(test_case, tracker, mapper)
    @test = test_case
    @tracker = tracker
    @mapper = mapper
  end

  def run
    @tracker.reset!
    setup_mock_responses

    begin
      result = @mapper.call(
        @test["operation"],
        path_params: @test["pathParams"] || {},
        query_params: @test["queryParams"] || {},
        body: @test["requestBody"]
      )
      verify_assertions(result: result, error: nil)
    rescue StandardError => e
      verify_assertions(result: nil, error: e)
    end
  end

  private

  def setup_mock_responses
    responses = @test["mockResponses"] || []
    return if responses.empty?

    # Build the URL pattern from path
    path = @test["path"]
    (@test["pathParams"] || {}).each do |key, value|
      path = path.gsub("{#{key}}", value.to_s)
    end

    # Queue up responses
    response_queue = responses.map do |r|
      {
        status: r["status"],
        body: r["body"]&.to_json || "",
        headers: { "Content-Type" => "application/json" }.merge(r["headers"] || {})
      }
    end

    # Register the stub with a block to track requests and return queued responses
    method = @test["method"]&.downcase&.to_sym || :get
    url_pattern = %r{#{Regexp.escape(path)}}

    stub = WebMock.stub_request(method, url_pattern)

    paginates = auto_paginates?
    call_count = 0
    stub.to_return do |request|
      @tracker.record_request(time: Time.now, method: request.method, uri: request.uri, headers: request.headers)
      if call_count < response_queue.size
        resp = response_queue[call_count]
        call_count += 1
        resp
      elsif paginates
        # Beyond defined responses for paginated ops: empty 200 terminates pagination
        call_count += 1
        { status: 200, body: "[]", headers: { "Content-Type" => "application/json" } }
      else
        # Non-paginated overflow: 500 so retry exhaustion surfaces the error
        call_count += 1
        { status: 500, body: '{"error":"No more mock responses"}', headers: { "Content-Type" => "application/json" } }
      end
    end
  end

  def auto_paginates?
    (@test["mockResponses"] || []).any? do |r|
      r.dig("headers", "Link")&.include?('rel="next"')
    end
  end

  def verify_assertions(result:, error:)
    failures = []

    (@test["assertions"] || []).each do |assertion|
      case assertion["type"]
      when "requestCount"
        actual = @tracker.request_count
        expected = assertion["expected"]
        if auto_paginates?
          unless actual >= expected
            failures << "Expected >= #{expected} requests (SDK auto-paginates), got #{actual}"
          end
        else
          unless actual == expected
            failures << "Expected #{expected} requests, got #{actual}"
          end
        end

      when "delayBetweenRequests"
        delays = @tracker.delays_between_requests
        min_delay = assertion["min"]
        if min_delay && delays.any? { |d| d < min_delay }
          failures << "Expected minimum delay of #{min_delay}ms, got #{delays.min}ms"
        end

      when "noError"
        if error
          failures << "Expected no error, got: #{error.class}: #{error.message}"
        end

      when "statusCode"
        expected = assertion["expected"]
        actual_status = extract_http_status(error)
        if actual_status
          unless actual_status == expected
            failures << "Expected status #{expected}, got #{actual_status}"
          end
        elsif error
          failures << "Expected status #{expected}, got non-HTTP error: #{error.class}: #{error.message}"
        elsif expected >= 400
          failures << "Expected error with status #{expected}, but operation succeeded"
        end
        # No error + expected < 400 (2xx/3xx) → success, assertion passes

      when "responseBody"
        path = assertion["path"]
        expected = assertion["expected"]
        actual = dig_path(result, path)
        unless actual == expected
          failures << "Expected #{path} to be #{expected}, got #{actual}"
        end

      when "errorType"
        expected_type = assertion["expected"]
        unless error
          failures << "Expected error type #{expected_type.inspect}, but got no error"
          next
        end
        # Map conformance canonical error types to Ruby SDK error codes
        code_map = {
          "not_found" => Basecamp::ErrorCode::NOT_FOUND,
          "auth_required" => Basecamp::ErrorCode::AUTH,
          "forbidden" => Basecamp::ErrorCode::FORBIDDEN,
          "rate_limit" => Basecamp::ErrorCode::RATE_LIMIT,
          "validation" => Basecamp::ErrorCode::VALIDATION,
        }
        expected_code = code_map[expected_type]
        if expected_code.nil?
          failures << "Unknown conformance error type #{expected_type.inspect} (add to code_map)"
        elsif error.respond_to?(:code) && error.code != expected_code
          failures << "Expected error code #{expected_code.inspect}, got #{error.code.inspect}"
        end

      when "requestPath"
        expected = assertion["expected"]
        requests = @tracker.requests
        if requests.empty?
          failures << "Expected a request to be made, but no requests were recorded"
        else
          actual_path = URI.parse(requests.first[:uri]).path
          unless actual_path == expected
            failures << "Expected request path #{expected.inspect}, got #{actual_path.inspect}"
          end
        end

      when "errorCode"
        expected = assertion["expected"]
        unless error
          failures << "Expected error code #{expected.inspect}, but got no error"
          next
        end
        if error.respond_to?(:code)
          unless error.code == expected
            failures << "Expected error code #{expected.inspect}, got #{error.code.inspect}"
          end
        else
          failures << "Expected error with code #{expected.inspect}, but error #{error.class} has no code"
        end

      when "errorMessage"
        expected = assertion["expected"]
        unless error
          failures << "Expected error message containing #{expected.inspect}, but got no error"
          next
        end
        unless error.message.include?(expected)
          failures << "Expected error message containing #{expected.inspect}, got #{error.message.inspect}"
        end

      when "errorField"
        field_path = assertion["path"]
        expected = assertion["expected"]
        unless error
          failures << "Expected error field #{field_path}, but got no error"
          next
        end
        actual = case field_path
                 when "httpStatus"
                   error.respond_to?(:http_status) ? error.http_status : nil
                 when "retryable"
                   error.respond_to?(:retryable) ? error.retryable : nil
                 when "requestId"
                   error.respond_to?(:request_id) ? error.request_id : nil
                 when "code"
                   error.respond_to?(:code) ? error.code : nil
                 when "message"
                   error.message
                 else
                   failures << "Unknown error field: #{field_path}"
                   next
                 end
        unless actual == expected
          failures << "Expected error.#{field_path} = #{expected.inspect}, got #{actual.inspect}"
        end

      when "headerInjected"
        header_name = assertion["path"]
        expected = assertion["expected"]
        requests = @tracker.requests
        if requests.empty?
          failures << "Expected header #{header_name}=#{expected.inspect}, but no requests recorded"
        else
          actual = requests.first[:headers]&.[](header_name)
          unless actual == expected
            failures << "Expected header #{header_name}=#{expected.inspect}, got #{actual.inspect}"
          end
        end

      when "headerPresent"
        header_name = assertion["path"]
        requests = @tracker.requests
        if requests.empty?
          failures << "Expected header #{header_name} to be present, but no requests recorded"
        else
          actual = requests.first[:headers]&.[](header_name)
          if actual.nil? || actual.empty?
            failures << "Expected header #{header_name} to be present, but it was missing or empty"
          end
        end

      when "headerValue"
        header_name = assertion["path"]
        expected = assertion["expected"]
        responses = @test["mockResponses"] || []
        if responses.empty?
          failures << "Expected response header #{header_name}=#{expected.inspect}, but no mock responses defined"
        else
          actual = responses.first.dig("headers", header_name)
          unless actual == expected
            failures << "Expected response header #{header_name}=#{expected.inspect}, got #{actual.inspect}"
          end
        end

      when "requestScheme"
        # HTTPS enforcement: SDK should refuse HTTP for non-localhost.
        # The errorCode assertion handles the specific error check.
        expected = assertion["expected"]
        if expected == "https" && !error
          failures << "Expected HTTPS enforcement error, but request succeeded over HTTP"
        end

      when "urlOrigin"
        # Cross-origin rejection: verified by requestCount=1 (link not followed).
        expected = assertion["expected"]
        if expected == "rejected" && @tracker.request_count > 1
          failures << "Expected cross-origin URL rejection (1 request), but #{@tracker.request_count} requests were made"
        end

      when "responseMeta"
        field_path = assertion["path"]
        expected = assertion["expected"]
        actual = result.is_a?(Hash) ? result[field_path] || result[field_path.to_sym] : nil
        unless actual == expected
          failures << "Expected responseMeta.#{field_path} = #{expected.inspect}, got #{actual.inspect}"
        end

      else
        failures << "Unknown assertion type: #{assertion["type"]}"
      end
    end

    if failures.empty?
      TestResult.new(@test["name"], true, nil)
    else
      TestResult.new(@test["name"], false, failures.join("; "))
    end
  end

  # Extract HTTP status from an error, handling both APIError (has http_status)
  # and NetworkError wrapping Faraday::ServerError (5xx on mutations).
  def extract_http_status(error)
    return nil unless error

    return error.http_status if error.respond_to?(:http_status) && error.http_status

    # Ruby SDK wraps Faraday::ServerError (5xx) as NetworkError on mutations.
    # Dig into the cause chain to find the HTTP status.
    cause = error.respond_to?(:cause) ? error.cause : nil
    cause.response_status if cause.respond_to?(:response_status)
  end

  def dig_path(obj, path)
    return obj if path.nil? || path.empty?

    path.split(".").reduce(obj) do |current, key|
      return nil if current.nil?

      if current.is_a?(Hash)
        current[key] || current[key.to_sym]
      elsif current.respond_to?(key)
        current.send(key)
      else
        nil
      end
    end
  end
end

# Main runner
class ConformanceRunner
  def initialize(tests_dir)
    @tests_dir = tests_dir
    @tracker = TestTracker.new

    # Create a test client (use "conformance-test-token" to match headerInjected assertions)
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com")
    token_provider = Basecamp::StaticTokenProvider.new("conformance-test-token")
    client = Basecamp::Client.new(config: config, token_provider: token_provider)
    @account = client.for_account("999")
    @mapper = OperationMapper.new(@account)
  end

  # Returns an OperationMapper for the test, handling configOverrides.
  # If configOverrides.baseUrl is set, constructs a new client with that URL.
  # Construction-time errors (e.g., HTTPS enforcement) are wrapped in an
  # ErrorMapper that always raises the caught error.
  def mapper_for_test(test_case)
    overrides = test_case["configOverrides"]
    return @mapper unless overrides

    has_base_url = overrides.key?("baseUrl")
    has_max_pages = overrides.key?("maxPages")
    return @mapper unless has_base_url || has_max_pages

    begin
      config_opts = { base_url: has_base_url ? overrides["baseUrl"] : "https://3.basecampapi.com" }
      config_opts[:max_pages] = overrides["maxPages"] if has_max_pages
      config = Basecamp::Config.new(**config_opts)
      token_provider = Basecamp::StaticTokenProvider.new("conformance-test-token")
      client = Basecamp::Client.new(config: config, token_provider: token_provider)
      account = client.for_account("999")
      OperationMapper.new(account)
    rescue StandardError => e
      ErrorMapper.new(e)
    end
  end

  def run
    files = Dir.glob(File.join(@tests_dir, "*.json"))

    if files.empty?
      puts "No test files found in #{@tests_dir}"
      return 0
    end

    passed = 0
    failed = 0
    skipped = 0
    results = []

    files.each do |file|
      puts "\n=== #{File.basename(file)} ==="

      tests = JSON.parse(File.read(file))
      tests.each do |test_case|
        if RUBY_SKIPS.include?(test_case["name"])
          skipped += 1
          reason = RUBY_SKIP_REASONS[test_case["name"]] || "Ruby SDK behavior differs"
          puts "  SKIP: #{test_case["name"]} (#{reason})"
          WebMock.reset!
          next
        end

        mapper = mapper_for_test(test_case)
        runner = TestRunner.new(test_case, @tracker, mapper)
        result = runner.run
        results << result

        WebMock.reset!

        if result.passed
          passed += 1
          puts "  PASS: #{result.name}"
        else
          failed += 1
          puts "  FAIL: #{result.name}"
          puts "        #{result.message}"
        end
      end
    end

    puts "\n" + "=" * 40
    puts "Results: #{passed} passed, #{failed} failed, #{skipped} skipped"

    failed > 0 ? 1 : 0
  end
end

# Run if executed directly
if __FILE__ == $PROGRAM_NAME
  tests_dir = File.expand_path("../../tests", __dir__)
  runner = ConformanceRunner.new(tests_dir)
  exit runner.run
end
