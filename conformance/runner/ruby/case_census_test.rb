# frozen_string_literal: true

# Case-census contract (#602).
#
# The check is green on the real fixture tree by construction, so a live run only
# ever proves it can say yes. These cases run it against a SYNTHETIC fixture set
# and prove it can say no — the `mode: "moc"` case in particular, which every
# runner's "mock unless told otherwise" filter drops with nothing printed. That
# divergence is asserted end-to-end here: the census and the run loop's own
# predicate (CaseCensus.mock_mode?, shared with the load path) disagree by one,
# and CaseCensus.count_failure reports it.
#
# Run: `bundle exec ruby case_census_test.rb`

require "minitest/autorun"
require "tmpdir"
require_relative "runner"

class CaseCensusTest < Minitest::Test
  # One case of each kind: a plain mock case (no `mode` at all, the common
  # spelling), a live case the runners are meant to drop, and a typo'd mode that
  # nothing recognizes.
  FIXTURE = [
    { "name" => "plain", "operation" => "GetProject" },
    { "name" => "live one", "operation" => "GetProject", "mode" => "live" },
    { "name" => "typo", "operation" => "GetProject", "mode" => "moc" }
  ].freeze

  def with_fixture_tree(files)
    Dir.mktmpdir do |dir|
      files.each do |relative, content|
        path = File.join(dir, relative)
        FileUtils.mkdir_p(File.dirname(path))
        File.write(path, content)
      end
      yield dir
    end
  end

  def test_census_counts_every_case_that_is_not_explicitly_live
    with_fixture_tree("cases.json" => JSON.dump(FIXTURE)) do |dir|
      assert_equal 2, CaseCensus.non_live_case_count(dir)
    end
  end

  def test_a_typoed_mode_makes_the_count_check_fail
    # The regression this whole check exists for. The runner's own filter keeps
    # one case; the census counts two; the difference is the case executed by
    # nothing.
    with_fixture_tree("cases.json" => JSON.dump(FIXTURE)) do |dir|
      ran = FIXTURE.count { |t| CaseCensus.mock_mode?(t["mode"]) }
      assert_equal 1, ran, "the run loop should keep only the plain case"

      failure = CaseCensus.count_failure(ran, CaseCensus.non_live_case_count(dir))

      refute_nil failure, "a case no runner recognizes must fail the count check"
      assert_includes failure, "1 executed by nothing"
    end
  end

  def test_census_finds_fixtures_nested_below_the_tests_directory
    # No runner globs recursively, so a case parked one directory down is run by
    # nothing. The census walks, which is what makes that visible.
    with_fixture_tree("nested/cases.json" => JSON.dump(FIXTURE)) do |dir|
      assert_equal 2, CaseCensus.non_live_case_count(dir)
    end
  end

  def test_census_rejects_a_fixture_that_does_not_parse
    with_fixture_tree("broken.json" => '[{"name": "truncated"') do |dir|
      assert_raises(CaseCensus::Error) { CaseCensus.non_live_case_count(dir) }
    end
  end

  def test_census_rejects_a_fixture_that_is_not_an_array
    with_fixture_tree("object.json" => '{"name": "not a list"}') do |dir|
      assert_raises(CaseCensus::Error) { CaseCensus.non_live_case_count(dir) }
    end
  end

  def test_census_reports_an_unreadable_subtree
    # Dir.glob (and Find.find, which rescues Errno::EACCES) swallowed the error
    # and omitted the subtree — and the runner's non-recursive glob omits it
    # too, leaving both sides of the census agreeing over cases neither counted.
    # Root reads through a 0o000 directory, so under root the assertion is that
    # the cases are still counted; either way they must never be dropped.
    files = { "cases.json" => JSON.dump(FIXTURE), "locked/nested.json" => JSON.dump(FIXTURE) }
    with_fixture_tree(files) do |dir|
      locked = File.join(dir, "locked")
      File.chmod(0o000, locked)
      begin
        if Process.euid.zero?
          assert_equal 4, CaseCensus.non_live_case_count(dir)
        else
          assert_raises(CaseCensus::Error) { CaseCensus.non_live_case_count(dir) }
        end
      ensure
        File.chmod(0o755, locked)
      end
    end
  end

  def test_census_rejects_an_empty_tree
    # A census that counted nothing certifies nothing: zero on both sides is the
    # shape a broken walk takes.
    with_fixture_tree({}) do |dir|
      assert_raises(CaseCensus::Error) { CaseCensus.non_live_case_count(dir) }
    end
  end

  def test_census_rejects_an_emptied_fixture
    # The one truncation both sides read identically: the runner registers
    # nothing from the file and the census would expect nothing, so the totals
    # fall together and no mismatch appears. Counting it as zero is what would
    # make the whole-file guarantee a lie, so the census refuses it instead.
    files = { "cases.json" => JSON.dump(FIXTURE), "emptied.json" => "[]" }
    with_fixture_tree(files) do |dir|
      assert_raises(CaseCensus::Error) { CaseCensus.non_live_case_count(dir) }
    end
  end

  def test_count_failure_accepts_agreement
    assert_nil CaseCensus.count_failure(42, 42)
  end

  def test_count_failure_names_an_over_count
    failure = CaseCensus.count_failure(43, 42)

    refute_nil failure
    assert_includes failure, "1 more than the fixtures declare"
  end

  def test_mock_mode_treats_absence_as_mock
    assert CaseCensus.mock_mode?(nil)
    assert CaseCensus.mock_mode?("mock")
    refute CaseCensus.mock_mode?("live")
    # The census is what catches this one; the filter must not run it.
    refute CaseCensus.mock_mode?("moc")
    # An explicit empty mode is not an absent one. Python defaulted on
    # falsiness and ran it.
    refute CaseCensus.mock_mode?("")
    # `mode || "mock"` defaulted this too, so Ruby alone ran it while the census
    # counted it as non-live — matching totals over a value nothing accepts.
    refute CaseCensus.mock_mode?(false)
  end
end
