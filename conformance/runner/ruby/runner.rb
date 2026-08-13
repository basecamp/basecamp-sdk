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
require "fileutils"

WebMock.enable!
WebMock.disable_net_connect!

# The case census (#602): every non-live fixture case must be accounted for by
# the run.
#
#   passed + failed + skipped  ==  cases in conformance/tests/**/*.json
#                                  whose mode != "live"
#
# The left side is what the runner actually did. The right side is counted by
# +non_live_case_count+ below — a SEPARATE walk and parse, deliberately not the
# runner's own load path. That independence is the entire point: a check fed by
# the load path can only confirm the load path agrees with itself.
#
# Why `mode != "live"` rather than `mode == "mock"`: all six runners select with
# "mock unless told otherwise" (+mock_mode?+ here, and its five equivalents), so
# a typo'd `mode: "moc"` is dropped by every runner at once with nothing printed
# anywhere. Counting the expected side as "not explicitly live" turns that
# silent divergence into arithmetic.
#
# Catches: an unrecognized `mode`; a fixture that failed to parse or was never
# globbed (including one nested below conformance/tests/, which no runner
# discovers — hence the recursive walk); a case dropped between load and
# dispatch; a fixture emptied to `[]` (which the census REFUSES rather than
# counts — see +non_live_case_count+, and note that counting it would make this
# bullet a lie); and any future skip channel that bypasses the counters, because
# the counters are what it reads.
#
# The typo is not this check's alone to catch, and saying so is what keeps the
# rest of the list honest: `make conformance-fixtures-check` validates
# conformance/tests/*.json against conformance/schema.json, whose `mode` is
# `enum: ["mock", "live"]`, so a typo in a TOP-LEVEL fixture fails there first
# and this census is defense in depth for that one case. What that gate
# structurally cannot see is everything else above — its glob is not recursive,
# so a fixture nested below conformance/tests/ is validated by nothing AND run
# by nothing (verified: such a file passes the schema gate and fails this
# census); a fixture truncated to `[]` is a valid array of zero cases; and a
# case dropped between load and dispatch is not a fixture-format question at
# all. Nor does that gate run when `make conformance-<lang>` is invoked alone.
#
# Does NOT catch the all-six case #602 names — one case every runner excludes
# for its own reason, which leaves each runner's own census green. That needs
# the six exclusion sets in one place, hence artifact plumbing across six CI
# jobs; #602 stays open for it.
#
# Kept apart from the runner so it is unit-testable (case_census_test.rb).
module CaseCensus
  # Raised for every fail-closed condition below, so a caller cannot catch the
  # parse failure and miss the empty-tree one.
  class Error < StandardError; end

  # Whether a fixture case's mode selects this runner.
  #
  # Absent means mock: live cases are TS-only (the canonical wire-capturer), and
  # every other value is nobody's. Shared with the census self-tests so the rule
  # the run loop applies is the rule under test, not a copy of it.
  #
  # Defaults on nil ONLY. `mode || "mock"` also defaults an explicit JSON
  # `false`, which Ruby alone would then run as mock while the census counts it
  # as non-live — matching totals over a value no other runner accepts. Python
  # had the same defect more broadly (its `or` defaulted every falsy value); the
  # typed runners reject a non-string `mode` when they decode it.
  def self.mock_mode?(mode)
    (mode.nil? ? "mock" : mode) == "mock"
  end

  # Counts fixture cases whose mode is not "live", recursively.
  #
  # Fail-closed in three places, each a way the count could certify nothing
  # while looking green: an unreadable tree, a fixture that does not parse, and
  # a walk that found no fixture files at all.
  # Every *.json under +dir+, recursively, sorted by path.
  #
  # A hand-rolled walk over Dir.children, NOT Dir.glob and NOT Find.find. Both
  # of those swallow the error from a directory they cannot read (find.rb
  # rescues Errno::EACCES and moves on), so an unreadable subdirectory is simply
  # omitted. The runner's non-recursive glob omits it too, so its cases leave
  # both sides of the census at once and the totals still agree — a fail-closed
  # walk failing open, which is the one failure this must not have.
  # Dir.children raises, which is the whole reason it is the API used here.
  def self.fixture_files(dir)
    entries = begin
      Dir.children(dir)
    rescue SystemCallError => e
      raise Error, "could not walk #{dir}: #{e.message}"
    end

    entries.sort.flat_map do |name|
      path = File.join(dir, name)
      if File.directory?(path)
        fixture_files(path)
      elsif File.extname(path) == ".json"
        [ path ]
      else
        []
      end
    end
  end

  def self.non_live_case_count(tests_dir)
    files = fixture_files(tests_dir).sort
    raise Error, "no *.json fixture files found under #{tests_dir}" if files.empty?

    files.sum do |file|
      begin
        parsed = JSON.parse(File.read(file))
      rescue JSON::ParserError, SystemCallError => e
        raise Error, "#{file}: #{e.message}"
      end
      raise Error, "#{file}: fixture is not a JSON array" unless parsed.is_a?(Array)

      # An emptied fixture is REFUSED, not counted as zero, and this is the one
      # rejection that carries the whole-file guarantee. It is the single
      # truncation both sides of the census read identically: the runner
      # registers nothing from the file and the census expects nothing, so the
      # two totals fall together and no mismatch ever appears. Counting it would
      # make "a fixture truncated to []" a claim this check cannot keep. A file
      # declaring no cases tests nothing, so refusing it costs nothing — and it
      # closes the same hole in conformance-fixtures-check, where an empty array
      # is a schema-valid list of zero items.
      if parsed.empty?
        raise Error, "#{file}: fixture declares no cases; delete the file or restore its cases"
      end

      # Only `mode` is read: the census must survive a fixture whose other
      # fields this runner cannot model, or it would report a failure for a case
      # the run itself handled fine.
      parsed.count { |test_case| !test_case.is_a?(Hash) || test_case["mode"] != "live" }
    end
  end

  # Compares what the run accounted for against the census, returning nil when
  # they agree and a message naming the short side otherwise.
  def self.count_failure(ran, expected)
    if ran == expected
      nil
    elsif ran < expected
      "case census: the run accounted for #{ran} case(s) (passed+failed+skipped) " \
        "but conformance/tests holds #{expected} non-live case(s) — " \
        "#{expected - ran} executed by nothing. An unrecognized `mode`, a fixture " \
        "that failed to parse or was never globbed, or a case dropped between load " \
        "and dispatch will do this."
    else
      "case census: the run accounted for #{ran} case(s) (passed+failed+skipped) " \
        "but conformance/tests holds only #{expected} non-live case(s) — " \
        "#{ran - expected} more than the fixtures declare."
    end
  end
end

# One runner's exclusion set for the cross-runner gate (#602).
#
# The case census answers "did THIS runner account for every case". A case every
# runner excludes leaves all six censuses green, because each counted its own
# skip — only scripts/check-fixture-execution.rb, comparing these manifests, can
# see it.
#
# +executed+ is recorded alongside the exclusions and asserted against the
# census total: without it a case a runner silently dropped is simply absent
# from the exclusion set, and "absent" reads identically to "ran fine".
module ExecutionManifest
  class Error < StandardError; end

  # Sorted, so a re-run is byte-identical.
  def self.write(runner, total, executed, excluded)
    if executed + excluded.length != total
      raise Error, "manifest for #{runner} is internally inconsistent: #{executed} executed + " \
                   "#{excluded.length} excluded != #{total} non-live cases; the run dropped a " \
                   "case without recording it as either"
    end

    dir = File.expand_path("../../../conformance/manifests", __dir__)
    FileUtils.mkdir_p(dir)
    body = {
      "runner" => runner,
      "total_non_live" => total,
      "executed" => executed,
      "excluded" => excluded.sort.map { |file, name, reason| { "file" => file, "name" => name, "reason" => reason } }
    }
    File.write(File.join(dir, "#{runner}.json"), "#{JSON.pretty_generate(body)}\n")
  end
