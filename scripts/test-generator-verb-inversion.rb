#!/usr/bin/env ruby
# frozen_string_literal: true

# Self-test: an operation on a verb the SDKs do not generate can never vanish.
#
# WHAT THIS PINS. Every per-language generator used to find operations by
# iterating its own hard-coded list of five HTTP verbs. Smithy's `@http` trait
# takes the method as a free-form string it "will use literally and will perform
# no validation on", so `method: "HEAD"` in spec/basecamp.smithy produced a valid
# model, a valid openapi.json, and NO METHOD ON ANY CLIENT in six languages — no
# error, no warning, nothing (#925). The generators now read a Path Item Object
# by EXCLUSION (its non-operation fields are a closed, spec-defined set; its
# extensions are `x-` prefixed; every other field is an operation) and an
# operation they cannot emit stops the run BY NAME.
#
# WHY IT IS A TEST RATHER THAN A PARAGRAPH. The live run only ever sees
# openapi.json, which declares four verbs and no non-operation path-item fields,
# so it exercises the passing case alone. The mutation is what states the case:
# restore the walks below to the ORIGINAL five-verb list and 13 of these 38 cases
# fail; restore them to the EIGHT verbs OpenAPI 3.1 names and 6 still fail —
# OpenAPI 3.2's `query` and `additionalOperations`, and the cases where a field
# that is neither a known non-operation field nor a readable operation is stepped
# over in silence. A longer list repairs the verbs it was extended with and
# nothing else; that gap is the whole argument for identifying an operation by
# what it is NOT, so it is a test rather than a paragraph.
#
# WHAT IT DRIVES. Ruby and Python, both service and metadata generators, plus the
# jq route generator — everything that runs on the stdlib toolchains the Spec
# Gates job already has. Kotlin, Swift, Rust and TypeScript carry the identical
# walk and bound but need a JVM, a Swift toolchain, cargo, or an npm install;
# scripts/test-compiled-generator-refusal drives those four real binaries in
# their own language jobs, since their drift checks read a spec that carries only
# emitted verbs and so can never see them regress.
#
# Stdlib only. Wired into `make check` and the spec-gates CI job.

require "json"
require "fileutils"
require "tmpdir"

ROOT = File.expand_path("..", __dir__)

FAILURES = []
PASSES = []

# Cases are numbered as they RUN rather than in their names: hand-numbering was
# already wrong twice after cases were inserted, and a duplicated number makes
# the mutation counts below unreadable.
CASE = Struct.new(:n)
COUNTER = CASE.new(0)

def check(name)
  COUNTER.n += 1
  label = "#{COUNTER.n}. #{name}"
  ok, detail = yield
  if ok
    PASSES << label
    puts "  PASS  #{label}"
  else
    FAILURES << "#{label}: #{detail}"
    puts "  FAIL  #{label} — #{detail}"
  end
rescue StandardError => e
  FAILURES << "#{label}: raised #{e.class}: #{e.message}"
  puts "  FAIL  #{label} — raised #{e.class}: #{e.message}"
end

# A minimal but real spec: account-scoped path, one tagged operation per verb
# named, request-free, with a 200 JSON response the generators can render.
def spec(path_items, verbs_declared: nil)
  doc = {
    "openapi" => "3.1.0",
    "info" => { "title" => "Basecamp", "version" => "1.0" },
    "paths" => path_items,
    "components" => { "schemas" => {
      "Widget" => { "type" => "object", "properties" => { "id" => { "type" => "integer", "format" => "int64" } } }
    } }
  }
  doc["x-verbs-declared"] = verbs_declared if verbs_declared
  doc
end

def operation(id, tag: "Widgets")
  {
    "operationId" => id,
    "tags" => [tag],
    "description" => "#{id} description",
    "x-basecamp-retry" => { "max" => 3, "base_delay_ms" => 1000, "backoff" => "exponential", "retry_on" => [429] },
    "responses" => { "200" => { "description" => "ok", "content" => {
      "application/json" => { "schema" => { "$ref" => "#/components/schemas/Widget" } }
    } } }
  }
end

