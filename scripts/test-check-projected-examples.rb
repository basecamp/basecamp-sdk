#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-projected-examples.rb.
#
# The gate's own `make check` run only exercises the VALID openapi.json, so
# nothing there proves it rejects anything. This test crafts each way a published
# example can contradict its projected schema and asserts the gate rejects it
# (non-zero exit + an expected message fragment), driving the checker through its
# PROJECTED_EXAMPLES_OPENAPI env override against tmp specs. It also confirms the
# real openapi.json still passes (positive control).
#
# Every case mutates a DEEP COPY of the real openapi.json — one mutation each, in
# a tmp file. The tracked spec is never written to. Mutating the real document
# rather than inventing a synthetic one keeps every case anchored to schemas this
# SDK actually ships: case 1 reproduces the #637 defect exactly (`color` in
# `Todolist.required` via `jsonAdd`, absent from both `GetTodolistOrGroup`
# examples) against today's schema.
#
# PROJECTED_EXAMPLES_CHECKER names the checker under test (default
# scripts/check-projected-examples.rb), so a deliberately-mutated copy can be
# driven through the same suite to prove the suite is not vacuous:
#
#   mkdir -p /tmp/m/scripts && cp scripts/check-projected-examples.rb \
#     scripts/schema_instance_validator.rb /tmp/m/scripts/   # mutate the COPY
#   PROJECTED_EXAMPLES_CHECKER=/tmp/m/scripts/check-projected-examples.rb \
#     ruby scripts/test-check-projected-examples.rb
#
# The copy needs scripts/schema_instance_validator.rb beside it (require_relative).
# Never mutate the tracked file.
#
# Every case below is pinned by at least one guard, and every guard by at least
# one case. These are the MEASURED results of driving mutated copies through this
# suite, not a prediction — each row is one guard removed, and the cases listed
# are exactly the ones that went red (a case goes red either by passing when it
# should fail, or by failing without its expected message):
#
#   guard removed in the copy                  cases that go red
#   ----------------------------------------   ------------------------------
#   response-example walk                      control, 1, 2, 3, 4, 7, 8, 9,
#                                              11, 12, 13, 14, 15a, 16
#   request-body-example walk                  5
#   parameter-example walk                     6, 15b
#   liveness floor (>= 1 response example)     10
#   components/examples `$ref` resolution      11
#   `externalValue` skip                       12
#   value-less Example Object rejection        13
#   Example Object type check                  14
#   triage keeps :error distinct from :skip    11, 13, 14
#   instance_errors result discarded           1, 2, 3, 4, 5, 6, 7, 8, 9,
#                                              15a, 15b
#   root-null check (schema_instance_validator) 15a, 15b
#   ...and its nullability test, i.e. a version
#   that bans EVERY root null                  16
#
# `instance_errors result discarded` is the row that matters most: it unwires the
# validator itself while leaving every walk in place, so a gate that visits all
# 37 examples and concludes nothing still goes red. Coverage and judgement are
# pinned separately.
#
# The last two rows are one guard and its over-correction, pinned in both
# directions on purpose. Root nulls were the third hole a reviewer found in this
# gate, and all three were the same shape: A PATH THAT RETURNS SUCCESS WITHOUT
# EXAMINING THE THING. The cheapest way to close it — reject every root null —
# would pass 15a/15b and start rejecting legitimate examples for
# required-and-nullable shapes, which is the class `Todolist.color` belongs to.
# So 16 pins that the check consults nullability rather than banning nulls.
#
# Note what is NOT in this table any more. Until #644 the projection published
# response examples still carrying the Smithy output wrapper their schema no
# longer had, and this gate unwrapped them — machinery with its own guards
# (multi-key wrapper, non-object wrapper, empty-wrapper skip) and its own cases.
# #648's `BareResponseExampleMapper` unwraps them in the projection instead, so
# all of that is deleted rather than left dead. Case 7 is what replaced it: put
# the wrapper back and the gate goes red, which makes this the regression guard
# for that mapper.
#
# Run directly (`ruby scripts/test-check-projected-examples.rb`) or via
# `make check-projected-examples` (which runs it after the live check).

require "json"
require "tmpdir"
require "open3"

# Per-case lines go to stdout and the failure report to stderr. Unsynced, stdout
# block-buffers when redirected to a file and the report lands ahead of the cases
# it summarizes — which is exactly when someone is reading the log.
$stdout.sync = true

ROOT = File.expand_path("..", __dir__)
CHECKER = File.expand_path(
  ENV.fetch("PROJECTED_EXAMPLES_CHECKER", "scripts/check-projected-examples.rb"), ROOT
)
REAL_OPENAPI = File.join(ROOT, "openapi.json")

