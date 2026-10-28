# frozen_string_literal: true

require "test_helper"

class RecordingsFilingWaitTest < Minitest::Test
  include TestHelper

  BASE = "https://3.basecampapi.com/12345"

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def filing(status, destination: false)
    body = { "id" => 4, "status" => status, "type" => "Copy", "source_recording_id" => 456, "url" => "#{BASE}/filings/4.json" }
    body["destination_recording"] = { "id" => 900, "type" => "Message", "url" => "u", "app_url" => "a" } if destination
    body
  end

  def json(status, body)
    { status: status, body: body.to_json, headers: { "Content-Type" => "application/json" } }
  end

  def test_polls_until_completed
    start = stub_request(:post, "#{BASE}/recordings/456/copies.json").to_return(json(201, filing("pending")))
    stub_request(:get, "#{BASE}/filings/4").to_return(json(200, filing("processing")), json(200, filing("completed", destination: true)))

    result = @account.recordings.copy_and_wait(recording_id: 456, filing_destination_id: 789)

    assert_equal 900, result["destination_recording"]["id"]
    assert_requested start, times: 1
  end

  def test_failed
    stub_request(:post, "#{BASE}/recordings/456/moves.json").to_return(json(201, filing("pending")))
    stub_request(:get, "#{BASE}/filings/4").to_return(json(200, filing("failed")))

    error = assert_raises(Basecamp::FilingFailedError) do
      @account.recordings.move_and_wait(recording_id: 456, filing_destination_id: 789)
    end
    assert_equal 4, error.filing["id"]
  end

  def test_wait_ends_while_processing
    start = stub_request(:post, "#{BASE}/recordings/456/copies.json").to_return(json(201, filing("pending")))
    stub_request(:get, "#{BASE}/filings/4").to_return(json(200, filing("processing")))

    error = assert_raises(Basecamp::FilingUnsettledError) do
      @account.recordings.copy_and_wait(recording_id: 456, filing_destination_id: 789, timeout: 0.6)
    end
    assert_equal "processing", error.filing["status"]
    assert_equal false, error.is_a?(Basecamp::Error)
    assert_requested start, times: 1
  end

  def test_poll_failure_is_unsettled
    stub_request(:post, "#{BASE}/recordings/456/copies.json").to_return(json(201, filing("pending")))
    stub_request(:get, "#{BASE}/filings/4").to_return(json(404, { "error" => "Not Found" }))

    error = assert_raises(Basecamp::FilingUnsettledError) do
      @account.recordings.copy_and_wait(recording_id: 456, filing_destination_id: 789)
    end
    assert_kind_of Basecamp::NotFoundError, error.cause
  end

  def test_malformed_poll_keeps_the_filing
    stub_request(:post, "#{BASE}/recordings/456/copies.json").to_return(json(201, filing("pending")))
    stub_request(:get, "#{BASE}/filings/4").to_return(json(200, []))

    error = assert_raises(Basecamp::FilingUnsettledError) do
      @account.recordings.copy_and_wait(recording_id: 456, filing_destination_id: 789)
    end
    assert_equal 4, error.filing["id"]
  end

  def test_start_refused
    [ 403, 404, 422 ].each do |status|
      WebMock.reset!
      start = stub_request(:post, "#{BASE}/recordings/456/moves.json").to_return(json(status, {}))

      error = assert_raises(Basecamp::FilingRefusedError) do
        @account.recordings.move_and_wait(recording_id: 456, filing_destination_id: 789)
      end
      assert_equal status, error.cause.http_status
      assert_requested start, times: 1
    end
  end

  def test_start_never_retried
    [ 500, 503 ].each do |status|
      WebMock.reset!
      start = stub_request(:post, "#{BASE}/recordings/456/copies.json").to_return(json(status, {}))

      assert_raises(Basecamp::FilingMayHaveStartedError) do
        @account.recordings.copy_and_wait(recording_id: 456, filing_destination_id: 789)
      end
      assert_requested start, times: 1
    end
  end

  def test_container_for
    {
      [ "Kanban::Card", "Kanban::Triage" ] => "kanban_board",
      [ "Kanban::Column", "Kanban::Board" ] => "kanban_board",
      [ "Message", "Message::Board" ] => "message_board",
      [ "Document", "Vault" ] => "vault",
      [ "Upload", "Vault" ] => "vault",
      [ "CloudFile", "Vault" ] => "vault",
      [ "GoogleDocument", "Vault" ] => "vault",
      [ "Vault", "Vault" ] => "vault",
      [ "Schedule::Entry", "Schedule" ] => "schedule",
      [ "Question", "Questionnaire" ] => "questionnaire",
      [ "Inbox::Forward", "Inbox" ] => "inbox",
      [ "Todolist", "Todoset" ] => "todoset",
      [ "Todolist", "Todolist" ] => "todolist",
      [ "Todo", "Todolist" ] => "todolist",
      [ "Vault", "" ] => nil,
      [ "Message::Board", "" ] => nil,
      [ "Kanban::Triage", "Kanban::Board" ] => nil,
      [ "Comment", "Message" ] => nil,
      [ "Question::Answer", "Question" ] => nil
    }.each do |(recording, parent), want|
      got = Basecamp::FilingWait.container_for(recording, parent)
      if want.nil?
        assert_nil got, "#{recording} under #{parent}"
      else
        assert_equal want, got, "#{recording} under #{parent}"
        assert_equal(want == "vault", Basecamp::FilingWait.takes_folders?(got))
      end
    end
  end
end
