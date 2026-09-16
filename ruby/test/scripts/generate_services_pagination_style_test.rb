# frozen_string_literal: true

# The pagination trait's `style` is load-bearing in this generator, and Ruby is
# the one place where it is not the only gate.
#
# `has_pagination` decides whether the emitted method follows `Link: rel="next"`
# and flattens the walk. It used to key off the trait's mere presence, so the
# `cursor` style the trait documents would have produced exactly the walk it
# exists to avoid. But `generate_method` asks `(returns_array || has_pagination)`
# — so a cursor operation answering a bare array would still have walked, by the
# other half of that disjunction. `cursor_pagination` is carried for that gate
# alone, and `test_a_cursor_operation_with_a_bare_array_response_still_does_not_walk`
# is the case that would otherwise pass unnoticed.

require "test_helper"
require_relative "../../scripts/generate-services"

class GenerateServicesPaginationStyleTest < Minitest::Test
  def setup
    @generator = ServiceGenerator.allocate
  end

  def test_link_style_auto_paginates
    assert parse(style: "link")[:has_pagination]
  end

  def test_cursor_style_does_not_auto_paginate
    assert_not parse(style: "cursor")[:has_pagination]
    assert parse(style: "cursor")[:cursor_pagination]
  end

  def test_no_trait_at_all_is_simply_unpaginated
    assert_not parse(style: :none)[:has_pagination]
    assert_not parse(style: :none)[:cursor_pagination]
  end

  # The key drives envelope unwrapping in type resolution: set, it turns
  # `{events: [...], position}` into the item type. A cursor operation is typed
  # as its envelope, so the key must not reach those consumers at all.
  def test_the_key_is_withheld_from_a_cursor_operation
    assert_equal "events", parse(style: "link")[:pagination_key]
    assert_nil parse(style: "cursor")[:pagination_key]
  end

  # The disjunction Ruby alone carries: a bare-array response sets returns_array,
  # which would send a cursor operation down the Link-following path even though
  # has_pagination is false for it.
  def test_a_cursor_operation_with_a_bare_array_response_still_does_not_walk
    operation = parse(style: "cursor", response_schema: { "type" => "array" })

    assert operation[:returns_array], "the fixture must exercise the returns_array half"
    assert_not paginated?(operation), "a cursor operation never follows Link headers"
  end

  def test_an_unrecognised_style_is_refused_rather_than_read_as_unpaginated
    [ "page", "linkk", "Link", "", nil ].each do |style|
      error = assert_raises(RuntimeError) { parse(style: style) }
      assert_match(/unsupported pagination style/, error.message, "style #{style.inspect}")
    end
  end

  private
    def parse(style:, response_schema: { "type" => "object" })
      operation = {
        "operationId" => "PollWidgets",
        "responses" => { "200" => { "content" => { "application/json" => { "schema" => response_schema } } } }
      }
      unless style == :none
        operation["x-basecamp-pagination"] = { "key" => "events" }
        operation["x-basecamp-pagination"]["style"] = style unless style.nil?
      end

      @generator.send(:parse_operation, "/{accountId}/widgets.json", "get", operation)
    end

    # The gate generate_method applies, kept in one place so the test names the
    # real predicate rather than a paraphrase of it.
    def paginated?(op)
      !op[:cursor_pagination] && (op[:returns_array] || op[:has_pagination]) && !op[:pagination_key]
    end
end