def behavior(ids)
  { "operations" => ids.to_h { |id| [id, { "idempotent" => true }] } }
end

def with_files(spec_doc, behavior_doc = nil, verbs: nil)
  Dir.mktmpdir("verb-inversion") do |dir|
    spec_path = File.join(dir, "openapi.json")
    File.write(spec_path, JSON.pretty_generate(spec_doc))
    behavior_path = File.join(dir, "behavior-model.json")
    File.write(behavior_path, JSON.pretty_generate(behavior_doc || { "operations" => {} }))
    verbs_path = nil
    if verbs
      verbs_path = File.join(dir, "generated-verbs.json")
      File.write(verbs_path, JSON.pretty_generate({ "verbs" => verbs }))
    end
    yield dir, spec_path, behavior_path, verbs_path
  end
end

def run(command, env = {})
  output = IO.popen(env, command, err: %i[child out], &:read)
  [$?.exitstatus, output]
end

# --------------------------------------------------------------- Ruby services

def ruby_services(spec_doc, verbs: nil)
  with_files(spec_doc, nil, verbs: verbs) do |dir, spec_path, _behavior, verbs_path|
    out_dir = File.join(dir, "services")
    env = verbs_path ? { "BASECAMP_GENERATED_VERBS" => verbs_path } : {}
    status, output = run(["ruby", File.join(ROOT, "ruby/scripts/generate-services.rb"),
                          "--openapi", spec_path, "--output", out_dir], env)
    emitted = Dir.exist?(out_dir) ? Dir.children(out_dir).sort : []
    bodies = emitted.map { |f| File.read(File.join(out_dir, f)) }.join
    [status, output, emitted, bodies]
  end
end

puts "==> generator verb inversion self-test"
puts
puts "Ruby service generator"

check("a GET alone still generates") do
  status, output, emitted, = ruby_services(spec({ "/{accountId}/widgets.json" => { "get" => operation("ListWidgets") } }))
  [status.zero? && !emitted.empty?, "exit #{status}, files #{emitted.inspect}: #{output}"]
end

check("a HEAD operation stops the run and names it") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => { "head" => operation("HeadWidgets") } }))
  [status != 0 && output.include?("HEAD /{accountId}/widgets.json") && output.include?("HeadWidgets"),
   "exit #{status}: #{output}"]
end

check("a HEAD beside a GET stops the run rather than silently emitting one of two") do
  status, output, _emitted, bodies = ruby_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"), "head" => operation("HeadWidgets")
  } }))
  [status != 0 && !bodies.include?("list_widgets"), "exit #{status}, body had list_widgets: #{output}"]
end

check("OpenAPI 3.2's `query` verb stops the run") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => { "query" => operation("QueryWidgets") } }))
  [status != 0 && output.include?("QUERY /{accountId}/widgets.json"), "exit #{status}: #{output}"]
end

check("non-operation path-item fields are skipped, not treated as operations") do
  status, output, _emitted, bodies = ruby_services(spec({ "/{accountId}/widgets.json" => {
    "summary" => "Widgets", "description" => "The widgets path", "servers" => [], "parameters" => [],
    "x-basecamp-note" => { "anything" => true },
    "get" => operation("ListWidgets")
  } }))
  [status.zero? && bodies.include?("list_widgets"), "exit #{status}: #{output}"]
end

check("a field that is neither known nor an operation object is named, not skipped") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"), "frobnicate" => "not an object"
  } }))
  [status != 0 && output.include?("frobnicate"), "exit #{status}: #{output}"]
end

check("a $ref path item is refused rather than read as empty") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => { "$ref" => "#/components/pathItems/Widgets" } }))
  [status != 0 && output.include?("$ref"), "exit #{status}: #{output}"]
end

check("an operation with no operationId is refused rather than skipped") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets").tap { |op| op.delete("operationId") }
  } }))
  [status != 0 && output.include?("operationId"), "exit #{status}: #{output}"]
end

