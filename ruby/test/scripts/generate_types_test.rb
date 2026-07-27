# frozen_string_literal: true

# Fixture for the deprecation doc-comment rendering in the Ruby types generator
# (#406, #420). A deprecation reason can be sourced from a multi-line OpenAPI
# description; interpolating one into a single "# @deprecated ..." line would
# leave every continuation on a bare, un-commented source line and produce a
# types.rb that does not parse.
#
# Ruby is the only owned generator exposed this way: Python escapes the reason
# (see python/tests/test_generate_deprecation.py), and Kotlin/TypeScript emit
# block comments. Swift solves it the same way this does, with
# deprecationDocLines (swift/Tests/BasecampTests/DeprecationDocCommentTests.swift).

require "test_helper"
require "ripper"

require_relative "../../scripts/generate-types"

class GenerateTypesDeprecationTest < Minitest::Test
  # A blank interior line, embedded double quotes, a backslash, and a final line
  # that is not valid bare Ruby — so the guard test below has real teeth.
  MULTILINE = <<~REASON.chomp
    prefer type_names[].

    The singular "type" param is a \\ legacy alias.
    end
  REASON

  # Renders the two emit sites into a plausible types.rb fragment.
  def types_fragment(class_tag_lines, attribute_tag_lines)
    <<~RUBY
      module Basecamp
        module Types
      #{class_tag_lines.join("\n")}
          class Search
            include TypeHelpers
      #{attribute_tag_lines.join("\n")}
            attr_accessor :type, :type_names
          end
        end
      end
    RUBY
  end

  def assert_parses(source, message)
    assert Ripper.sexp(source), message
  end

  def test_every_emitted_line_is_a_comment
    lines = deprecation_doc_lines(MULTILINE, indent: "    ")

    assert_operator lines.length, :>, 1, "multi-line reason should emit multiple lines"
    lines.each do |line|
      assert_match(/\A\s*#/, line, "line escaped the comment leader: #{line.inspect}")
    end
  end

  def test_multiline_reason_keeps_generated_types_parseable
    source = types_fragment(
      deprecation_doc_lines(MULTILINE, indent: "    "),
      deprecation_doc_lines(MULTILINE, indent: "      ", tag_prefix: "  ")
    )

    assert_parses source, "generated types.rb fragment must parse for a multi-line reason"
  end

  def test_raw_interpolation_would_break_without_this_helper
    # Guard: the fixture really is hostile. The pre-fix single-line
    # interpolation does not parse, so the helper is doing real work.
    source = types_fragment(
      [ "    # @deprecated #{MULTILINE}" ],
      [ "      #   @deprecated #{MULTILINE}" ]
    )

    assert_nil Ripper.sexp(source),
               "expected the raw single-line interpolation to break parsing"
  end

  def test_continuations_are_indented_under_the_yard_tag
    lines = deprecation_doc_lines(MULTILINE, indent: "      ", tag_prefix: "  ")

    assert_equal "      #   @deprecated prefer type_names[].", lines[0]
    assert_equal "      #", lines[1], "blank interior line should stay a bare comment"
    assert_equal '      #     The singular "type" param is a \\ legacy alias.', lines[2]
    assert_equal "      #     end", lines[3]
  end

  # Output neutrality: a single-line reason must render byte-identically to the
  # interpolation this replaced, so regenerating types.rb produces no drift.
  def test_single_line_reason_matches_the_previous_interpolation
    reason = "Use typeNames (type_names[]) instead"

    assert_equal [ "    # @deprecated #{reason}" ],
                 deprecation_doc_lines(reason, indent: "    ")
    assert_equal [ "      #   @deprecated #{reason}" ],
                 deprecation_doc_lines(reason, indent: "      ", tag_prefix: "  ")
  end

  def test_crlf_reason_does_not_leak_carriage_returns
    lines = deprecation_doc_lines("first\r\nsecond", indent: "    ")

    assert_equal [ "    # @deprecated first", "    #   second" ], lines
  end
end