TODOLIST_PATH = "/{accountId}/todolists/{id}"
SUBSCRIPTION_PATH = "/{accountId}/recordings/{recordingId}/subscription.json"
RECORDINGS_PATH = "/{accountId}/projects/recordings.json"

def read_utf8(path) = File.read(path, encoding: "UTF-8")

# Run the checker against a given spec; returns [combined_output, status].
#
# The captured bytes are tagged UTF-8 explicitly. Under LC_ALL=C (which CI uses,
# to prove the readers stay pinned) Open3 hands back a US-ASCII-tagged string,
# and comparing it against a UTF-8 fragment from this file — the messages carry
# em dashes — raises Encoding::CompatibilityError instead of reporting a case.
def run_checker(openapi:)
  out, status = Open3.capture2e({ "PROJECTED_EXAMPLES_OPENAPI" => openapi }, "ruby", CHECKER)
  [out.dup.force_encoding("UTF-8"), status]
end

# The real spec, deep-copied, with exactly one mutation applied in the block.
# JSON round-tripping breaks every reference back to the parsed original, so a
# mutation cannot leak into a later case either.
def with_mutated_spec
  doc = JSON.parse(read_utf8(REAL_OPENAPI))
  yield doc
  Dir.mktmpdir("projected-examples-test") do |dir|
    tmp = File.join(dir, "openapi.json")
    File.write(tmp, JSON.generate(doc))
    run_checker(openapi: tmp)
  end
end

# Every case anchors on a real operation, so a spec change that moves one out
# from under this suite has to SAY so. Without this the cases would die on
# `NoMethodError: undefined method for nil`, which reads like a bug in the gate
# rather than a test that needs re-anchoring.
def anchor(doc, *path)
  node = doc.dig(*path)
  raise "self-test anchor #{path.inspect} no longer resolves in openapi.json — " \
        "the spec moved and this case needs re-anchoring, the gate is not at fault" if node.nil?

  node
end

# The GetTodolistOrGroup 200 response examples map (the shape the #637 defect
# lived in): name => example object, each with a `{"result": {...}}` value.
def todolist_response_examples(doc)
  anchor(doc, "paths", TODOLIST_PATH, "get", "responses", "200", "content", "application/json", "examples")
end

# #648 unwraps the response examples in the projection, so the published value IS
# the Todolist — there is no `result` wrapper left to reach through.
def todolist_payload(doc, example)
  todolist_response_examples(doc).fetch(example).fetch("value")
end

failures = []

def expect_pass(failures, label, out, status)
  if status.success?
    puts "  PASS  #{label}"
  else
    puts "  FAIL  #{label}"
    failures << "#{label}: expected PASS but checker failed:\n#{out}"
  end
end

def expect_pass_containing(failures, label, out, status, fragment)
  if !status.success?
    puts "  FAIL  #{label}"
    failures << "#{label}: expected PASS but checker failed:\n#{out}"
  elsif !out.include?(fragment)
    puts "  FAIL  #{label}"
    failures << "#{label}: passed but message missing #{fragment.inspect}:\n#{out}"
  else
    puts "  PASS  #{label}"
  end
end

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    puts "  FAIL  #{label}"
    failures << "#{label}: expected FAILURE but checker passed:\n#{out}"
  elsif !out.include?(fragment)
    puts "  FAIL  #{label}"
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{out}"
  else
    puts "  PASS  #{label}"
  end
end

puts "==> projected-example self-test (checker: #{CHECKER.sub("#{ROOT}/", '')})"

# --- Positive control ----------------------------------------------------------
#
# First, and load-bearing twice over: if the real spec does not pass, every
# negative case below fails for a reason that has nothing to do with its
# mutation. It is also the only thing that pins the bare-response envelope
# unwrap — the real GetTodolistOrGroup examples carry a `result` wrapper the
# projected schema no longer has.

out, status = run_checker(openapi: REAL_OPENAPI)
expect_pass(failures, "positive control: the real openapi.json passes", out, status)

# --- 1. The #637 defect, exactly -- THE ONE THAT MATTERS -----------------------
#
# `Todolist.required` gains `color` through spec/smithy-build.json's
# `jsonAdd .../required/-`. `smithy validate` sees the pre-projection model,
# where `color` is optional, so it passes. Nothing else looked. Both published
# examples shipped without it.

out, status = with_mutated_spec do |doc|
  todolist_payload(doc, "GetTodolistOrGroup_example1").delete("color")
  todolist_payload(doc, "GetTodolistOrGroup_example2").delete("color")
end
expect_fail(failures, "1. #637: projection-added required field missing from both examples", out, status,
            "GetTodolistOrGroup_example1: missing required field `color`")

# --- 2. Required field present as null ------------------------------------------
#
# `comments_app_url` is required and `type: string` with no null union: absence
# and null are different wire facts and neither is legal here.

out, status = with_mutated_spec do |doc|
  todolist_payload(doc, "GetTodolistOrGroup_example1")["comments_app_url"] = nil