check("emission order follows the declaration, not the document") do
  _status, _output, _emitted, bodies = ruby_services(spec({ "/{accountId}/widgets/{widgetId}.json" => {
    "delete" => operation("DeleteWidget"), "get" => operation("GetWidget")
  } }))
  at_get = bodies.index("def get_widget")
  at_delete = bodies.index("def delete_widget")
  [!at_get.nil? && !at_delete.nil? && at_get < at_delete, "get at #{at_get.inspect}, delete at #{at_delete.inspect}"]
end

check("the bound is read from spec/generated-verbs.json, not a private literal") do
  status, output, = ruby_services(
    spec({ "/{accountId}/widgets/{widgetId}.json" => { "delete" => operation("DeleteWidget") } }),
    verbs: %w[get post put patch]
  )
  [status != 0 && output.include?("DELETE /{accountId}/widgets/{widgetId}.json"), "exit #{status}: #{output}"]
end

check("OpenAPI 3.2's `additionalOperations` map is refused by name, not read as one operation") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"),
    "additionalOperations" => { "PURGE" => operation("PurgeWidgets") }
  } }))
  [status != 0 && output.include?("additionalOperations"), "exit #{status}: #{output}"]
end

# Every loader has to read the declaration the SAME way. A file one generator
# accepts and another refuses is the cross-SDK divergence a shared declaration
# exists to prevent, so the bad shapes are asserted against Ruby AND Python.
# The shapes are the ones two review rounds produced plus the ones the next
# round would have: a verb is `[a-z]+`, so none of these is a question about
# any language's definition of whitespace.
[["a non-string entry", ["get", 1]],
 ["an ASCII-blank entry", ["get", "  "]],
 ["a Unicode-blank entry (U+00A0, which Ruby's strip keeps)", ["get", "\u00a0"]],
 ["a Unicode-blank entry (U+2003, which Ruby's strip keeps and Python's removes)", ["get", "\u2003"]],
 ["an entry with a leading BOM", ["get", "\ufeffdelete"]],
 ["an uppercase entry", ["get", "DELETE"]],
 ["an empty array", []]].each_with_index do |(what, verbs), i|
  check("the declaration rejects #{what}, in Ruby and in Python") do
    results = {}
    with_files(spec({ "/{accountId}/widgets.json" => { "get" => operation("ListWidgets") } })) do |dir, spec_path, _b, _v|
      verbs_path = File.join(dir, "bad-verbs.json")
      File.write(verbs_path, JSON.generate({ "verbs" => verbs }))
      env = { "BASECAMP_GENERATED_VERBS" => verbs_path }
      results[:ruby] = run(["ruby", File.join(ROOT, "ruby/scripts/generate-services.rb"),
                            "--openapi", spec_path, "--output", File.join(dir, "rb")], env)
      results[:python] = run(["python3", File.join(ROOT, "python/scripts/generate_services.py"),
                              "--openapi", spec_path, "--output", File.join(dir, "py")],
                             env.merge("PYTHONDONTWRITEBYTECODE" => "1"))
    end
    ok = results.values.all? { |status, output| status != 0 && output.include?("verbs") }
    [ok, results.map { |lang, (status, output)| "#{lang} exit #{status}: #{output.lines.first}" }.join(" | ")]
  end
end

check("PATCH is refused — the declaration does not name it, and two runtimes cannot serve it") do
  status, output, = ruby_services(spec({ "/{accountId}/widgets/{widgetId}.json" => { "patch" => operation("PatchWidget") } }))
  [status != 0 && output.include?("PATCH /{accountId}/widgets/{widgetId}.json"), "exit #{status}: #{output}"]
end

puts
puts "Ruby metadata extractor (verb-agnostic: it must EXTRACT the unemitted verb, not skip it)"

check("a HEAD operation's retry metadata is extracted") do
  with_files(spec({ "/{accountId}/widgets.json" => { "head" => operation("HeadWidgets") } })) do |_dir, spec_path, _b, _v|
    status, output = run(["ruby", File.join(ROOT, "ruby/scripts/generate-metadata.rb"), spec_path])
    [status.zero? && output.include?("HeadWidgets"), "exit #{status}: #{output[0, 400]}"]
  end
end

