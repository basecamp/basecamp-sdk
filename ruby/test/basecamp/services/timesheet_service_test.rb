# frozen_string_literal: true

# Tests for the TimesheetsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Service accessor: timesheets (plural) not timesheet
# - Method names: for_project(), for_recording() (not project_report, recording_report)

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

    result = @account.timesheets.report
    assert_kind_of Hash, result
    assert_kind_of Array, result["entries"]
    assert_equal 8.0, result["entries"].first["hours"]
  end

  def test_report_with_date_range
    response = { "entries" => [ { "id" => 1, "hours" => 4.0 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/reports/timesheet\.json\?from=2024-01-01&to=2024-01-31})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheets.report(from: "2024-01-01", to: "2024-01-31")
    assert_equal 4.0, result["entries"].first["hours"]
  end

  def test_for_project
    response = { "entries" => [ { "id" => 1, "hours" => 6.0 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/projects/\d+/timesheet\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheets.for_project(project_id: 1)
    assert_kind_of Hash, result
    assert_equal 6.0, result["entries"].first["hours"]
  end

  def test_for_recording
    response = { "entries" => [ { "id" => 1, "hours" => 2.5 } ] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/projects/\d+/recordings/\d+/timesheet\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.timesheets.for_recording(project_id: 1, recording_id: 2)
    assert_kind_of Hash, result
    assert_equal 2.5, result["entries"].first["hours"]
  end
end