end
expect_fail(failures, "2. required field present as null against a non-nullable schema", out, status,
            "comments_app_url: required field is null but its schema is not nullable")

# --- 3. Nested type contradiction ------------------------------------------------
#
# Inside `creator`, which the example reaches only through a `$ref` to Person —
# so this also pins that the walk descends through refs rather than stopping at
# the top-level object.

out, status = with_mutated_spec do |doc|
  todolist_payload(doc, "GetTodolistOrGroup_example1")["creator"]["id"] = "1"
end
expect_fail(failures, "3. nested type contradiction behind a $ref: creator/id string for integer", out, status,
            "creator/id: expected integer, got string")

# --- 4. Nested missing required --------------------------------------------------

out, status = with_mutated_spec do |doc|
  todolist_payload(doc, "GetTodolistOrGroup_example2")["creator"].delete("name")
end
expect_fail(failures, "4. nested missing required field: creator/name", out, status,
            "missing required field `creator/name`")

# --- 5. Request-body example ------------------------------------------------------
#
# Response examples are where the projection seam lives, but request bodies are
# published too and are covered. An element-level fault also pins array descent.

out, status = with_mutated_spec do |doc|
  anchor(doc, "paths", SUBSCRIPTION_PATH, "put", "requestBody", "content", "application/json",
         "examples", "UpdateSubscription_example1")["value"]["subscriptions"] = [111, "222"]
end
expect_fail(failures, "5. request-body example: string in an array of integers", out, status,
            "subscriptions[1]: expected integer, got string")

# --- 6. Parameter example ----------------------------------------------------------
#
# `accountId` is a numeric STRING. A bare integer in the example would document a
# request the SDK cannot send.

out, status = with_mutated_spec do |doc|
  anchor(doc, "paths", RECORDINGS_PATH, "get", "parameters")
    .find { |p| p["name"] == "accountId" }["examples"]["ListRecordings_example1"]["value"] = 999
end
expect_fail(failures, "6. parameter example: integer for a numeric-string parameter", out, status,
            "parameters/accountId/examples/ListRecordings_example1: (root): expected string, got integer")

# --- 7. The wrapper #648's mapper removes must not come back ---------------------
#
# Before #648 the bare-response mappers rewrote the SCHEMA into the bare payload
# and left the wrapper on the example, so every published response example
# contradicted its own schema. `BareResponseExampleMapper` mirrors the unwrapping
# onto examples; this case is the regression guard for it. Re-wrap one example
# and every required field of the bare shape goes missing at once — including
# `color`, the field this whole gate exists for.
#
# This is why the gate does NOT unwrap anything itself. A gate that also accepted
# the wrapped shape would be silent here, and a regression in that mapper would
# ship exactly the way #637 did.

out, status = with_mutated_spec do |doc|
  ex = todolist_response_examples(doc).fetch("GetTodolistOrGroup_example1")
  ex["value"] = { "result" => ex["value"] }
end
expect_fail(failures, "7. re-wrapped response example (BareResponseExampleMapper regression)", out, status,
            "GetTodolistOrGroup_example1: missing required field `color`")

# --- 8. A response example that is not the shape its schema declares --------------
#
# `ListRecordings` publishes an ARRAY of Recording. An object there is the
# category of fault the old wrapper was, stated as what it actually is: a type
# contradiction against the published schema.

out, status = with_mutated_spec do |doc|
  anchor(doc, "paths", RECORDINGS_PATH, "get", "responses", "200", "content", "application/json",
         "examples", "ListRecordings_example1")["value"] = { "not" => "an array" }
end
expect_fail(failures, "8. array-schema response example given an object", out, status,
            "ListRecordings_example1: (root): expected array, got object")

# --- 9. Every operation's response examples are reached, not just one -------------
#
# Four operations publish response examples and #648 gave all ten a real payload.
# A walk that reached only the first would still pass cases 1-4, so fault an
# example on an operation none of those touch.

out, status = with_mutated_spec do |doc|
  anchor(doc, "paths", SUBSCRIPTION_PATH, "put", "responses", "200", "content", "application/json",
         "examples", "UpdateSubscription_example2")["value"].delete("count")
end
expect_fail(failures, "9. response example on a fourth operation is reached", out, status,
            "UpdateSubscription_example2: missing required field `count`")

# --- 10. Liveness floor ----------------------------------------------------------------
#
# Strip every response example in the document. The gate then validates none of
# the class the seam lives in — and would otherwise pass, because request-body
# and parameter examples still count. A gate that no-ops is the failure mode this
# gate exists to prevent.

