# frozen_string_literal: true

require "set"

# Composition-aware structural validation of a JSON instance against a schema
# from the generated `openapi.json`.
#
# Extracted from scripts/check-fixture-coverage.rb so a second gate can reuse it
# rather than grow a parallel validator that drifts. Two callers today:
#
#   * check-fixture-coverage.rb  — fixture instances vs their declared schema
#   * check-projected-examples.rb — the spec's own published `examples` vs the
#     schema they sit under, which is where `jsonAdd`-injected `required` entries
#     first become checkable (Smithy validates `@examples` against the *Smithy*
#     model, before the projection adds anything).
#
# Both scripts are stdlib-only and read the same generated document, so the
# validator carries no I/O and no configuration — pass it parsed JSON.
#
# --- Scope: a deliberately PARTIAL structural validator ------------------------
#
# This is NOT a complete JSON-Schema / OpenAPI validator. It validates the subset
# that keeps an instance structurally faithful to the generated types:
#   * required-field presence
#   * declared types (with integer/number integrality) and nullability
#   * arrays and array-element typing/nullability
#   * $ref (incl. 3.1 $ref-with-siblings) and allOf conjunction (required unioned;
#     duplicate properties and array `items` conjoined; type constraints
#     intersected; nullability = every part permits null)
#   * anyOf/oneOf as AT-LEAST-ONE (the value must satisfy some branch; a group is
#     nullable only if a branch is)
#
# Intentionally NOT implemented (out of scope unless a case is reached by the
# actual generated schemas): exact-one `oneOf` selection, `enum`, `const`,
# discriminators, `pattern`, `format`, numeric bounds, `additionalProperties`,
# `uniqueItems`, and other assertion keywords. Findings about these are declined
# unless they affect the current generated `openapi.json` or this documented
# subset.
#
# `module_function` + a top-level `include` in the calling script keeps the
# receiverless call style the extraction moved out of; `SchemaInstanceValidator.x`
# works too.
module SchemaInstanceValidator
  module_function

  # Ruby name of a `$ref`, or nil.
  def ref_name(schema)
    return nil unless schema.is_a?(Hash) && schema["$ref"].is_a?(String)

    schema["$ref"].match(%r{/components/schemas/(.+)\z})&.captures&.first
  end

  # Returns [Set(declared non-null json-type strings), nullable?] for a single
  # schema node (no traversal). Handles OpenAPI 3.1 null-union (`type:[X,"null"]`)
  # and 3.0 `nullable:true`. An empty set means the node declares no type.
  def allowed_types(schema)
    return [Set.new, false] unless schema.is_a?(Hash)

    t = schema["type"]
    case t
    when Array
      members = t.compact
      non_null = members - ["null"]
      # A union of only ["null"] still constrains the value to null — keep "null"
      # as the type so a non-null value fails (an empty set would be unconstrained).
      types = non_null.empty? ? Set.new(["null"]) : Set.new(non_null)
      [types, members.include?("null")]
    when String
      # OpenAPI 3.1 scalar null type: the value must BE null — a real "null" type
      # constraint (so a non-null value fails), and it is nullable. Returning an
      # empty type set would wrongly impose no constraint at all.
      return [Set.new(["null"]), true] if t == "null"

      [Set.new([t]), schema["nullable"] == true]
    else
      [Set.new, schema["nullable"] == true]
    end
  end

  def json_type(value)
    case value
    when Hash then "object"
    when Array then "array"
    when String then "string"
    when true, false then "boolean"
    when Integer then "integer"
    when Float then "number"
    else "null"
    end
  end

  # True when `value`'s JSON type satisfies the declared `types`. integer/number
  # interchange with one guard: a float supplied for an integer-only field passes
  # only when it is mathematically integral (so FlexInt `1024.0` passes but `1.5`
  # fails).
  def type_matches?(types, value)
    return true if types.empty?

    actual = json_type(value)
    return true if types.include?(actual)

    if actual == "number" && types.include?("integer") && !types.include?("number")
      return value.is_a?(Float) && value.finite? && value == value.truncate
    end
    return true if actual == "integer" && types.include?("number")

    false
  end

  # Merges a schema's effective constraints across `$ref` (including 3.1
  # `$ref`-with-siblings) and `allOf`, returning
  # [required(Array), properties(Hash), types(Set), nullable(bool), items(schema),
  #  alt_groups(Array of branch-arrays), type_sets(Array of Sets)].
  # `allOf` is a conjunction: required is unioned, and properties AND array `items`
  # constrained by more than one branch are conjoined (allOf-wrapped) so a value
  # must satisfy all of them; `type_sets` holds each part's declared type-set (a
  # value must match every one). `anyOf`/`oneOf` are alternatives: their branches
  # are NOT merged (that would over-require) but each group is collected so
  # validation can require at least one branch to match — including groups
  # inherited through `$ref` and `allOf`. `visited` (component names) + depth guard
  # terminate reference/composition cycles.
  def merged_constraints(schema, components, visited = Set.new, depth = 0)
    req = []
    props = {}
    types = Set.new       # union of all declared types (for messages + concrete_for?)
    type_sets = []        # per-conjunctive-part declared type-sets — a value must
                          # satisfy EVERY one (allOf/$ref are a conjunction, so their
                          # type constraints INTERSECT, not union).
    # `$ref`(+siblings) and allOf form a CONJUNCTION: null is allowed only if every
    # part allows it, so a part that imposes a non-nullable type FORBIDS null. We
    # accumulate `forbids_null` (OR) and return its negation as the nullable flag.
    forbids_null = false
    items = nil
    alt_groups = []
    return [req, props, types, true, items, alt_groups, type_sets] if depth > 40 || !schema.is_a?(Hash)

    # When the same property (or array `items`) is constrained by more than one
    # conjunctive part (e.g. declared in two allOf branches), conjoin the schemas
    # so the value must satisfy ALL of them — not just the first seen.
    add_prop = lambda do |k, v|
      props[k] = props.key?(k) ? { "allOf" => [props[k], v] } : v
    end
    add_items = lambda do |i|
      items = items ? { "allOf" => [items, i] } : i
    end

    absorb = lambda do |sub|
      r2, p2, t2, sub_nullable, i2, a2, ts2 = merged_constraints(sub, components, visited, depth + 1)
      req.concat(r2)
      p2.each { |k, v| add_prop.call(k, v) }
      types.merge(t2)
      type_sets.concat(ts2)
      forbids_null ||= !sub_nullable
      add_items.call(i2) if i2
      alt_groups.concat(a2)
    end

    name = ref_name(schema)
    if name && !visited.include?(name)
      visited << name
      absorb.call(components[name])
      # fall through to local keywords (OpenAPI 3.1 permits $ref siblings)
    end

    t, nn = allowed_types(schema)
    types.merge(t)
    type_sets << t unless t.empty?
    # A node that imposes a concrete type but does not permit null forbids null.
    forbids_null ||= (!t.empty? && !nn)
    (schema["properties"] || {}).each { |k, v| add_prop.call(k, v) }
    (schema["required"] || []).each { |r| req << r }
    add_items.call(schema["items"]) if schema["items"]
    (schema["allOf"] || []).each { |sub| absorb.call(sub) }
    %w[anyOf oneOf].each do |key|
      alt_groups << schema[key] if schema[key].is_a?(Array) && !schema[key].empty?
    end

    # An anyOf/oneOf group (local or inherited via $ref/allOf) permits null only if
    # at least one branch does; if every branch forbids null, the group forbids it
    # (the value must satisfy some branch). Conjoin that with the surrounding
    # $ref/allOf constraints. Branch nullability is computed with a FRESH visited
    # set so outer traversal state can't short-circuit it.
    alt_groups.each do |branches|
      group_allows_null = branches.any? do |branch|
        _, _, _, branch_nullable, = merged_constraints(branch, components, Set.new, depth + 1)
        branch_nullable
      end
      forbids_null ||= !group_allows_null
    end

    [req, props, types, !forbids_null, items, alt_groups, type_sets]
  end

  # Composition-aware validation of `value` against `schema`. Reports
  # (path-tagged) errors for a missing required field, a required field present as
  # null against a non-nullable schema, a null array element against a non-nullable
  # item schema, a ROOT null against a non-nullable schema, and a present value
  # whose JSON type contradicts the declared type.
  #
  # NESTED nulls are tolerated: the Smithy-derived OpenAPI under-marks some
  # nullable optionals (e.g. Person.bio/location are `type:string` yet the wire
  # sends null), so flagging them would be a false positive. That exemption is
  # only sound because something above HAS looked — a required-but-null field is
  # caught by the required loop in its parent, a null array element by the items
  # check in its array. It is an "the enclosing context already judged this"
  # rule, not an "any null is fine" rule.
  #
  # A ROOT null (depth 0) has no enclosing context and therefore nothing standing
  # behind the exemption, so it is checked here. Skipping it let a published
  # example of `value: null` under a non-nullable schema be counted as VALIDATED
  # — the gate approving exactly the class of contradiction it exists to catch.
  def instance_errors(prefix, value, schema, components, depth = 0)
    return [] if depth > 60

    if value.nil?
      return [] unless depth.zero?

      _, _, _, root_nullable, = merged_constraints(schema, components)
      return [] if root_nullable

      label = prefix.empty? ? "(root)" : prefix
      return ["#{label}: value is null but the schema is not nullable"]
    end

    req, props, _types, _nullable, items, alt_groups, type_sets = merged_constraints(schema, components)

    # The value must satisfy EVERY conjunctive part's declared type (allOf/$ref
    # intersect their type constraints — a value matching only one contradictory
    # branch fails).
    type_sets.each do |ts|
      next if type_matches?(ts, value)

      label = prefix.empty? ? "(root)" : prefix
      return ["#{label}: expected #{ts.to_a.sort.join('|')}, got #{json_type(value)}"]
    end

    errs = []

    # anyOf/oneOf: the value must satisfy at least one branch of each group
    # (oneOf is validated as "at least one" — enforcing exactly-one would need full
    # discriminator/enum/const validation to avoid false positives).
    alt_groups.each do |branches|
      next if branches.any? { |branch| instance_errors(prefix, value, branch, components, depth + 1).empty? }

      label = prefix.empty? ? "(root)" : prefix
      errs << "#{label}: value matches none of the #{branches.length} allowed alternatives (anyOf/oneOf)"
    end

    if value.is_a?(Hash)
      req.uniq.each do |rk|
        field = prefix.empty? ? rk : "#{prefix}/#{rk}"
        if !value.key?(rk)
          errs << "missing required field `#{field}`"
        elsif value[rk].nil?
          _, _, _, field_nullable, = merged_constraints(props[rk] || {}, components)
          errs << "#{field}: required field is null but its schema is not nullable" unless field_nullable
        end
      end
      value.each do |k, v|
        next unless props.key?(k)

        child = prefix.empty? ? k : "#{prefix}/#{k}"
        errs.concat(instance_errors(child, v, props[k], components, depth + 1))
      end
    elsif value.is_a?(Array) && items
      _, _, _, item_nullable, = merged_constraints(items, components)
      value.each_with_index do |item, i|
        ip = "#{prefix}[#{i}]"
        if item.nil?
          errs << "#{ip}: null array element but the item schema is not nullable" unless item_nullable
          next
        end
        errs.concat(instance_errors(ip, item, items, components, depth + 1))
      end
    end

    errs
  end
end
