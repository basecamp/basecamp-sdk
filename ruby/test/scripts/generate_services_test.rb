# frozen_string_literal: true

# Fixture for the YARD `@param` continuation rendering in the Ruby services
# generator.
#
# A body or query member's description can be multi-line, and the generator
# folds it under the `# @param name [Type] ` tag it has already written. The
# interesting line is the blank one: a paragraph break inside a description must
# emit a bare `#`, not `#` followed by the continuation padding. Padding an
# empty line is trailing whitespace, and `git diff --check` fails on it.
#
# This lives here, not in the emitted file, for the reason the fix exists: a
# generated file patched by hand regrows the defect on the next
# `make generate`. It is the sibling of the deprecation-reason case in
# generate_types_test.rb, which had the same shape.

require "test_helper"

require_relative "../../scripts/generate-services"

class GenerateServicesYardParamTest < Minitest::Test
  # One blank interior line and nothing else exotic — the case the fix exists
  # for, kept clean enough that the line-by-line assertion below can name every
  # rendered line. The other spellings of a break, and the CRLF pair, carry their
  # own literals in the tests further down: trailing-space and tab-only in
  # `test_no_emitted_line_carries_trailing_whitespace`, spaces-only in
  # `test_whitespace_only_line_is_treated_as_a_break`, `\r\n` in
  # `test_carriage_returns_do_not_leak`.
  MULTILINE = "The entry's join link.\n\nRead it back as `join_url`.\nNever as `url`."

  def setup
    @generator = ServiceGenerator.allocate
  end

  def render(text)
    @generator.send(:yard_param_description, text)
  end

  def test_blank_interior_line_becomes_a_bare_comment
    lines = render(MULTILINE).split("\n", -1)

    assert_equal "The entry's join link.", lines[0], "first line stays bare for the caller's tag"
    assert_equal "      #", lines[1], "a paragraph break must not carry the continuation padding"
    assert_equal "      #   Read it back as `join_url`.", lines[2]
    assert_equal "      #   Never as `url`.", lines[3]
  end

  def test_no_emitted_line_carries_trailing_whitespace
    lines = render("first\n\nsecond   \n\t\nthird").split("\n", -1)

    lines.each do |line|
      assert_equal line.rstrip, line, "trailing whitespace escaped into: #{line.inspect}"
    end
  end

  # A whitespace-only line is a paragraph break too — it just arrived spelled
  # with spaces. Padding it would be the same defect wearing a hat.
  def test_whitespace_only_line_is_treated_as_a_break
    assert_equal "first\n      #\n      #   second", render("first\n   \nsecond")
  end

  def test_carriage_returns_do_not_leak
    assert_equal "first\n      #   second", render("first\r\nsecond")
  end

  # Output neutrality: a single-line description must render byte-identically to
  # the interpolation this replaced, so regenerating the services produces no
  # drift beyond the blank-line fix.
  def test_single_line_description_is_unchanged
    assert_equal "participant ids", render("participant ids")
  end

  def test_nil_description_renders_empty
    assert_equal "", render(nil)
  end
end