out, status = with_mutated_spec do |doc|
  doc.fetch("paths").each_value do |path_item|
    next unless path_item.is_a?(Hash)

    path_item.each_value do |op|
      next unless op.is_a?(Hash) && op["responses"].is_a?(Hash)

      op["responses"].each_value do |response|
        next unless response.is_a?(Hash) && response["content"].is_a?(Hash)

        response["content"].each_value { |m| m.delete("examples") if m.is_a?(Hash) }
      end
    end
  end
end
expect_fail(failures, "10. vacuous run: no response example validated", out, status,
            "no response example was validated")

# --- 11. Unresolvable example $ref -------------------------------------------------------
#
# An example given as a `$ref` into components/examples that does not resolve is
# an unchecked example, not an absent one.

out, status = with_mutated_spec do |doc|
  todolist_response_examples(doc)["GetTodolistOrGroup_example1"] =
    { "$ref" => "#/components/examples/NoSuchExample" }
end
expect_fail(failures, "11. example $ref that resolves to nothing", out, status,
            "`$ref` does not resolve to an entry in components/examples")

# --- 12. externalValue -------------------------------------------------------------------
#
# A remote example is reported as skipped, by name, rather than fetched: an
# offline gate that reaches the network is a gate that skips when the network is
# down. example2 still carries a payload, so the liveness floor is satisfied and
# the run stays green — the point is that the skip is VISIBLE.

out, status = with_mutated_spec do |doc|
  todolist_response_examples(doc)["GetTodolistOrGroup_example1"] =
    { "externalValue" => "https://example.invalid/todolist.json" }
end
expect_pass_containing(failures, "12. externalValue example is skipped by name, not fetched", out, status,
                       "`externalValue` URL, which this gate does not fetch")

# --- 13. Example Object with no value at all ------------------------------------------------
#
# Neither `value` nor `externalValue`: there is nothing to check and nothing that
# says so, which is the one thing a skip must never be confused with.

out, status = with_mutated_spec do |doc|
  todolist_response_examples(doc)["GetTodolistOrGroup_example1"] = { "summary" => "no value here" }
end
expect_fail(failures, "13. Example Object declaring neither value nor externalValue", out, status,
            "Example Object declares neither `value` nor `externalValue`")

# --- 14. Not an Example Object at all ---------------------------------------------------------
#
# An `examples` map entry is an Example Object, never a bare value. Accepting a
# bare value here is what would let the singular-`example` reading leak into the
# plural spelling and start unwrapping a payload field named `value`.

out, status = with_mutated_spec do |doc|
  todolist_response_examples(doc)["GetTodolistOrGroup_example1"] = "just a string"
end
expect_fail(failures, "14. examples map entry that is not an Example Object", out, status,
            "example is not an Example Object")

# --- 15. Root null against a non-nullable schema ---------------------------------
#
# `instance_errors` tolerates a null because the ENCLOSING context has already
# judged it — a required-but-null field is caught by the required loop in its
# parent, a null element by the items check in its array. A root value has no
# enclosing context, so before the fix this returned no errors and the example
# was COUNTED AS VALIDATED: the gate approving exactly the contradiction it
# exists to catch. Both a response and a parameter example, because the early
# return sat below every kind of caller alike.

out, status = with_mutated_spec do |doc|
  todolist_response_examples(doc).fetch("GetTodolistOrGroup_example1")["value"] = nil
end
expect_fail(failures, "15a. response example value:null against a non-nullable schema", out, status,
            "GetTodolistOrGroup_example1: (root): value is null but the schema is not nullable")

out, status = with_mutated_spec do |doc|
  anchor(doc, "paths", RECORDINGS_PATH, "get", "parameters")
    .find { |p| p["name"] == "accountId" }["examples"]["ListRecordings_example1"]["value"] = nil
end
expect_fail(failures, "15b. parameter example value:null against a non-nullable schema", out, status,
            "parameters/accountId/examples/ListRecordings_example1: (root): value is null but the schema is not nullable")

# --- 16. Root null against a NULLABLE schema is still fine -------------------------
#
# The guard for case 15 has to check nullability, not ban root nulls. Without
# this case the cheapest way to pass 15 — reject every null — would look correct
# and would start failing legitimate examples the moment the spec publishes one
# for a required-and-nullable shape, which is the exact class `Todolist.color`
# belongs to.

out, status = with_mutated_spec do |doc|
  param = anchor(doc, "paths", RECORDINGS_PATH, "get", "parameters").find { |p| p["name"] == "accountId" }
  param["schema"] = { "type" => %w[string null] }
  param["examples"]["ListRecordings_example1"]["value"] = nil
end
expect_pass(failures, "16. root null IS allowed where the schema permits null", out, status)

# --- Report --------------------------------------------------------------------

if failures.empty?
  puts "==> projected-example self-test passed — 2 positive + 16 negative/skip cases"
  exit 0
else
  warn "projected-example self-test FAILED:"
  failures.each { |f| warn "  - #{f}" }
  exit 1
end
