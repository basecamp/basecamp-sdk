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
    return nil if body_text.nil? || body_text.empty?

    parsed = JSON.parse(body_text)
    Basecamp::Http.normalize_person_ids(parsed)
    parsed
  end

  DECODERS = {
    "ListProjects"              => SDK_DECODE,
    "GetProject"                => SDK_DECODE,
    "GetMyAssignments"          => SDK_DECODE,
    "GetMyCompletedAssignments" => SDK_DECODE,
    "GetMyDueAssignments"       => SDK_DECODE,
    "GetMyNotifications"        => SDK_DECODE,
    "GetMyProfile"              => SDK_DECODE,
    "GetTodoset"                => SDK_DECODE,
    "ListTodolists"             => SDK_DECODE,
    "ListTodos"                 => SDK_DECODE,
  }.freeze

  def initialize(replay_dir, backend, fixture_path, openapi_path)
    @replay_dir = replay_dir
    @backend = backend
    @fixture_path = fixture_path
    @walker = Basecamp::Conformance::SchemaWalker.new(openapi_path)
    @fixture = JSON.parse(File.read(fixture_path)).select { |t| t["mode"] == "live" }
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
      result = decode_snapshot(snapshot)
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

      # Per-test skipReason files are not part of the PR2 contract today;
      # treat missing snapshots as runner failure with a clear pointer.
      msgs << "Snapshot missing for operation #{t["operation"]} (test #{t["name"]}); " \
              "expected at #{f}. Re-run TS live capture or check skip status."
    end

    # 3. Snapshot recognition: every snapshot's operation is in the fixture.
    if Dir.exist?(wire_dir)
      Dir.glob(File.join(wire_dir, "*.json")).each do |f|
        snap = JSON.parse(File.read(f))
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
      end
    end

    msgs
  end

  def read_snapshot(test_name)
    path = File.join(@replay_dir, @backend, "wire", "#{safe_name(test_name)}.json")
    JSON.parse(File.read(path))
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
