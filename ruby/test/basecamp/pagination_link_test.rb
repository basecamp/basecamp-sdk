# frozen_string_literal: true

require "test_helper"

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
end
