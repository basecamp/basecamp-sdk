# frozen_string_literal: true

require "test_helper"

class CalendarsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_calendar
    {
      "id" => 2085958497,
      "type" => "Calendar",
      "name" => "Honcho Design Calendar",
      "color" => "blue",
      "created_at" => "2026-05-28T17:22:22.133Z",
      "updated_at" => "2026-07-20T04:05:52.374Z",
      "url" => "https://3.basecampapi.com/12345/calendars/2085958497.json",
      "app_url" => "https://3.basecamp.com/12345/calendars/2085958497",
      "schedule_url" => "https://3.basecampapi.com/12345/schedules/1069478892.json"
    }
  end

  def test_get_calendar
    stub_get("/12345/calendars/2085958497", response_body: sample_calendar)

    calendar = @account.calendars.get_calendar(calendar_id: 2085958497)

    assert_equal 2085958497, calendar["id"]
    assert_equal "blue", calendar["color"]
  end

  def test_get_calendar_raises_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/calendars/999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.calendars.get_calendar(calendar_id: 999)
    end
  end

  def test_update_calendar_sends_nested_envelope
    stub_request(:put, "https://3.basecampapi.com/12345/calendars/2085958497")
      .with(body: { "calendar" => { "color" => "green" } }.to_json)
      .to_return(status: 200, body: sample_calendar.merge("color" => "green").to_json,
                 headers: { "Content-Type" => "application/json" })

    calendar = @account.calendars.update_calendar(calendar_id: 2085958497, calendar: { "color" => "green" })

    assert_equal "green", calendar["color"]
  end

  def test_update_calendar_raises_validation_error_on_field_keyed_422
    stub_request(:put, "https://3.basecampapi.com/12345/calendars/2085958497")
      .to_return(status: 422, body: { "errors" => { "color" => [ "is not a valid color" ] } }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ValidationError) do
      @account.calendars.update_calendar(calendar_id: 2085958497, calendar: { "color" => "chartreuse" })
    end
  end
end
