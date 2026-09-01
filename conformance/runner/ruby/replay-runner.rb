#!/usr/bin/env ruby
# frozen_string_literal: true

# Wire-replay runner for the Ruby SDK conformance suite.
#
# Reads snapshots written by the canonical TS live runner (see
# conformance/runner/typescript/live-runner.test.ts), decodes each page
# through the Ruby SDK's decode boundary, and writes a per-test
# decode-result snapshot under <WIRE_REPLAY_DIR>/<BACKEND>/decode/ruby/.
#
# Mode-gate: invoking this script directly aborts unless WIRE_REPLAY_DIR
# and BASECAMP_BACKEND are set. The make target `conformance-ruby-replay`
# is the intended entrypoint and enforces both env vars in its preflight;
# the existing mock runner.rb is unaffected.

require "bundler/setup"
require "basecamp"
require "json"
require "fileutils"
require_relative "schema-walker"

class ReplayRunner
  SCHEMA_VERSION = 1

  # Maps operation_id -> proc(body_text) -> raises on parse/decode failure.
  #
  # The Ruby SDK has no typed deserializers — every response method ultimately
  # calls JSON.parse(body) followed by Http.normalize_person_ids(result), then
  # returns the resulting Hash/Array unchanged. That pipeline (the contents
  # of RequestResult#json plus the post-processing the paginators apply on
  # every page) is the canonical decoder boundary, so each lambda exercises
  # exactly that.
  SDK_DECODE = lambda do |body_text|
    # No empty-body guard: Basecamp::Http#json calls JSON.parse(@body)
    # directly, so an empty body surfaces as a JSON::ParserError in
    # production. The runner mirrors that to record decode_error rather
    # than silently green-passing on an empty wire payload.
    parsed = JSON.parse(body_text)
    Basecamp::Http.normalize_person_ids(parsed)
    parsed
  end

  # Must cover every live operation in conformance/tests/live-my-surface.json.
  # coverage_gate enforces that at replay time, but replay only runs during a
  # live canary — which skips whenever the canary secrets are unset, so this
  # map silently lost twenty operations for the length of #553. The static
  # guard scripts/check-replay-decoder-parity now compares it against the
  # fixture on every `make check` and CI run.
  DECODERS = {
    "ListProjects"                 => SDK_DECODE,
    "ListRecentProjects"           => SDK_DECODE,
    "GetProject"                   => SDK_DECODE,
    "GetMyAssignments"             => SDK_DECODE,
    "GetMyCompletedAssignments"    => SDK_DECODE,
    "GetMyDueAssignments"          => SDK_DECODE,
    "GetMyNotifications"           => SDK_DECODE,
    "GetMyProfile"                 => SDK_DECODE,
    "GetTodoset"                   => SDK_DECODE,
    "ListTodolists"                => SDK_DECODE,
    "ListTodos"                    => SDK_DECODE,
    "GetCalendar"                  => SDK_DECODE,
    "GetProgressReport"            => SDK_DECODE,
    "GetBubbleUps"                 => SDK_DECODE,
    "ListRecordings"               => SDK_DECODE,
    "Search"                       => SDK_DECODE,
    # The sixteen Everything aggregates.
    "GetEverythingMessages"        => SDK_DECODE,
    "GetEverythingComments"        => SDK_DECODE,
    "GetEverythingCheckins"        => SDK_DECODE,
    "GetEverythingFiles"           => SDK_DECODE,
    "GetEverythingForwards"        => SDK_DECODE,
    "GetEverythingOpenTodos"       => SDK_DECODE,
    "GetEverythingCompletedTodos"  => SDK_DECODE,
    "GetEverythingOverdueTodos"    => SDK_DECODE,
    "GetEverythingUnassignedTodos" => SDK_DECODE,
    "GetEverythingNoDueDateTodos"  => SDK_DECODE,
    "GetEverythingOpenCards"       => SDK_DECODE,
    "GetEverythingCompletedCards"  => SDK_DECODE,
    "GetEverythingOverdueCards"    => SDK_DECODE,
    "GetEverythingUnassignedCards" => SDK_DECODE,
    "GetEverythingNoDueDateCards"  => SDK_DECODE,
    "GetEverythingNotNowCards"     => SDK_DECODE,
  }.freeze

  def initialize(replay_dir, backend, fixture_path, openapi_path)
    @replay_dir = replay_dir
    @backend = backend
    @fixture_path = fixture_path
    @walker = Basecamp::Conformance::SchemaWalker.new(openapi_path)
    # Fixture and snapshot reads are pinned to UTF-8 regardless of process
    # locale (LC_ALL=C would otherwise read as US-ASCII).
    @fixture = JSON.parse(File.read(fixture_path, encoding: "UTF-8")).select { |t| t["mode"] == "live" }
  end

  def run
    fail_messages = coverage_gate
    if fail_messages.any?
      warn fail_messages.join("\n")
      return 1
    end

    out_dir = File.join(@replay_dir, @backend, "decode", "ruby")
    FileUtils.mkdir_p(out_dir)

    failures = 0
    @fixture.each do |test|
      snapshot = read_snapshot(test["name"])
      result = \
        if snapshot["skipped"] == true
          # Nothing to decode; record the skip explicitly so downstream
          # consumers see a marker rather than a missing decode result.
          puts "skip #{snapshot["operation"]}: #{snapshot["skip_reason"]}"
          { schema_version: SCHEMA_VERSION, operation: snapshot["operation"], pages: [],
            skipped: true, skip_reason: snapshot["skip_reason"].to_s }
        else
          decode_snapshot(snapshot)
        end
      # Unpinned on purpose, like the manifest write in runner.rb — File.write
      # emits the string's bytes and only transcodes when handed an explicit
      # encoding:. See that comment for the read/write asymmetry.
      File.write(File.join(out_dir, "#{safe_name(test["name"])}.json"), JSON.pretty_generate(result))
      failures += 1 if result[:pages].any? { |p| !p[:decoded] || p[:missing_required].any? }
    end

    failures.zero? ? 0 : 1
  end

  private

  def coverage_gate
    msgs = []
    fixture_ops = @fixture.map { |t| t["operation"] }.uniq

    # 1. Decoder coverage: every fixture operation has a decoder.
    missing_decoders = fixture_ops.reject { |op| DECODERS.key?(op) }
    if missing_decoders.any?
      msgs << "Ruby replay runner missing decoders for: #{missing_decoders.join(", ")}. " \
              "Add to DECODERS in replay-runner.rb."
    end

    # 2. Snapshot completeness: every fixture op has a snapshot file.
    wire_dir = File.join(@replay_dir, @backend, "wire")
    @fixture.each do |t|
      f = File.join(wire_dir, "#{safe_name(t["name"])}.json")
      next if File.exist?(f)

      # A deliberately skipped test still leaves a skip-marker snapshot
      # (see live-runner.test.ts persistSkipMarker), so a genuinely missing
      # file always means the capture didn't run.
      msgs << "Snapshot missing for operation #{t["operation"]} (test #{t["name"]}); " \
              "expected at #{f}. Re-run TS live capture or check skip status."
    end

    # 3. Snapshot recognition: every snapshot's operation is in the fixture.
    if Dir.exist?(wire_dir)
      Dir.glob(File.join(wire_dir, "*.json")).each do |f|
        snap = begin
          JSON.parse(File.read(f, encoding: "UTF-8"))
        rescue Errno::ENOENT, IOError, SystemCallError => e
          msgs << "Snapshot #{File.basename(f)} could not be read: #{e.class}: #{e.message}."
          next
        rescue JSON::ParserError => e
          msgs << "Snapshot #{File.basename(f)} is not valid JSON: #{e.message}."
          next
        end

        op = snap["operation"]
        if op.nil?
          msgs << "Snapshot #{File.basename(f)} is missing the top-level `operation` field. " \
                  "Re-run the TS live canary; pre-PR3 snapshots are no longer supported."
          next
        end

        unless fixture_ops.include?(op)
          msgs << "Unknown operation #{op.inspect} in snapshot #{File.basename(f)}; " \
                  "TS dispatch table appears to have drifted from live-my-surface.json."
        end

        # Skip markers (written by the TS runner when a live test skips
        # before wire capture) legitimately carry zero pages — but ONLY
        # zero pages.
        if snap["skipped"] == true
          pages_ok = snap["pages"].nil? || (snap["pages"].is_a?(Array) && snap["pages"].empty?)
          unless pages_ok && snap["pages_count"] == 0
            msgs << "Snapshot #{File.basename(f)} is marked skipped but carries pages; " \
                    "a skip marker must be empty."
          end
          next
        end

        # A snapshot like `{"operation": "GetProject"}` would pass the gates
        # above and then crash decode_snapshot's `snapshot["pages"].map` with
        # NoMethodError. Mirror the Go runner's read-time checks: require a
        # non-empty `pages` array and a matching `pages_count` so the gate
        # fails fast with a deterministic message.
        pages = snap["pages"]
        unless pages.is_a?(Array) && !pages.empty?
          msgs << "Snapshot #{File.basename(f)} has no pages; expected at least one wire response."
          next
        end

        unless snap["pages_count"] == pages.length
          msgs << "Snapshot #{File.basename(f)} pages_count (#{snap["pages_count"].inspect}) " \
                  "does not match len(pages) (#{pages.length})."
        end
      end
    end

    msgs
  end

  def read_snapshot(test_name)
    path = File.join(@replay_dir, @backend, "wire", "#{safe_name(test_name)}.json")
    JSON.parse(File.read(path, encoding: "UTF-8"))
  end

  def decode_snapshot(snapshot)
    operation = snapshot["operation"]
    decoder = DECODERS[operation]
    schema = @walker.find_response_schema(operation)

    pages = snapshot["pages"].map do |page|
      decode_page(page, operation, decoder, schema)
    end

    { schema_version: SCHEMA_VERSION, operation: operation, pages: pages }
  end

  def decode_page(page, operation, decoder, schema)
    body_text = page["bodyText"] || JSON.generate(page["body"])
    decoded = false
    decode_error = nil

    begin
      decoder.call(body_text)
      decoded = true
    rescue StandardError => e
      decode_error = "#{e.class}: #{e.message}"
    end

    missing_required = []
    extras_seen = []

    if schema
      # Per the TS validator: walk against parsed JSON, not the SDK-decoded
      # structure (decoders may drop unknown fields). A page whose body was
      # not parseable JSON gets empty arrays here — the decoded:false above
      # already captures the failure.
      body = page["body"]
      body = (JSON.parse(body_text) rescue nil) if body.is_a?(String) || body.nil?
      if body.is_a?(Hash) || body.is_a?(Array)
        missing_required = @walker.missing_required(body, schema)
        extras_seen = @walker.extras_seen(body, schema)
      end
    end

    {
      decoded: decoded,
      decode_error: decode_error,
      missing_required: missing_required,
      extras_seen: extras_seen,
    }
  end

  def safe_name(name)
    name.gsub(/[^a-z0-9_-]+/i, "_")
  end
end

if __FILE__ == $PROGRAM_NAME
  replay_dir = ENV["WIRE_REPLAY_DIR"]
  backend = ENV["BASECAMP_BACKEND"]
  abort "WIRE_REPLAY_DIR is required" if replay_dir.nil? || replay_dir.empty?
  abort "BASECAMP_BACKEND is required" if backend.nil? || backend.empty?

  fixture_path = File.expand_path("../../tests/live-my-surface.json", __dir__)
  openapi_path = File.expand_path("../../../openapi.json", __dir__)

  exit ReplayRunner.new(replay_dir, backend, fixture_path, openapi_path).run
end