check("`additionalOperations` is refused by the UNBOUNDED walker too") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"),
    "additionalOperations" => { "PURGE" => operation("PurgeWidgets") }
  } })) do |_dir, spec_path, _b, _v|
    status, output = run(["ruby", File.join(ROOT, "ruby/scripts/generate-metadata.rb"), spec_path])
    [status != 0 && output.include?("additionalOperations"), "exit #{status}: #{output[0, 400]}"]
  end
end

check("an unreadable path-item field is named rather than skipped") do
  with_files(spec({ "/{accountId}/widgets.json" => { "get" => operation("ListWidgets"), "frobnicate" => 7 } })) do |_dir, spec_path, _b, _v|
    status, output = run(["ruby", File.join(ROOT, "ruby/scripts/generate-metadata.rb"), spec_path])
    [status != 0 && output.include?("frobnicate"), "exit #{status}: #{output[0, 400]}"]
  end
end

puts
puts "Python service generator"

def python_services(spec_doc, verbs: nil)
  with_files(spec_doc, nil, verbs: verbs) do |dir, spec_path, _behavior, verbs_path|
    out_dir = File.join(dir, "services")
    env = { "PYTHONDONTWRITEBYTECODE" => "1" }
    env["BASECAMP_GENERATED_VERBS"] = verbs_path if verbs_path
    status, output = run(["python3", File.join(ROOT, "python/scripts/generate_services.py"),
                          "--openapi", spec_path, "--output", out_dir], env)
    emitted = Dir.exist?(out_dir) ? Dir.children(out_dir).sort : []
    bodies = emitted.map { |f| File.read(File.join(out_dir, f)) }.join
    [status, output, emitted, bodies]
  end
end

check("a GET alone still generates") do
  status, output, emitted, = python_services(spec({ "/{accountId}/widgets.json" => { "get" => operation("ListWidgets") } }))
  [status.zero? && !emitted.empty?, "exit #{status}, files #{emitted.inspect}: #{output}"]
end

check("a HEAD operation stops the run and names it") do
  status, output, = python_services(spec({ "/{accountId}/widgets.json" => { "head" => operation("HeadWidgets") } }))
  [status != 0 && output.include?("HEAD /{accountId}/widgets.json") && output.include?("HeadWidgets"),
   "exit #{status}: #{output}"]
end

check("a HEAD beside a GET stops the run rather than silently emitting one of two") do
  status, output, _emitted, bodies = python_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"), "head" => operation("HeadWidgets")
  } }))
  [status != 0 && !bodies.include?("def list_widgets"), "exit #{status}: #{output}"]
end

check("non-operation path-item fields are skipped, not treated as operations") do
  status, output, _emitted, bodies = python_services(spec({ "/{accountId}/widgets.json" => {
    "summary" => "Widgets", "parameters" => [], "x-basecamp-note" => { "anything" => true },
    "get" => operation("ListWidgets")
  } }))
  [status.zero? && bodies.include?("def list_widgets"), "exit #{status}: #{output}"]
end

check("a $ref path item is refused rather than read as empty") do
  status, output, = python_services(spec({ "/{accountId}/widgets.json" => { "$ref" => "#/components/pathItems/Widgets" } }))
  [status != 0 && output.include?("$ref"), "exit #{status}: #{output}"]
end

check("an operation with no operationId is refused rather than skipped") do
  status, output, = python_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets").tap { |op| op.delete("operationId") }
  } }))
  [status != 0 && output.include?("operationId"), "exit #{status}: #{output}"]
end

check("the bound is read from spec/generated-verbs.json, not a private literal") do
  status, output, = python_services(
    spec({ "/{accountId}/widgets/{widgetId}.json" => { "delete" => operation("DeleteWidget") } }),
    verbs: %w[get post put patch]
  )
  [status != 0 && output.include?("DELETE /{accountId}/widgets/{widgetId}.json"), "exit #{status}: #{output}"]
end

check("OpenAPI 3.2's `additionalOperations` map is refused by name") do
  status, output, = python_services(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"),
    "additionalOperations" => { "PURGE" => operation("PurgeWidgets") }
  } }))
  [status != 0 && output.include?("additionalOperations"), "exit #{status}: #{output}"]
