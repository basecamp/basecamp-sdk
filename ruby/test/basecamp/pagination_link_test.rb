# frozen_string_literal: true

require "test_helper"
require "timeout"

# Link-header parsing tests.
#
# parse_next_link had no dedicated unit test — it was only ever exercised
# indirectly through pagination flows, always with well-formed headers. The Link
# header is attacker-influenced (Security.same_origin? guards the parsed URL to
# stop SSRF through a poisoned one), so its behaviour on malformed input is part
# of the contract.
# The adversarial cases below exist in all six SDKs.
#
# The method is private, so it is called through send rather than promoted to
# the public surface for the benefit of a test.
class PaginationLinkTest < Minitest::Test
  def setup
    @http = Basecamp::Http.allocate
  end

  def parse(header)
    @http.send(:parse_next_link, header)
  end

  def test_extracts_url_from_standard_header
    assert_equal "https://api.example.com/items?page=2",
                 parse('<https://api.example.com/items?page=2>; rel="next"')
  end

  def test_picks_next_out_of_several_rels
    header = '<https://api.example.com/items?page=1>; rel="first", ' \
             '<https://api.example.com/items?page=2>; rel="next", ' \
             '<https://api.example.com/items?page=10>; rel="last"'

    assert_equal "https://api.example.com/items?page=2", parse(header)
  end

  def test_returns_nil_when_no_next_rel
    assert_nil parse('<https://api.example.com/items?page=1>; rel="first"')
  end

  def test_returns_nil_for_empty_and_nil_headers
    assert_nil parse("")
    assert_nil parse(nil)
  end

  # --- Adversarial input ---

  def test_returns_nil_when_bracket_never_closes
    assert_nil parse('<https://api.example.com/page2; rel="next"')
  end

  def test_reads_closing_bracket_before_opening_bracket
    assert_equal "https://api.example.com/page2",
                 parse('>x<https://api.example.com/page2>; rel="next"')
  end

  # Parity with the old /<([^>]+)>/ spelling: [^>] cannot span a ">".
  def test_truncates_url_at_first_raw_closing_bracket
    assert_equal "https://api.example.com/page2?q=a",
                 parse('<https://api.example.com/page2?q=a>b>; rel="next"')
  end

  # Parity with the old spelling: leftmost match wins.
  def test_takes_first_of_multiple_bracket_pairs_in_one_part
    assert_equal "https://api.example.com/a",
                 parse('<https://api.example.com/a> <https://api.example.com/b>; rel="next"')
  end

  # Parity with the old spelling: [^>]+ requires at least one character, so an
  # empty <> is not a match and the scan moves on. A naive index(">", start + 1)
  # without this check would return "".
  def test_skips_empty_bracket_pair
    assert_equal "https://api.example.com/page2",
                 parse('<> <https://api.example.com/page2>; rel="next"')
  end

  def test_keeps_scanning_past_a_malformed_part
    assert_equal "https://api.example.com/page2",
                 parse('<malformed; rel="next", <https://api.example.com/page2>; rel="next"')
  end

  # Many "<" start positions with no reachable ">" — the shape that punishes a
  # backtracking regex. Onigmo does not actually realize the blowup CodeQL flags
  # in alert 48 (measured linear to 3.2M characters), so this case documents the
  # contract and guards the shared behaviour rather than proving a fix.
  # Asserting behaviour and completion, not elapsed time: the suite already has
  # timing flakiness (#655) and a duration bound would add more.
  def test_handles_pathological_header
    many = "<" * 50_000

    assert_nil parse(%(#{many}; rel="next"))
    assert_nil parse(%(>#{many}; rel="next"))
  end

  # The pathological case for the scan that replaced the regex, which is a
  # different shape from the one above: that header returns after a single
  # index(">") and never takes the empty-<> branch, so the skip loop's own worst
  # case went untested. Every "<>" here advances the cursor by one and goes
  # round again.
  #
  # The leading "é" is the whole point. It pushes the string off CR_7BIT, and
  # String#index(str, offset) takes a CHARACTER offset, which on such a string
  # Ruby resolves by walking from the start — O(cursor) per call, so the skip
  # loop turns quadratic. Indexing the binary view is O(1) in the offset. This
  # is the one place a timeout earns its keep: scanning part.b does this in
  # ~30ms, the character-indexed one in ~15s, so 5s sits ~150x above the fixed
  # path and well below the broken one — a regression gate, not a timing bound.
  # (The other SDKs need no such guard; only Ruby indexes by character.)
  def test_handles_many_empty_bracket_pairs_in_a_non_ascii_header
    pairs = "é" + ("<>" * 160_000)

    Timeout.timeout(5) do
      # No non-empty pair anywhere: every iteration skips, then it runs out.
      assert_nil parse(%(#{pairs}; rel="next"))

      # Same prefix, but the skips have to land on a real pair at the end.
      assert_equal "https://api.example.com/page2",
                   parse(%(#{pairs}<https://api.example.com/page2>; rel="next"))
    end
  end

  # --- Malformed UTF-8 ---
  #
  # The header comes off the wire and nothing between the socket and here
  # validates it as UTF-8. Net::HTTP happens to hand Faraday ASCII-8BIT values,
  # where every byte offset is a character and none of this can bite, but that
  # is an adapter's accident rather than a contract — so both the extractor and
  # its caller have to be total on bytes that are not valid UTF-8.
  #
  # The witness is "\xC2<\x80>" (bytes 194 60 128 62) tagged UTF-8. "\xC2" is a
  # two-byte lead, so Ruby reads byte 2 — the one just past the "<" — as the
  # middle of a character.

  MALFORMED = "\xC2<\x80>"

  # byteindex is O(1) in the offset but RAISES IndexError when that offset does
  # not land on a character boundary, which is exactly what byte 2 above is. A
  # binary view has no character boundaries to violate, so every offset is
  # legal; force_encoding hands the caller's encoding back on the way out.
  def test_extracts_from_a_part_whose_bytes_are_not_valid_utf8
    part = MALFORMED.b.force_encoding(Encoding::UTF_8)

    extracted = @http.send(:extract_angle_bracketed, part)

    assert_equal "\x80".b, extracted.b
    assert_equal Encoding::UTF_8, extracted.encoding
  end

  # Binary in, binary out: the scan must not silently retag its result.
  def test_preserves_binary_encoding_of_the_part
    extracted = @http.send(:extract_angle_bracketed, MALFORMED.b)

    assert_equal "\x80".b, extracted
    assert_equal Encoding::BINARY, extracted.encoding
  end

  # Valid input keeps its encoding too, so the .b round trip is invisible.
  def test_preserves_utf8_encoding_of_the_part
    extracted = @http.send(:extract_angle_bracketed, "<https://api.example.com/é>")

    assert_equal "https://api.example.com/é", extracted
    assert_equal Encoding::UTF_8, extracted.encoding
  end

  # The whole path, not just the extractor. String#split and String#strip raise
  # ArgumentError on a broken coderange, so parse_next_link crashed on this
  # header one frame ABOVE extract_angle_bracketed — a fix confined to the
  # extractor would leave the header just as fatal. Splitting a binary view is
  # total, and ASCII-8BIT strips and compares identically for ASCII literals.
  def test_parses_a_header_whose_bytes_are_not_valid_utf8
    header = %(#{MALFORMED}; rel="next").b.force_encoding(Encoding::UTF_8)

    next_url = parse(header)

    assert_equal "\x80".b, next_url.b
    assert_equal Encoding::UTF_8, next_url.encoding
  end

  # Same header, binary-tagged — the shape Net::HTTP actually produces.
  def test_parses_a_binary_header
    next_url = parse(%(#{MALFORMED}; rel="next").b)

    assert_equal "\x80".b, next_url
    assert_equal Encoding::BINARY, next_url.encoding
  end

  # A malformed part must not swallow a well-formed one after it, and the comma
  # split has to survive the malformed bytes to reach it.
  def test_keeps_scanning_past_a_malformed_utf8_part
    header = %(#{MALFORMED[0]}<; rel="next", <https://api.example.com/page2>; rel="next")
             .b.force_encoding(Encoding::UTF_8)

    assert_equal "https://api.example.com/page2", parse(header)
  end

  # --- Non-ASCII-compatible encodings ---
  #
  # Real transports never produce these: Net::HTTP hands Faraday ASCII-8BIT
  # header values, so only a test stub or a custom adapter can tag a header
  # UTF-16/32. The parser still has a contract there — the byte scan's ASCII
  # literals cannot match "<" spelled 3C 00, so before the transcode a
  # genuinely UTF-16LE header parsed to nil silently (where the pre-#678
  # character path at least crashed loudly).

  # A genuinely UTF-16LE-encoded header transcodes and parses; the URL comes
  # back UTF-8 because the transcode rebinds the header before the retag.
  def test_parses_a_genuinely_utf16_encoded_header
    header = '<https://api.example.com/page2>; rel="next"'.encode(Encoding::UTF_16LE)

    next_url = parse(header)

    assert_equal "https://api.example.com/page2", next_url
    assert_equal Encoding::UTF_8, next_url.encoding
  end

  # ASCII bytes MIStagged UTF-16LE, natural (odd) bytesize: invalid UTF-16LE,
  # so the transcode raises and the byte-scan fallthrough keeps the pre-fix
  # behaviour — the URL comes back still mistagged, and Security.same_origin?
  # refuses it downstream (ApiError at the follow, no origin-check bypass).
  # Green before and after the transcode was added; it locks the fallthrough.
  def test_mistagged_odd_bytesize_header_keeps_the_refusal_path
    header = '<https://api.example.com/page2>; rel="next"'.b.force_encoding(Encoding::UTF_16LE)

    next_url = parse(header)

    assert_equal "https://api.example.com/page2".b, next_url.b
    assert_equal Encoding::UTF_16LE, next_url.encoding
    assert_not Basecamp::Security.same_origin?(next_url, "https://api.example.com")
  end

  # ASCII bytes mistagged UTF-16LE at an even bytesize ARE valid UTF-16LE, so
  # the transcode succeeds and yields CJK mojibake with no ASCII rel="next":
  # the parse is nil (before the transcode: a mistagged URL that downstream
  # refused with ApiError). Garbage-tagged garbage — nil is as good a refusal
  # as a raise, and this pins that documented trade rather than narrating it.
  def test_mistagged_even_bytesize_header_parses_to_nil
    header = '<https://api.example.com/page2>; rel="next" '.b.force_encoding(Encoding::UTF_16LE)

    assert_equal 44, header.bytesize
    assert_nil parse(header)
  end
end