end

# The errorRaised assertion contract, kept apart from the runner so its failing
# branch is unit-testable (error_raised_test.rb).
module ErrorRaised
  # Validates one assertion, returning nil when it holds and a failure message
  # otherwise.
  #
  # The inverse of noError, and deliberately code-agnostic. The
  # malformed-response family (#576) is refused by a hand-written guard in
  # TypeScript, Python and Ruby and by the model decoder in Go, Kotlin and
  # Swift; those two mechanisms share no canonical error code, so pinning
  # errorType would make the fixture unwritable. What all six agree on is that
  # the call fails at all — which, paired with requestCount, is the whole
  # contract: the composite refused the field instead of writing it.
  #
  # NO COMMITTED FIXTURE CAN REACH THE FAILING BRANCH: every case declaring
  # errorRaised is one the SDK does refuse, so a handler that accepted
  # everything would report green in all six runners at once. That is the #563
  # shape — a delayBetweenRequests check that passed vacuously because no
  # fixture supplied a gap it could fail on — and the reason
  # `make conformance-runner-tests` exists.
  #
  # The message is pinned verbatim by the unit tests in all six runners: a
  # fixture debugged in one language should not read differently in another.
  def self.check(dispatch_failed)
    dispatch_failed ? nil : "Expected the call to fail, but it succeeded"
  end
end

# The delayBetweenRequests assertion contract, kept apart from the runner so
# its bounds branches are unit-testable (delay_gaps_test.rb).
module DelayGaps
  # Validates one assertion against the recorded inter-request gaps, returning
  # nil when it holds and a failure message otherwise.
  #
  # +delays+ holds the inter-request gaps, so N requests yield N-1 entries and
  # gap i is the interval between request i and request i+1. The contract in
  # conformance/schema.json:
  #
  # * A NAMED index selects exactly that gap, bounds-checked unconditionally.
  #   A gap the run never produced is a failure, not a silent pass — the whole
  #   point of a timing pin is to catch a dropped backoff, and a dropped
  #   backoff is precisely what removes the gap.
  # * An OMITTED index requires the minimum on EVERY gap. Zero gaps means
  #   nothing was measured, so that fails too: an "every gap" rule with no gaps
  #   left would otherwise wave through a run that dropped every retry.
  # * Negative indexes are rejected rather than wrapping to the end like the
  #   per-request assertions. There is no sensible "last gap" when the point of
  #   naming one is to pin a specific backoff.
  #
  # An absent or zero +min_delay+ still asserts that the gap EXISTS. The default
  # is applied HERE rather than at the call site so gating on the value's
  # presence cannot quietly reduce the assertion to nothing — the false-green
  # class this exists to kill.
  #
  # +request_count+ is passed rather than inferred as +delays.length + 1+: that
  # inference assumes at least one request, so a run that failed before
  # dispatching anything would report "only 1 request(s) were made".
  def self.check(delays, min_delay, index, request_count)
    min_delay ||= 0

    if index
      named_gap_failure(delays, min_delay, index, request_count)
    elsif delays.empty?
      "Expected a delay between requests, but only #{request_count} request(s) were made"
    else
      short = delays.each_with_index.find { |delay, _| delay < min_delay }
      short && "Expected minimum delay of #{min_delay}ms at gap #{short.last}, got #{short.first}ms"
    end
  end

  def self.named_gap_failure(delays, min_delay, index, request_count)
    if index.negative?
      "delayBetweenRequests gap index must be non-negative, got #{index}"
    elsif index >= delays.length
      "Expected a delay at gap #{index}, but only #{request_count} request(s) were made"
    elsif delays[index] < min_delay
      "Expected minimum delay of #{min_delay}ms at gap #{index}, got #{delays[index]}ms"
    end
  end
end

# The requestCount assertion contract, kept apart from the runner so its bounds
# branches are unit-testable (request_count_test.rb).
module RequestCount
  # Validates one assertion, returning nil when it holds and a failure message
  # otherwise.
  #
  # EXACT, always — including the auto-paginating fixtures. The runner used to
  # relax this to a lower bound whenever any mock response carried
  # `Link: rel="next"`, on the theory that an auto-paginating SDK would
  # legitimately make more requests than the fixture named. That is backwards
  # for the fixtures the relaxation covered: in conformance/tests/pagination.json,
  # "Pagination stops at maxPages safety cap" and "maxItems caps results across
  # pages" each queue THREE pages and expect TWO requests, because stopping
  # early is the behavior under test. `>=` passes an SDK that ignored the cap
  # and walked every page. "Auto-pagination follows Link headers across
  # multiple pages" is the exposed case: its only assertions are requestCount
  # and noError, so an over-fetch has nothing else to catch it.
  #
  # The one fixture where the count genuinely does not apply to an
  # auto-paginating SDK — "List operation returns first page with Link header",
  # which asserts a single request — carries the `link-header` tag, and
  # `applies?` returns false for it. Nothing that still reaches this method
  # needs the relaxation.
  #
  # Swift took this in #558; #573 is the same fix for the other five runners.
  def self.check(actual, expected)
    "Expected #{expected} requests, got #{actual}" unless actual == expected
  end

  # Marks a fixture whose requestCount counts first-page requests only, which
  # an auto-paginating SDK cannot satisfy.
  LINK_HEADER_TAG = "link-header"

  # Whether a fixture's `requestCount` assertion is meaningful for this SDK.
  #
  # SCOPE: this suppresses ONE ASSERTION, not the whole test case. An earlier
  # revision skipped the entire `link-header` case in every runner, which took
  # its `statusCode: 200` and `noError` assertions down with the inapplicable
  # `requestCount` — Kotlin and Swift had always skipped the case wholesale, so
  # once Go, Python, Ruby and TypeScript joined them the fixture was executed by
  # nothing at all while still sitting in conformance/tests/pagination.json,
  # passing conformance-fixtures-check and check-fixture-coverage. That is the
  # #572 shape ("present, run by nothing") one layer down. Only the count is
  # inapplicable; the status code and the absence of an error are not, and they
  # are the assertions that catch an auto-paginating SDK that walked the Link
  # header into an error.
  def self.applies?(tags)
    !(tags || []).include?(LINK_HEADER_TAG)
  end
end

