# frozen_string_literal: true

module Basecamp
  # Text trimming that matches the reference's, byte for byte.
  #
  # Ruby's <tt>String#strip</tt> is NOT that function, and the difference runs
  # in both directions. Measured over all 256 single bytes and the whole of
  # +unicode.IsSpace+:
  #
  # * <tt>strip</tt> removes a leading or trailing NUL and +strings.TrimSpace+
  #   does not — <tt>unicode.IsSpace</tt> has no NUL in it. That is the
  #   ACCEPTING direction wherever a trimmed value is compared against a table:
  #   <tt>"Comment\\0"</tt> selected a real recording type here and was
  #   +unknown_recording_type+ there.
  # * <tt>strip</tt> on a BYTE string removes none of the nineteen multi-byte
  #   spaces +IsSpace+ carries — U+0085, U+00A0, U+1680, U+2000..U+200A,
  #   U+2028, U+2029, U+202F, U+205F, U+3000 — which the reference trims. That
  #   direction merely refuses, but it is still a divergence.
  #
  # One byte of the 256 diverges in the first direction and nineteen characters
  # in the second, so neither <tt>strip</tt> nor a plain ASCII trim is the
  # reference's function; this walk is.
  #
  # It lives here rather than in either caller because both {Basecamp::Mentions}
  # and the recording-summary composite need the same trim, and a second
  # hand-written copy of a byte walk is how the two would come apart.
  module Text
    # Every member of +unicode.IsSpace+, as its UTF-8 bytes. Built from the
    # literals rather than a range so the set is readable, and keyed by byte
    # string so a lookup needs no decoding. No member is a prefix, a suffix or a
    # substring of another, which is what lets the walk below take the first
    # width that matches rather than the longest.
    UTF8_SPACE_BYTES = ([ " ", "\t", "\n", "\v", "\f", "\r", "", " ", " ",
                          " ", " ", " ", " ", "　" ] +
                        (0x2000..0x200A).map { |codepoint| codepoint.chr(Encoding::UTF_8) })
                       .to_h { |character| [ character.b, true ] }.freeze

    # The widest member, so the walk knows how far to look.
    MAX_SPACE_WIDTH = UTF8_SPACE_BYTES.keys.map(&:bytesize).max

    module_function

    # The value with leading and trailing +unicode.IsSpace+ runs removed, as
    # +strings.TrimSpace+ removes them.
    #
    # @param value [String]
    # @return [String] the trimmed bytes
    def trim_space(value)
      bytes = value.b
      first = space_run_end(bytes)
      bytes.byteslice(first, space_run_start(bytes, first) - first)
    end

    # The offset just past the leading run of spaces.
    def space_run_end(bytes)
      offset = 0
      while (width = space_width_at(bytes, offset, bytes.bytesize))
        offset += width
      end
      offset
    end

    # The offset where the trailing run of spaces begins, never below +floor+ —
    # so a value that is nothing but spaces trims to empty rather than
    # underflowing past the leading run already consumed.
    def space_run_start(bytes, floor)
      offset = bytes.bytesize
      while (width = space_width_behind(bytes, offset, floor))
        offset -= width
      end
      offset
    end

    # The width of the space at +offset+, or nil when there is none.
    def space_width_at(bytes, offset, ceiling)
      (1..MAX_SPACE_WIDTH).each do |width|
        next if offset + width > ceiling
        return width if UTF8_SPACE_BYTES.key?(bytes.byteslice(offset, width))
      end
      nil
    end

    # The width of the space ending at +offset+, or nil when there is none.
    def space_width_behind(bytes, offset, floor)
      (1..MAX_SPACE_WIDTH).each do |width|
        next if offset - width < floor
        return width if UTF8_SPACE_BYTES.key?(bytes.byteslice(offset - width, width))
      end
      nil
    end

    private_class_method :space_run_end, :space_run_start, :space_width_at, :space_width_behind
  end
end