end

puts
puts "Python metadata extractor (verb-agnostic)"

check("`additionalOperations` is refused by the UNBOUNDED walker too") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"),
    "additionalOperations" => { "PURGE" => operation("PurgeWidgets") }
  } }), behavior(%w[ListWidgets])) do |dir, spec_path, behavior_path, _v|
    out = File.join(dir, "metadata.json")
    status, output = run(["python3", File.join(ROOT, "python/scripts/generate_metadata.py"),
                          "--openapi", spec_path, "--behavior", behavior_path, "--output", out],
                         { "PYTHONDONTWRITEBYTECODE" => "1" })
    [status != 0 && output.include?("additionalOperations"), "exit #{status}: #{output[0, 400]}"]
  end
end

check("a HEAD operation's retry metadata is extracted") do
  with_files(spec({ "/{accountId}/widgets.json" => { "head" => operation("HeadWidgets") } }),
             behavior(%w[HeadWidgets])) do |dir, spec_path, behavior_path, _v|
    out = File.join(dir, "metadata.json")
    status, output = run(["python3", File.join(ROOT, "python/scripts/generate_metadata.py"),
                          "--openapi", spec_path, "--behavior", behavior_path, "--output", out])
    written = File.exist?(out) ? File.read(out) : ""
    [status.zero? && written.include?("HeadWidgets"), "exit #{status}: #{output} / #{written[0, 200]}"]
  end
end

puts
puts "Route generator (jq; verb-agnostic — a route's operations map is method -> operationId)"

check("a HEAD operation reaches url-routes.json rather than being dropped") do
  with_files(spec({ "/{accountId}/widgets.json" => { "head" => operation("HeadWidgets") } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    routes = File.exist?(out) ? JSON.parse(File.read(out))["routes"] : []
    [status.zero? && routes.dig(0, "operations", "HEAD") == "HeadWidgets", "exit #{status}: #{output} / #{routes.inspect}"]
  end
end

check("non-operation path-item fields do not become routes") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "summary" => "Widgets", "parameters" => [], "x-basecamp-note" => { "anything" => true },
    "get" => operation("ListWidgets")
  } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    ops = File.exist?(out) ? JSON.parse(File.read(out)).dig("routes", 0, "operations") : nil
    [status.zero? && ops == { "GET" => "ListWidgets" }, "exit #{status}: #{output} / #{ops.inspect}"]
  end
end

check("a $ref path item is refused rather than read as empty") do
  with_files(spec({ "/{accountId}/widgets.json" => { "$ref" => "#/components/pathItems/Widgets" } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    [status != 0 && output.include?("$ref"), "exit #{status}: #{output}"]
  end
end

check("OpenAPI 3.2's `additionalOperations` map is refused by name") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"),
    "additionalOperations" => { "PURGE" => operation("PurgeWidgets") }
  } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    [status != 0 && output.include?("additionalOperations"), "exit #{status}: #{output}"]
  end
end

check("a field with no operationId is named rather than reaching the route table unnamed") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"), "frobnicate" => { "responses" => {} }
  } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    [status != 0 && output.include?("frobnicate"), "exit #{status}: #{output}"]
  end
end

check("a scalar path-item field is named rather than erroring obliquely") do
  with_files(spec({ "/{accountId}/widgets.json" => {
    "get" => operation("ListWidgets"), "frobnicate" => "not an object"
  } })) do |dir, spec_path, _b, _v|
    out = File.join(dir, "url-routes.json")
    status, output = run([File.join(ROOT, "scripts/generate-url-routes"), spec_path, out])
    [status != 0 && output.include?("frobnicate"), "exit #{status}: #{output}"]
  end
end

puts
if FAILURES.empty?
  puts "==> generator verb inversion self-test: all #{PASSES.length} cases passed"
  exit 0
end
warn "==> generator verb inversion self-test: #{FAILURES.length} of #{PASSES.length + FAILURES.length} cases failed"
FAILURES.each { |f| warn "  - #{f}" }
exit 1