# Test execution tracking
class TestTracker
  attr_reader :requests

  def initialize
    @requests = []
    @mutex = Mutex.new
  end

  def record_request(monotonic_time:, method:, uri:, headers: {}, body: nil)
    @mutex.synchronize do
      @requests << { monotonic_time: monotonic_time, method: method, uri: uri.to_s, headers: headers, body: body }
    end
  end

  def reset!
    @mutex.synchronize { @requests.clear }
  end

  def request_count
    @requests.size
  end

  # Elapsed ms between consecutive requests, from monotonic captures.
  # Wall-clock (Time.now) can step or slew mid-test and read a sleep as
  # shorter than it was — the delay-flake class from #496. Monotonic
  # deltas mirror the Go runner's time.Time subtraction.
  def delays_between_requests
    return [] if @requests.size < 2

    @requests.each_cons(2).map do |a, b|
      ((b[:monotonic_time] - a[:monotonic_time]) * 1000).to_i # milliseconds
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

  def call(operation, path_params: {}, query_params: {}, body: nil, path: "", max_items: nil, page: nil)
    case operation
    when "DownloadURL"
      raise "DownloadURL test case requires a non-empty path" if path.nil? || path.empty?
      raw_url = "https://storage.3.basecamp.com" + path
      @account.download_url(raw_url)
    when "ListProjects"
      # Consumed and summarized HERE rather than returned unconsumed: the
      # summary is what lets a fixture assert on the accumulated items, and
      # building it requires the walk to have finished anyway. The plain arity
      # stays exercised when the fixture carries neither a maxItems cap nor a
      # pinned page.
      summarize_projects(
        if max_items || page
          @account.projects.list(max_items: max_items, page: page)
        else
          @account.projects.list
        end
      )
    when "Search"
      # Consumed and summarized HERE for the same reason ListProjects is: the
      # summary is what lets a fixture assert on the decoded hits.
      summarize_search(@account.search.search(q: SEARCH_QUERY))
    when "GetProject"
      @account.projects.get(project_id: path_params["projectId"])
    when "CreateProject"
      @account.projects.create(name: body["name"])
    when "ListTodos"
      max_items ? \
        @account.todos.list(todolist_id: path_params["todolistId"], max_items: max_items) : \
        @account.todos.list(todolist_id: path_params["todolistId"])
    when "GetTodo"
      @account.todos.get(
        todo_id: path_params["todoId"]
      )
    when "CreateTodo"
      @account.todos.create(
        todolist_id: path_params["todolistId"],
        content: body["content"]
      )
    when "ListMyBookmarks"
      @account.bookmarks.list_my_bookmarks.to_a
    when "ListMyDrafts"
      @account.drafts.list_my_drafts.to_a
    when "GetMyNote"
      @account.my_notes.get_my_note
    when "PrioritizeAssignment"
      @account.my_assignments.prioritize_assignment(id: body["id"])
    when "DeprioritizeAssignment"
      @account.my_assignments.deprioritize_assignment(recording_id: path_params["recordingId"])
    when "ReorderUpNext"
      @account.my_assignments.reorder_up_next(source_id: body["source_id"], position: body["position"])
    when "GetCalendar"
      @account.calendars.get_calendar(calendar_id: path_params["calendarId"])
    when "UpdateCalendar"
      @account.calendars.update_calendar(calendar_id: path_params["calendarId"], calendar: body["calendar"])
    when "UpdateMyNote"
      @account.my_notes.update_my_note(note: body["note"])
    when "GetBookmark"
      @account.bookmarks.get_bookmark(recording_id: path_params["recordingId"])
    when "CreateBookmark"
      @account.bookmarks.create_bookmark(recording_id: path_params["recordingId"])
    when "DeleteBookmark"
      @account.bookmarks.delete_bookmark(recording_id: path_params["recordingId"])
    when "ListFolders"
      @account.folders.list_folders
    when "GetFolder"
      @account.folders.get_folder(folder_id: path_params["folderId"])
    when "CreateFolder"
      @account.folders.create_folder(name: body["name"], project_ids: body["project_ids"])
    when "UpdateFolder"
      @account.folders.update_folder(folder_id: path_params["folderId"], name: body["name"])
    when "DeleteFolder"
      @account.folders.delete_folder(folder_id: path_params["folderId"])
    when "CreateTodosetTodo"
      @account.todos.create_todoset_todo(
        bucket_id: path_params["bucketId"],
        todoset_id: path_params["todosetId"],
        content: body["content"]
      )
    when "GetTimesheetEntry"
      @account.timesheets.get(
        entry_id: path_params["entryId"]
      )
    when "DestroyTimesheetEntry"
      @account.timesheets.destroy(
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
    # The window is fixed here rather than read from the case: no mock runner
    # consumes query_params, and no assertion type can pin a query string. Both
    # bounds are required, so the call cannot be made without them. The flat
    # summary keeps the responseBody assertions portable to Go and TypeScript,
    # which resolve only top-level keys.
    when "GetUpcomingSchedule"
      summarize_upcoming(
        @account.reports.upcoming(
          window_starts_on: UPCOMING_WINDOW_START,
          window_ends_on: UPCOMING_WINDOW_END
        )
      )
    when "GetTool"
      @account.tools.get(
        tool_id: path_params["toolId"]
      )
    when "CreateTool"
      @account.tools.create(
        bucket_id: path_params["bucketId"],
        tool_type: body["tool_type"],
        title: body["title"]
      )
    when "EnableTool"
      @account.tools.enable(
        tool_id: path_params["toolId"]
      )
    when "UploadsDownload"
      @account.uploads.download(upload_id: path_params["uploadId"])
    when "CreateUploadVersion"
      # Presence-bearing, like ReplaceScheduleEntry: only keys the fixture
      # carries are passed, so an unaddressed description stays off the wire
      # while an explicit "" survives compact_params (which strips only nil).
      @account.uploads.create_version(
        upload_id: path_params["uploadId"],
        attachable_sgid: body["attachable_sgid"],
        **upload_version_write_kwargs(body)
      )
    when "UpdateUpload"
      @account.uploads.update(upload_id: path_params["uploadId"], **upload_version_write_kwargs(body))
    when "ListUploadVersions"
      summarize_upload_versions(@account.uploads.list_versions(upload_id: path_params["uploadId"]).to_a)
    when "UpdateTodo"
      @account.todos.update(
        todo_id: path_params["todoId"],
        **todo_write_kwargs(body)
      )
    # ReplaceScheduleEntry is the real operationId; UpdateScheduleEntry and
    # EditScheduleEntry are SYNTHETIC scenario keys. All three ride the one wire
    # operation (PUT /schedule_entries/{id}) and name the three SDK surfaces
    # over it, so the fixture can pin each one's request shape.
    # `url`, `highlighted` and `status` are the three #641 members. The write
    # spelling is `url`; `join_url` is read-only and BC3 drops it from a write
    # body without complaining.
    when "CreateScheduleEntry"
      @account.schedules.create_entry(
        schedule_id: path_params["scheduleId"],
        **schedule_entry_create_kwargs(body)
      )
    when "ReplaceScheduleEntry"
      # Raw single PUT, no read-before-write. Presence-bearing: only keys the
      # fixture carries are passed, so an absent url stays off the wire while an
      # explicit "" survives compact_params (which strips only nil).
      @account.schedules.replace_entry(entry_id: path_params["entryId"], **schedule_entry_write_kwargs(body))
    when "UpdateScheduleEntry"
      # Merge-safe composite: GET then PUT of the five full-state members, plus
      # only the carve-outs the caller addressed.
      @account.schedules.update_entry(entry_id: path_params["entryId"], **schedule_entry_write_kwargs(body))
    when "EditScheduleEntry"
      # Read-modify-write closure. Assigning by name is what makes the carve-out
      # dirty tracking observable: a key the fixture omits is never assigned, so
      # it never reaches the wire, while a key whose value equals the read-back
      # is assigned and therefore does.
      @account.schedules.edit_entry(entry_id: path_params["entryId"]) do |entry|
        (body || {}).each { |key, value| entry.public_send("#{key}=", value) }
      end
    when "UpdateCard"
      # Merge-safe composite: GET then PUT, resending the fetched due_on.
      @account.cards.update(card_id: path_params["cardId"], **card_write_kwargs(body))
    when "UpdateCardVerbatim"
      # Raw single PUT, no read-before-write.
      @account.cards.update_verbatim(card_id: path_params["cardId"], **card_write_kwargs(body))
    when "EditTodo"
      @account.todos.edit(todo_id: path_params["todoId"]) do |t|
        (body || {}).each { |key, value| t.public_send("#{key}=", value) }
      end
    when "ReplaceTodo"
      @account.todos.replace(
        todo_id: path_params["todoId"],
        **todo_write_kwargs(body)
      )
    # #544: a group is a Todolist. GET /todolists/{id} answers the flat
    # recordable JSON for a list and for a group alike, so this one dispatch
    # serves both read cases in todolists_read.json. The Hash is returned as
    # the case RESULT so the responseBody assertions read the value the SDK
    # handed back — wire-shaped keys, since Ruby's generated get returns
    # http_get(...).json verbatim.
    when "GetTodolistOrGroup"
      @account.todolists.get(id: path_params["id"])
    # The group list returns an array of that same flat shape. Convention for
    # this case, stated in the fixture's own description: the FIRST decoded
    # element is the result, so responseBody reads element 0. .to_a is
    # load-bearing (paginate is lazy, so an unconsumed enumerator never reaches
    # the wire); an empty list fails rather than yielding nil for every path.
    when "ListTodolistGroups"
      groups = @account.todolist_groups.list(todolist_id: path_params["todolistId"]).to_a
      raise "ListTodolistGroups returned no groups; expected at least one to assert on" if groups.empty?

      groups.first
    # UpdateTodolist / EditTodolist / ReplaceTodolist are SYNTHETIC scenario
    # keys, not spec operationIds: all three ride the one real operation,
    # UpdateTodolistOrGroup (PUT /todolists/{id}). They name the three SDK
    # surfaces over it so the fixture can pin each one's request shape.
    when "UpdateTodolist"
      # Merge-safe composite: GET then PUT, resending the fetched description.
      # Variant-agnostic — a todolist group answers the same route and takes
      # the same path with no branching.
      @account.todolists.update(id: path_params["id"], **todolist_write_kwargs(body))
    when "EditTodolist"
      @account.todolists.edit(id: path_params["id"]) do |list|
        (body || {}).each { |key, value| list.public_send("#{key}=", value) }
      end
    when "ReplaceTodolist"
      # Raw single PUT, no read-before-write: omitted fields stay omitted.
      @account.todolists.replace(id: path_params["id"], **todolist_write_kwargs(body))
    when "UpdateDocument"
      # Merge-safe composite: GET then PUT of the full {title, content}.
      @account.documents.update(document_id: path_params["documentId"], **document_write_kwargs(body))
    when "EditDocument"
      @account.documents.edit(document_id: path_params["documentId"]) do |doc|
        (body || {}).each { |key, value| doc.public_send("#{key}=", value) }
      end
    when "ReplaceDocument"
      # Raw single PUT, no read-before-write: omitted fields stay omitted.
      @account.documents.replace(document_id: path_params["documentId"], **document_write_kwargs(body))
    when "GetEverythingMessages"
      @account.everything.get_everything_messages.to_a
    when "GetEverythingComments"
      @account.everything.get_everything_comments.to_a
    when "GetEverythingCheckins"
      @account.everything.get_everything_checkins.to_a
    when "GetEverythingForwards"
      @account.everything.get_everything_forwards.to_a
    when "GetEverythingFiles"
      @account.everything.get_everything_files.to_a
    when "GetEverythingOverdueTodos"
      @account.everything.get_everything_overdue_todos
    when "GetEverythingOverdueCards"
      @account.everything.get_everything_overdue_cards
    when "GetEverythingOpenTodos"
      @account.everything.get_everything_open_todos.to_a
    when "GetEverythingCompletedTodos"
      @account.everything.get_everything_completed_todos.to_a
    when "GetEverythingUnassignedTodos"
      @account.everything.get_everything_unassigned_todos.to_a
    when "GetEverythingNoDueDateTodos"
      @account.everything.get_everything_no_due_date_todos.to_a
    when "GetEverythingOpenCards"
      @account.everything.get_everything_open_cards.to_a
    when "GetEverythingCompletedCards"
      @account.everything.get_everything_completed_cards.to_a
    when "GetEverythingUnassignedCards"
      @account.everything.get_everything_unassigned_cards.to_a
    when "GetEverythingNoDueDateCards"
      @account.everything.get_everything_no_due_date_cards.to_a
    when "GetEverythingNotNowCards"
      @account.everything.get_everything_not_now_cards.to_a
    when "ListForwards"
      # .to_a is load-bearing: paginate is lazy, so an unconsumed enumerator
      # never reaches the wire and requestPath/requestCount would see nothing.
      @account.forwards.list(inbox_id: path_params["inboxId"]).to_a
    # #588: nine flat spellings bc3 only draws bucket-scoped. Each pins the
    # bucketId segment on the wire — the segment whose absence made them 404.
    # .to_a on the list operations is load-bearing (lazy paginate).
    when "ListChatbots"
      @account.campfires.list_chatbots(
        bucket_id: path_params["bucketId"],
        campfire_id: path_params["campfireId"]
      ).to_a
    when "GetChatbot"
      @account.campfires.get_chatbot(
        bucket_id: path_params["bucketId"],
        campfire_id: path_params["campfireId"],
        chatbot_id: path_params["chatbotId"]
      )
    when "CreateChatbot"
      @account.campfires.create_chatbot(
        bucket_id: path_params["bucketId"],
        campfire_id: path_params["campfireId"],
        service_name: body["service_name"],
        command_url: body["command_url"]
      )
    when "UpdateChatbot"
      @account.campfires.update_chatbot(
        bucket_id: path_params["bucketId"],
        campfire_id: path_params["campfireId"],
        chatbot_id: path_params["chatbotId"],
        service_name: body["service_name"],
        command_url: body["command_url"]
      )
    when "DeleteChatbot"
      @account.campfires.delete_chatbot(
        bucket_id: path_params["bucketId"],
        campfire_id: path_params["campfireId"],
        chatbot_id: path_params["chatbotId"]
      )
    when "ListClientApprovals"
      @account.client_approvals.list(bucket_id: path_params["bucketId"]).to_a
    when "ListClientCorrespondences"
      @account.client_correspondences.list(bucket_id: path_params["bucketId"]).to_a
    when "ListClientReplies"
      @account.client_replies.list(
        bucket_id: path_params["bucketId"],
        recording_id: path_params["recordingId"]
      ).to_a
    when "GetClientReply"
      @account.client_replies.get(
        bucket_id: path_params["bucketId"],
        recording_id: path_params["recordingId"],
        reply_id: path_params["replyId"]
      )
    when "RepositionTodolistGroup"
      @account.todolist_groups.reposition(
        group_id: path_params["groupId"],
        position: body["position"]
      )
    else
      raise "Unknown operation: #{operation}"
    end
  end

  private

  # The date window every GetUpcomingSchedule case is dispatched with. Fixed in
  # the runner because no mock runner consumes query_params and no assertion type
  # can pin a query string — every runner records the path with the query
  # stripped.
  UPCOMING_WINDOW_START = "2026-06-01"
  UPCOMING_WINDOW_END = "2026-06-30"

  # The query every Search case is dispatched with, fixed for the same reason.
  # It is required, and the mock returns its queued body regardless of what is
  # asked for.
  SEARCH_QUERY = "Leto"

  # Flatten a search result list into top-level scalars, one group per branch of
  # BC3's polymorphic search projection.
  #
  # Flat and scalar for the reason summarize_projects gives. Boolean for a
  # second reason: the response is an ARRAY and no assertion type expresses
  # absence inside one — there is headerAbsent and requestBodyAbsent, but no
  # responseBodyAbsent — and the file-attachment branch is recognized precisely
  # BY the absence of the five envelope keys. Encoding that as a boolean is the
  # established idiom (last_has_upload).
  #
  # Each hit is selected by predicate rather than by index, so a fixture can
  # present one branch alone and still assert the others report honestly.
  #
  # Ruby is lenient — `search` hands back the parsed hashes verbatim — so this
  # reads the wire keys directly; the strict tiers (Swift, Kotlin) build the
  # same summary out of decoded models, which is where the contract is enforced.
  def summarize_search(enum)
    results = enum.to_a
    generic = results.find { |r| !r["type"].nil? }
    attachment = results.find { |r| r["type"].nil? }
    upload_line = results.find { |r| r["type"] == "Chat::Lines::Upload" }
    needle = results.find { |r| r["type"] == "Gauge::Needle" }
    kanban = results.find { |r| r["type"] == "Kanban::Column" }

    upload_attachment = (upload_line&.fetch("attachments", nil) || [])[0] || {}
    needle_attachment = (needle&.fetch("attachments", nil) || [])[0] || {}
    generic ||= {}
    attachment ||= {}
    upload_line ||= {}
    needle ||= {}
    kanban ||= {}

    {
      "result_count" => results.length,
      "bubble_up_url_count" => results.count { |r| !r["bubble_up_url"].nil? },

      # The generic recording envelope — the control group.
      "generic_type" => generic["type"] || "",
      "generic_has_id" => !generic["id"].nil?,
      "generic_has_title" => !generic["title"].nil?,
      "generic_has_type" => !generic["type"].nil?,
      "generic_has_url" => !generic["url"].nil?,
      "generic_has_app_url" => !generic["app_url"].nil?,

      # The file-attachment branch: searches/_attachment.json.jbuilder writes
      # its own projection, so the absence of a type IS the discriminator.
      "attachment_has_id" => !attachment["id"].nil?,
      "attachment_has_title" => !attachment["title"].nil?,
      "attachment_has_type" => !attachment["type"].nil?,
      "attachment_has_url" => !attachment["url"].nil?,
      "attachment_has_app_url" => !attachment["app_url"].nil?,
      "attachment_has_content" => !attachment["content"].nil?,
      "attachment_has_description" => !attachment["description"].nil?,
      "attachment_filename" => attachment["filename"] || "",
      "attachment_content_type" => attachment["content_type"] || "",
      "attachment_byte_size" => attachment["byte_size"] || 0,
      "attachment_previewable" => attachment["previewable"] || false,
      # Narrowed HERE, not by the SDK: Ruby has no typed search model, so a
      # float-spelled 1920.0 reaches the caller as a Float. The narrowing is
      # load-bearing in the statically-typed tiers (Go, Kotlin, Swift), where
      # the model declares an integer and a plain int decode would throw.
      "attachment_width" => attachment["width"]&.to_i || 0,
      "attachment_height" => attachment["height"]&.to_i || 0,

      # The chat upload line: a bespoke six-key attachments aggregate carrying
      # title/url and NONE of the rich-text id/sgid/preview keys.
      "upload_line_type" => upload_line["type"] || "",
      "upload_boosts_count" => upload_line["boosts_count"] || 0,
      "upload_attachment_filename" => upload_attachment["filename"] || "",
      "upload_attachment_has_title" => !upload_attachment["title"].nil?,
      "upload_attachment_has_id" => !upload_attachment["id"].nil?,
      "upload_attachment_has_sgid" => !upload_attachment["sgid"].nil?,

      # The gauge needle: the same attachments key carrying the OTHER variant —
      # the rich-text one, with id and sgid populated.
      "needle_type" => needle["type"] || "",
      "needle_color" => needle["color"] || "",
      "needle_position" => needle["position"] || 0,
      "needle_comments_count" => needle["comments_count"] || 0,
      "needle_comment_count" => needle["comment_count"] || 0,
      "needle_boosts_count" => needle["boosts_count"] || 0,
      "needle_attachment_has_id" => !needle_attachment["id"].nil?,
      "needle_attachment_has_sgid" => !needle_attachment["sgid"].nil?,
      "needle_attachment_width" => needle_attachment["width"]&.to_i || 0,

      # The kanban list: list-partial keys over the envelope, on_hold nested,
      # and a color emitted unconditionally with a null value.
      "kanban_type" => kanban["type"] || "",
      "kanban_position" => kanban["position"] || 0,
      "kanban_cards_count" => kanban["cards_count"] || 0,
      "kanban_comment_count" => kanban["comment_count"] || 0,
      "kanban_subscriber_count" => (kanban["subscribers"] || []).length,
      "kanban_has_color" => !kanban["color"].nil?,
      "kanban_has_on_hold" => !kanban["on_hold"].nil?,
      "kanban_on_hold_cards_count" => kanban.dig("on_hold", "cards_count") || 0
    }
  end

  # Flatten the upcoming-schedule envelope into top-level scalars. Go and
  # TypeScript resolve a responseBody path as a top-level key only, so the
  # assertions read scalars rather than walk into the arrays. Ruby is lenient —
  # `upcoming` hands back the parsed body verbatim — so this reads the wire keys
  # directly; the strict tiers (Swift, Kotlin) build the same summary out of
  # decoded models, which is where the contract is enforced.
  def summarize_upcoming(envelope)
    entries = envelope["schedule_entries"]
    occurrences = envelope["recurring_schedule_entry_occurrences"]
    assignables = envelope["assignables"]

    summary = {
      "schedule_entries_count" => entries.length,
      "recurring_occurrences_count" => occurrences.length,
      "assignables_count" => assignables.length
    }
    if entries.any?
      summary["entry_summary"] = entries[0]["summary"]
      summary["entry_recurring"] = entries[0]["recurring"]
      summary["entry_bucket_name"] = entries[0].dig("bucket", "name")
    end
    if occurrences.any?
      summary["occurrence_recurring"] = occurrences[0]["recurring"]
      summary["occurrence_all_day"] = occurrences[0]["all_day"]
      summary["occurrence_starts_at"] = occurrences[0]["starts_at"]
    end
    if assignables.any?
      summary["assignable_content"] = assignables[0]["content"]
      summary["assignable_type"] = assignables[0]["type"]
      summary["assignable_parent_title"] = assignables[0].dig("parent", "title")
      summary["assignable_completion_url"] = assignables[0]["completion_url"]
    end
    summary
  end

  # Flatten an accumulated project list into top-level scalars.
  #
  # Flat and scalar because that is the only path form every runner can
  # resolve: Go and TypeScript read a responseBody path as a top-level key with
  # no dot splitting, and the Swift and Kotlin navigators descend through
  # objects only, so neither a dotted path nor an array index is portable.
  #
  # It exists so a fixture can prove the items of a followed page were
  # ACCUMULATED, not merely fetched. requestCount only sees that the second
  # request happened, and meta.totalCount is the X-Total-Count header rather
  # than the item count, so an SDK that fetched page 2 and discarded its body
  # satisfies both.
  #
  # The returned Hash replaces the enumerator for BOTH assertion families, so
  # it carries the two responseMeta fields under their JSON names as well; the
  # responseMeta arm falls back to a Hash lookup when the result has no #meta.
  # `truncated` is final only after the enumeration completes, which #to_a is.
  def summarize_projects(enum)
    items = enum.to_a
    {
      "project_count" => items.length,
      "first_project_id" => project_id(items.first),
      "last_project_id" => project_id(items.last),
      "totalCount" => enum.meta.total_count,
      "truncated" => enum.meta.truncated
    }
  end

  # The "id" of a list item, or 0 when the item is absent or not an object.
  #
  # Single-key envelope bodies (retry.json's legacy `{"projects": []}`) no
  # longer reach the SDK — normalize_body unwraps them at the mock, in parity
  # with the other runners — so list items are expected to be Hashes. The
  # guard remains as a backstop for any future fixture whose items are not
  # objects.
  def project_id(item)
    item.is_a?(Hash) ? item["id"] || 0 : 0
  end

  # Fixture requestBody keys map 1:1 to the todo write kwargs.
  TODO_WRITE_KEYS = %w[
    content description assignee_ids completion_subscriber_ids due_on starts_on notify
  ].freeze

  # The full writable vocabulary of PUT /schedule_entries/{id}: five full-state
  # members plus the four addressed-only ones (participant_ids, url and
  # highlighted are BC3's preservedOnOmission carve-out; notify is a directive).
  SCHEDULE_ENTRY_WRITE_KEYS = %w[
    summary starts_at ends_at description all_day participant_ids notify url highlighted
  ].freeze

  # Create takes `status` too — it is a Recording column, so BC3 reads it
  # outside the schedule_entry envelope and it is not a ReplaceScheduleEntry
  # member.
  SCHEDULE_ENTRY_CREATE_KEYS = (SCHEDULE_ENTRY_WRITE_KEYS + %w[ status ]).freeze

  def schedule_entry_create_kwargs(body)
    SCHEDULE_ENTRY_CREATE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  def schedule_entry_write_kwargs(body)
    SCHEDULE_ENTRY_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  # attachable_sgid is passed positionally by the dispatch (it is required), so
  # it is deliberately absent here — this list is only the presence-bearing
  # members, where "sent as empty" and "not sent" are different writes.
  UPLOAD_VERSION_WRITE_KEYS = %w[base_name description notify subscriptions].freeze

  def upload_version_write_kwargs(body)
    UPLOAD_VERSION_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  # GET /uploads/{id}/versions.json returns an ARRAY, and the assertion path
  # resolvers walk objects, not array indices. Flatten to the same summary shape
  # summarize_upcoming uses so the fixture can assert on it.
  def summarize_upload_versions(versions)
    summary = {
      "versions_count" => versions.length,
      "current_count" => versions.count { |v| v.dig("upload", "current") }
    }
    if versions.any?
      first = versions.first
      summary["first_action"] = first["action"]
      summary["first_filename"] = first.dig("upload", "filename")
      summary["first_content_type"] = first.dig("upload", "content_type")
      summary["first_byte_size"] = first.dig("upload", "byte_size")
      summary["first_current"] = first.dig("upload", "current")

      last = versions.last
      summary["last_action"] = last["action"]
      # A version whose recordable no longer resolves omits the upload object
      # entirely — the optionality UploadVersion.upload declares.
      summary["last_has_upload"] = !last["upload"].nil?
    end
    summary
  end

  CARD_WRITE_KEYS = %w[title content due_on assignee_ids].freeze

  def card_write_kwargs(body)
    CARD_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  def todo_write_kwargs(body)
    TODO_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  # BC3 permits exactly {name, description} on PUT /todolists/{id}. Only the
  # keys the fixture carries are passed: nil is the composite's "keep it"
  # signal and compact_params strips it on the raw path, so an absent key must
  # stay absent rather than arriving as an explicit nil.
  TODOLIST_WRITE_KEYS = %w[name description].freeze
  DOCUMENT_WRITE_KEYS = %w[title content].freeze

  def todolist_write_kwargs(body)
    TODOLIST_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end

  def document_write_kwargs(body)
    DOCUMENT_WRITE_KEYS.select { |key| (body || {}).key?(key) } \
      .to_h { |key| [key.to_sym, body[key]] }
  end
end

# Test result
TestResult = Struct.new(:name, :passed, :message)

# Tests where the Ruby SDK's behavior intentionally differs.
#
# The Ruby SDK only retries GET requests (see Http#request). PUT and DELETE
# are sent once even though they're naturally idempotent. Tests asserting
# mutation-retry behavior are skipped.
RUBY_SKIPS = Set.new([
  "PUT operation is naturally idempotent",
  "DELETE operation is naturally idempotent",
  "POST operation retries when marked idempotent",
  "Subscribe POST retries when marked idempotent",
  "CreateBookmark POST retries when marked idempotent",
  "DeleteBookmark DELETE retries when marked idempotent",
  "UpdateMyNote PUT retries when marked idempotent",
  "UpdateCalendar PUT retries when marked idempotent",
  "PrioritizeAssignment POST retries when marked idempotent",
  "DeprioritizeAssignment DELETE retries when marked idempotent",
  "Network error on an idempotent POST is retried then succeeds",
].freeze)

RUBY_SKIP_REASONS = {
  "PUT operation is naturally idempotent" => "Ruby SDK only retries GET",
  "DELETE operation is naturally idempotent" => "Ruby SDK only retries GET",
  "POST operation retries when marked idempotent" => "Ruby SDK only retries GET",
  "Subscribe POST retries when marked idempotent" => "Ruby SDK only retries GET",
  "CreateBookmark POST retries when marked idempotent" => "Ruby SDK only retries GET",
  "DeleteBookmark DELETE retries when marked idempotent" => "Ruby SDK only retries GET",
  "UpdateMyNote PUT retries when marked idempotent" => "Ruby SDK only retries GET",
  "UpdateCalendar PUT retries when marked idempotent" => "Ruby SDK only retries GET",
  "PrioritizeAssignment POST retries when marked idempotent" => "Ruby SDK only retries GET",
  "DeprioritizeAssignment DELETE retries when marked idempotent" => "Ruby SDK only retries GET",
  "Network error on an idempotent POST is retried then succeeds" => "Ruby SDK only retries GET network errors; mutations go through single_request with no retry",
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

    # Defense-in-depth backstop for the operationally-harmful mockResponses
    # shapes (neither mode set, or both active). The AUTHORITATIVE oneOf
    # enforcement is `make conformance-fixtures-check` (check-jsonschema against
    # conformance/schema.json), which runs before the runners and rejects
    # {status, networkError:false} / non-true networkError that this truthiness
    # backstop intentionally lets through for cross-runner parity.
    (@test["mockResponses"] || []).each_with_index do |r, i|
      has_status = r.key?("status")
      has_network_error = r["networkError"] == true
      if has_status == has_network_error
        return TestResult.new(@test["name"], false, "mockResponses[#{i}] must set exactly one of status or networkError")
      end
    end

    setup_mock_responses

    begin
      result = @mapper.call(
        @test["operation"],
        path_params: @test["pathParams"] || {},
        query_params: @test["queryParams"] || {},
        body: @test["requestBody"],
        path: @test["path"] || "",
        max_items: (@test["configOverrides"] || {})["maxItems"],
        page: (@test["configOverrides"] || {})["page"]
      )
      # Consume-then-assert: pagination metadata (truncated in particular) is
      # final only after the lazy enumeration completes, and consumption is
      # what drives the follow-up page requests that requestCount observes.
      # The enumerator itself is kept so responseMeta can reach .meta.
      #
      # A no-op for ListProjects, whose arm consumes and summarizes in place
      # (summarize_projects) so responseBody can assert on the accumulated
      # items — every OTHER list arm still returns an unconsumed enumerator and
      # still needs this.
      result.to_a if result.is_a?(Enumerator)
      verify_assertions(result: result, error: nil)
    rescue StandardError => e
      verify_assertions(result: nil, error: e)
    end
  end

  private

  # Normalizes a mock response body for SDK compatibility.
  #
  # Conformance test fixtures may wrap arrays in objects (e.g.,
  # `{"projects": [...]}`), but the Ruby SDK's list operations expect a raw
  # JSON array. When the body is a JSON object with a single key whose value
  # is an array, unwrap it — matching the other runners' semantics.
  #
  # Success bodies only: an error body with one array-valued key is the
  # unwrapped field map (`{"payload_url": ["is invalid"]}`), and unwrapping
  # it would rewrite the fixture on the wire.
  def normalize_body(body, status)
    if (status || 200) < 400 && body.is_a?(Hash) && body.size == 1 && body.values.first.is_a?(Array)
      body.values.first
    else
      body
    end
  end

  def setup_mock_responses
    responses = @test["mockResponses"] || []
    return if responses.empty?

    # Queue up responses
    response_queue = responses.map do |r|
      if r["networkError"]
        { network_error: true }
      else
        {
          status: r["status"],
          body: normalize_body(r["body"], r["status"])&.to_json || "",
          headers: { "Content-Type" => "application/json" }.merge(r["headers"] || {})
        }
      end
    end

    # Method-agnostic catch-all on the active client's origin (derived from
    # configOverrides.baseUrl when present): the SDK decides method and path
    # (including multi-hop flows like DownloadURL's relative Location
    # resolution), while a misroute to a different host fails instead of
    # consuming a queued response. Path correctness is enforced by the
    # implicit first-request invariant and requestPath assertions.
    stub = WebMock.stub_request(:any, %r{\A#{Regexp.escape(api_origin)}/})

    paginates = auto_paginates?
    call_count = 0
    stub.to_return do |request|
      @tracker.record_request(
        monotonic_time: Process.clock_gettime(Process::CLOCK_MONOTONIC),
        method: request.method,
        uri: request.uri,
        headers: request.headers,
        body: parse_json_body(request.body)
      )
      if call_count < response_queue.size
        resp = response_queue[call_count]
        call_count += 1
        # Genuine transport failure for this queued entry only: raise a Faraday
        # connection error (the SDK's rescue Faraday::Error path maps it to a
        # NetworkError). The request is already recorded above, so requestCount
        # is correct. Raising in-block keeps the rest of the queue intact,
        # unlike a blanket stub .to_raise.
        raise Faraday::ConnectionFailed, "simulated network error" if resp[:network_error]

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

  # The active API origin: configOverrides.baseUrl when present, else the
  # default runner base URL. Normalized to WebMock's canonical request-URI
  # form (lowercase scheme/host, default port dropped) so a mixed-case or
  # explicit-default-port baseUrl still matches the stub.
  def api_origin
    overrides = @test["configOverrides"] || {}
    uri = URI.parse(overrides["baseUrl"] || "https://3.basecampapi.com")
    port_part = uri.port && uri.port != uri.default_port ? ":#{uri.port}" : ""
    "#{uri.scheme.downcase}://#{uri.host.downcase}#{port_part}"
  end

  def parse_json_body(raw)
    raw.nil? || raw.empty? ? nil : JSON.parse(raw)
  rescue JSON::ParserError
    nil
  end

  # The test case path with pathParams substituted.
  def substituted_path
    (@test["pathParams"] || {}).reduce(@test["path"]) do |path, (key, value)|
      path.gsub("{#{key}}", value.to_s)
    end
  end

  # Return the captured request at index (0-based; negative counts from end), or nil if out of range.
  def request_at(index)
    requests = @tracker.requests
    n = requests.size
    resolved = index < 0 ? index + n : index
    resolved >= 0 && resolved < n ? requests[resolved] : nil
  end

  # Return captured headers at index (0-based; negative counts from end), or nil if out of range.
  def request_headers_at(index)
    request_at(index)&.[](:headers)
  end

  # Walk a dot-notation key path into a parsed JSON body. Returns a
  # [present, value] pair so an absent key is distinguishable from an
  # explicit null.
  def fetch_body_key(body, key_path)
    key_path.split(".").reduce([true, body]) do |(present, current), key|
      present && current.is_a?(Hash) && current.key?(key) ? [true, current[key]] : [false, nil]
    end
  end

  def verify_assertions(result:, error:)
    failures = []

    # DownloadURL implicit invariant: hop 1 must hit the test case path.
    # The mock route is origin-wide so hop 2's relative-resolved URL is
    # served, but a regression that misroutes hop 1 to a different path
    # on the same origin would otherwise pass silently.
    if @test["operation"] == "DownloadURL" && @tracker.requests.any?
      expected_path = @test["path"]
      actual_path = URI.parse(@tracker.requests.first[:uri]).path
      unless actual_path == expected_path
        failures << "DownloadURL hop 1 expected path #{expected_path.inspect}, got #{actual_path.inspect}"
      end
    end

    # Generic implicit invariant for every other operation that defines a
    # path: the mock route is origin-wide, so the first request must target
    # the pathParams-substituted fixture path — a misrouted request on the
    # same origin would otherwise consume a queued response silently.
    if @test["operation"] != "DownloadURL" && !@test["path"].to_s.empty? && @tracker.requests.any?
      expected_path = substituted_path
      actual_path = URI.parse(@tracker.requests.first[:uri]).path
      unless actual_path.include?(expected_path)
        failures << "Expected first request path to contain #{expected_path.inspect}, got #{actual_path.inspect}"
      end
    end

    # Implicit method invariant: the mock queue is method-agnostic, so a
    # wrong-verb request (e.g. a PUT regressing to POST) would consume a
    # queued response silently. When the fixture declares a method and
    # carries no explicit requestMethod assertions, the first request must
    # use the fixture method.
    has_method_assertions = (@test["assertions"] || []).any? { |a| a["type"] == "requestMethod" }
    if @test["method"] && !has_method_assertions && @tracker.requests.any?
      expected_method = @test["method"].upcase
      actual_method = @tracker.requests.first[:method].to_s.upcase
      unless actual_method == expected_method
        failures << "Expected first request method #{expected_method.inspect}, got #{actual_method.inspect}"
      end
    end

    (@test["assertions"] || []).each do |assertion|
      case assertion["type"]
      when "requestCount"
        # The Ruby SDK auto-paginates list operations, so a fixture that counts
        # first-page requests only is inapplicable — but ONLY its count is. The
        # rest of the case still runs. See RequestCount.applies? (#573).
        if RequestCount.applies?(@test["tags"])
          failure = RequestCount.check(@tracker.request_count, assertion["expected"])
          failures << failure if failure
        end

      when "delayBetweenRequests"
        # Not all gaps are retry gaps — the download flow's final gap is the
        # redirect hop to the signed URL, which is deliberately un-delayed —
        # so those fixtures name a gap with an index. See DelayGaps.check for
        # the contract.
        #
        # An absent `min` still asserts that the gap EXISTS, so it defaults to
        # zero rather than skipping the assertion: gating on presence degrades
        # the check to nothing, the very false-green class this exists to kill.
        failure = DelayGaps.check(@tracker.delays_between_requests, assertion["min"], assertion["index"],
          @tracker.request_count)
        failures << failure if failure

      when "noError"
        if error
          failures << "Expected no error, got: #{error.class}: #{error.message}"
        end

      # The inverse of noError, and deliberately code-agnostic. See
      # ErrorRaised.check for the contract and for why the branch lives there
      # rather than inline: no committed fixture can reach its failing side, so
      # it is unit-tested instead.
      when "errorRaised"
        failure = ErrorRaised.check(!error.nil?)
        failures << failure if failure

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
          "network" => Basecamp::ErrorCode::NETWORK,
        }
        expected_code = code_map[expected_type]
        if expected_code.nil?
          failures << "Unknown conformance error type #{expected_type.inspect} (add to code_map)"
        else
          # Require a canonical code that exists and matches — an error that
          # carries no code must fail, not silently pass.
          actual_code = error.respond_to?(:code) ? error.code : nil
          if actual_code.nil?
            failures << "Expected error code #{expected_code.inspect}, but #{error.class} carries no code: #{error}"
          elsif actual_code != expected_code
            failures << "Expected error code #{expected_code.inspect}, got #{actual_code.inspect}"
          end
        end

      when "requestPath"
        expected = assertion["expected"]
        idx = assertion["index"] || 0
        request = request_at(idx)
        if request.nil?
          failures << "Expected request path #{expected.inspect} on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        else
          actual_path = URI.parse(request[:uri]).path
          unless actual_path == expected
            failures << "Expected request path #{expected.inspect} on request index #{idx}, got #{actual_path.inspect}"
          end
        end

      when "requestMethod"
        expected = assertion["expected"]
        idx = assertion["index"] || 0
        request = request_at(idx)
        if request.nil?
          failures << "Expected request method #{expected.inspect} on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        else
          actual = request[:method].to_s.upcase
          unless actual == expected
            failures << "Expected request method #{expected.inspect} on request index #{idx}, got #{actual.inspect}"
          end
        end

      when "requestBody"
        key_path = assertion["path"]
        expected = assertion["expected"]
        idx = assertion["index"] || 0
        request = request_at(idx)
        if request.nil?
          failures << "Expected request body #{key_path} on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        elsif request[:body].nil?
          failures << "Expected request body #{key_path} on request index #{idx}, but the request had no JSON body"
        else
          present, actual = fetch_body_key(request[:body], key_path)
          if !present
            failures << "Expected request body key #{key_path.inspect} on request index #{idx}, but it was absent"
          elsif actual != expected
            failures << "Expected request body #{key_path} = #{expected.inspect} on request index #{idx}, got #{actual.inspect}"
          end
        end

      when "requestBodyAbsent"
        key_path = assertion["path"]
        idx = assertion["index"] || 0
        request = request_at(idx)
        if request.nil?
          failures << "Expected request body key #{key_path} absent on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        elsif request[:body]
          present, actual = fetch_body_key(request[:body], key_path)
          if present
            failures << "Expected request body key #{key_path.inspect} absent on request index #{idx}, got #{actual.inspect}"
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
        idx = assertion["index"] || 0
        headers = request_headers_at(idx)
        if headers.nil?
          failures << "Expected header #{header_name}=#{expected.inspect} on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        else
          actual = headers[header_name]
          unless actual == expected
            failures << "Expected header #{header_name}=#{expected.inspect} on request index #{idx}, got #{actual.inspect}"
          end
        end

      when "headerPresent"
        header_name = assertion["path"]
        idx = assertion["index"] || 0
        headers = request_headers_at(idx)
        if headers.nil?
          failures << "Expected header #{header_name} on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        else
          actual = headers[header_name]
          if actual.nil? || actual.empty?
            failures << "Expected header #{header_name} on request index #{idx}, but it was empty or missing"
          end
        end

      when "headerAbsent"
        header_name = assertion["path"]
        idx = assertion["index"] || 0
        headers = request_headers_at(idx)
        if headers.nil?
          failures << "Expected header #{header_name} absent on request index #{idx}, but only #{@tracker.request_count} requests were recorded"
        else
          actual = headers[header_name]
          unless actual.nil? || actual.empty?
            failures << "Expected header #{header_name} absent on request index #{idx}, got #{actual.inspect}"
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
        snake_field = field_path.gsub(/([A-Z])/) { "_#{Regexp.last_match(1).downcase}" }
        actual = if result.respond_to?(:meta) && result.meta.respond_to?(snake_field)
          result.meta.public_send(snake_field)
        elsif result.is_a?(Hash)
          # Key presence, not truthiness. `result[key] || result[key.to_sym]`
          # reads a present `false` as a miss and falls through to the symbol
          # key and then to nil, so `truncated => false` — the assertion every
          # non-truncating pagination fixture makes — could never pass once a
          # dispatch arm returns a summary Hash instead of an object with #meta.
          # Same trap dig_path already fixed below.
          result.key?(field_path) ? result[field_path] : result[field_path.to_sym]
        end
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
        # `current[key] || current[key.to_sym]` reads a present `false` as a
        # miss and falls through to the symbol key, then to nil — so a
        # responseBody assertion on a boolean field could never see false. Check
        # key presence instead of truthiness.
        current.key?(key) ? current[key] : current[key.to_sym]
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
    # maxRetries has to be in this list or a case overriding ONLY the retry cap
    # silently gets the shared default client and passes while testing nothing.
    has_max_retries = overrides.key?("maxRetries")
    return @mapper unless has_base_url || has_max_pages || has_max_retries

    begin
      config_opts = { base_url: has_base_url ? overrides["baseUrl"] : "https://3.basecampapi.com" }
      config_opts[:max_pages] = overrides["maxPages"] if has_max_pages
      config_opts[:max_retries] = overrides["maxRetries"] if has_max_retries
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
    # Case census (#602) — see CaseCensus. Taken up front, by its own walk, so a
    # fixture tree this runner's glob cannot see is reported before the run
    # rather than inferred from a short count afterwards.
    begin
      expected_cases = CaseCensus.non_live_case_count(@tests_dir)
    rescue CaseCensus::Error => e
      warn "Error taking fixture census: #{e.message}"
      return 1
    end

    # No early return on an empty glob. The census walks recursively and this
    # glob does not, so "the census found fixtures but this runner globbed none"
    # is exactly the nested-fixture under-count the census exists to reject —
    # and returning success here would step over the comparison that rejects it.
    # Falling through runs zero cases and lets the count check fail, which is
    # the correct answer.
    files = Dir.glob(File.join(@tests_dir, "*.json"))
    puts "No test files found in #{@tests_dir}" if files.empty?

    passed = 0
    failed = 0
    skipped = 0
    results = []
    # Recorded from the same branch that increments +skipped+, so the manifest
    # cannot claim a different set than the run took.
    excluded = []

    files.each do |file|
      # UTF-8 regardless of process locale (LC_ALL=C would otherwise read as US-ASCII)
      tests = JSON.parse(File.read(file, encoding: "UTF-8"))
      # Live tests are TS-only (canonical wire-capturer); accept only mock
      # so unresolved ${PROJECT_ID} fixtures and live-only operations don't
      # surface as mock failures or false passes — and any future mode added
      # to the schema enum stays opt-in for this runner.
      tests = tests.select { |t| CaseCensus.mock_mode?(t["mode"]) }
      next if tests.empty?

      puts "\n=== #{File.basename(file)} ==="

      tests.each do |test_case|
        if RUBY_SKIPS.include?(test_case["name"])
          skipped += 1
          reason = RUBY_SKIP_REASONS[test_case["name"]] || "Ruby SDK behavior differs"
          excluded << [ File.basename(file), test_case["name"], reason ]
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
    puts "Results: #{passed} passed, #{failed} failed, #{skipped} skipped " \
         "(fixtures declare #{expected_cases} non-live case(s))"

    count_failure = CaseCensus.count_failure(passed + failed + skipped, expected_cases)
    warn "\nFAIL: #{count_failure}" if count_failure

    # Written even when the run failed: a failing runner still has a truthful
    # exclusion set, and a missing manifest reads to the gate as "this runner
    # did not report", turning one failure into two.
    manifest_failure = nil
    begin
      ExecutionManifest.write("ruby", expected_cases, passed + failed, excluded)
    rescue ExecutionManifest::Error, SystemCallError => e
      manifest_failure = e
      warn "\nFAIL: could not write execution manifest: #{e.message}"
    end

    failed > 0 || count_failure || manifest_failure ? 1 : 0
  end
end

# Run if executed directly
if __FILE__ == $PROGRAM_NAME
  tests_dir = File.expand_path("../../tests", __dir__)
  runner = ConformanceRunner.new(tests_dir)
  exit runner.run
end
