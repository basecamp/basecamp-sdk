# frozen_string_literal: true

require "test_helper"

class TimesheetServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_report
    response = {
      "entries" => [
        { "id" => 1, "hours" => 8.0, "description" => "Development work" }
      ]
    }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/timesheet\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheet.report
    assert_kind_of Hash, result
    assert_kind_of Array, result["entries"]
    assert_equal 8.0, result["entries"].first["hours"]
  end

  def test_report_with_date_range
    response = { "entries" => [ { "id" => 1, "hours" => 4.0 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/timesheet\.json\?from=2024-01-01&to=2024-01-31})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheet.report(from: "2024-01-01", to: "2024-01-31")
    assert_equal 4.0, result["entries"].first["hours"]
  end

  def test_project_report
    response = { "entries" => [ { "id" => 1, "hours" => 6.0 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/timesheet\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheet.project_report(project_id: 1)
    assert_kind_of Hash, result
    assert_equal 6.0, result["entries"].first["hours"]
  end

  def test_recording_report
    response = { "entries" => [ { "id" => 1, "hours" => 2.5 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/recordings/\d+/timesheet\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheet.recording_report(project_id: 1, recording_id: 2)
    assert_kind_of Hash, result
    assert_equal 2.5, result["entries"].first["hours"]
  end
end
