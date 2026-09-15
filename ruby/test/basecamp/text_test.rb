# frozen_string_literal: true

require "test_helper"

# Tests for Basecamp::Text \u2014 the strings.TrimSpace equivalent both the mention
# helpers and the recording-summary router trim with.
#
# It had NO test file at all until a mutation run found out: dropping the floor
# argument from the trailing walk makes an all-whitespace value return nil
# instead of "", which raises NoMethodError out of two public methods \u2014 and the
# whole suite stayed green, because the method is only ever reached incidentally
# through inputs that are not entirely spaces.
#
# Every non-ASCII character in a test VALUE here is written as an escape. Prose
# punctuation is not, because it is visible; an invisible character in an input
# or an expectation is the hazard, and the distinction is the claim. That was
# not true of the first two versions. The first asserted a plain space against
# an input carrying a literal U+2003 — the expectation looked right in the diff
# and was a different string. The second added this very paragraph claiming the
# rule while leaving the literals in place, which a reviewer found with cat -A:
# a comment asserting a property of the file it sits in, contradicted by the
# file. An invisible character in a test is a defect waiting for a reader who
# trusts their eyes, and a claim that there are none is worse than neither.
class TextTest < Minitest::Test
  include TestHelper

  NUL = 0.chr

  # Every member of unicode.IsSpace, which is the set the reference trims.
  ASCII_SPACES = [ " ", "\t", "\n", "\v", "\f", "\r" ].freeze
  MULTI_BYTE_SPACES = ([ "\u0085", "\u00A0", "\u1680", "\u2028", "\u2029",
                         "\u202F", "\u205F", "\u3000" ] +
                       (0x2000..0x200A).map { |codepoint| codepoint.chr(Encoding::UTF_8) }).freeze

  def trim(value)
    Basecamp::Text.trim_space(value)
  end

  def test_the_table_is_exactly_unicode_is_space
    # 25 members, six of them ASCII. The counts are here so that adding or
    # losing one is a failure rather than a silent change in what gets trimmed.
    assert_equal 25, Basecamp::Text::UTF8_SPACE_BYTES.length
    assert_equal 3, Basecamp::Text::MAX_SPACE_WIDTH
    assert_equal 6, ASCII_SPACES.length
    assert_equal 19, MULTI_BYTE_SPACES.length

    (ASCII_SPACES + MULTI_BYTE_SPACES).each do |space|
      assert Basecamp::Text::UTF8_SPACE_BYTES.key?(space.b), "#{space.inspect} is in unicode.IsSpace"
    end
  end

  def test_a_value_that_is_entirely_spaces_trims_to_empty
    # THE case the floor argument exists for. Without it the trailing walk runs
    # past the leading one, byteslice gets a negative length and returns nil,
    # and nil raises out of every caller. A mutation removing the floor left the
    # whole suite green.
    [ "", " ", "   ", "\t", "\t\n\r", "\u00A0", "\u3000 ", "\u2003 \t\u2009",
      "\u0085\u2028\u2029" ].each do |value|
      assert_equal "", trim(value), "#{value.inspect} is nothing but spaces"
    end

    # And each member on its own, since the floor interacts with the width walk.
    (ASCII_SPACES + MULTI_BYTE_SPACES).each do |space|
      assert_equal "", trim(space), "#{space.inspect} alone"
      assert_equal "", trim(space * 3), "#{space.inspect} repeated"
    end
  end

  def test_leading_and_trailing_spaces_go_and_interior_ones_stay
    assert_equal "a b", trim("  a b  ")
    assert_equal "a\tb", trim("\n a\tb \n")
    assert_equal "x", trim("  x\u3000")
    assert_equal "x\u2003y".b, trim("\u00A0x\u2003y\u00A0")
    assert_equal "a\u200Ab".b, trim("\u2028a\u200Ab\u2029")
  end

  def test_nul_is_not_a_space
    # unicode.IsSpace has no NUL in it, and String#strip does \u2014 the one byte of
    # 256 where the two disagree. A routing key is compared against a table
    # after trimming, so trimming a NUL made a malformed key select a real type.
    assert_equal "#{NUL}x#{NUL}", trim("#{NUL}x#{NUL}")
    assert_equal NUL, trim(" #{NUL} ")
    assert_equal "x#{NUL}", trim(" x#{NUL} ")
  end

  def test_every_other_byte_is_left_alone
    # The complement of the rule: 256 bytes, and only the six ASCII spaces trim.
    (0..255).each do |byte|
      framed = (byte.chr + "x" + byte.chr).b
      expected = ASCII_SPACES.include?(byte.chr) ? "x" : framed

      assert_equal expected, trim(framed), "byte 0x#{format("%02X", byte)}"
    end
  end

  def test_a_partial_space_sequence_is_not_trimmed
    # U+00A0 is 0xC2 0xA0. Neither byte alone is a space, so a truncated
    # sequence at either end has to survive \u2014 this is what the width walk is
    # for, and a byte-at-a-time implementation would get it wrong.
    assert_equal "\xC2".b, trim("\xC2".b)
    assert_equal "\xC2x".b, trim("\xC2x".b)
    assert_equal "x\xC2".b, trim(" x\xC2 ".b)
    assert_equal "\xA0x".b, trim("\xA0x".b)
  end

  def test_invalid_utf_8_does_not_raise
    # The values reaching this are rich text and routing arguments other people
    # wrote, so a broken encoding has to trim rather than raise.
    assert_equal "\xFF\xFE".b, trim(" \xFF\xFE ".b)
    assert_equal "", trim("\u00A0\u2003".b)
  end

  def test_the_result_is_bytes
    # Callers scan the result, and a byte string is what the rest of the
    # mention path assumes.
    assert_equal Encoding::BINARY, trim(" x ").encoding
  end
end
