# frozen_string_literal: true

module Basecamp
  # Move or copy a recording and wait for Basecamp to finish it (SPEC.md section
  # 18, Appendix F "Filing Waits"), ported from +go/pkg/basecamp/filing_wait.go+.
  # The start is sent once and never resent, because each copy request makes
  # another copy; GetFiling is then polled with backoff until the filing
  # completes or fails, or the wait ends.
  module FilingWait
    # The kind of container a move or copy files a recording into, by recordable
    # type. Every value but "todolist" is a tool's dock name: the destination
    # project's container is its enabled dock item of that name. A to-do list is
    # not on the dock; list the destination project's to-do lists under its
    # todoset.
    CONTAINERS = {
      "Kanban::Card" => "kanban_board",
      "Kanban::Column" => "kanban_board",
      "Message" => "message_board",
      "Document" => "vault",
      "Upload" => "vault",
      "CloudFile" => "vault",
      "GoogleDocument" => "vault",
      "Vault" => "vault",
      "Schedule::Entry" => "schedule",
      "Question" => "questionnaire",
      "Inbox::Forward" => "inbox",
      "Todolist" => "todoset",
      "Todo" => "todolist"
    }.freeze

    DEFAULT_WAIT = 60.0
    FIRST_POLL = 0.25
    MAX_POLL = 2.0

    module_function

    # The kind of container a move or copy files a recording into, from its
    # recordable type and its parent's type as the recording's own read reports
    # them ("" when it has no parent). A card lands in Triage, or in Done if it
    # is completed. Nil for everything else: a recording that moves only with
    # its parent, such as a comment or the columns every card table has, and a
    # dock tool, such as a docked Vault, which is filed by
    # +destination_project_id+.
    #
    # @return [String, nil]
    def container_for(recording_type, parent_type)
      return "todolist" if recording_type == "Todolist" && parent_type == "Todolist"
      return nil if recording_type == "Vault" && parent_type != "Vault"

      CONTAINERS[recording_type]
    end

    # Whether a destination of this kind can take +parent_id+, a folder inside it.
    def takes_folders?(container)
      container == "vault"
    end

    # Starts a filing once and waits for it; the body of +move_and_wait+ and
    # +copy_and_wait+.
    def run(start:, get_filing:, timeout: nil)
      begin
        filing = start.call
      rescue Basecamp::Error => e
        raise FilingRefusedError, "filing refused: #{e.message}" if refused?(e)

        raise FilingMayHaveStartedError, "filing may have started: #{e.message}"
      rescue StandardError => e
        raise FilingMayHaveStartedError, "filing may have started: #{e.message}"
      end
      raise FilingMayHaveStartedError, "filing may have started: unreadable answer #{filing.inspect}" unless filing?(filing)

      deadline = monotonic + (timeout.to_f.positive? ? timeout.to_f : DEFAULT_WAIT)
      delay = FIRST_POLL
      loop do
        return filing if filing["status"] == "completed" && filing["destination_recording"]
        raise FilingFailedError.new(filing) if filing["status"] == "failed"

        remaining = deadline - monotonic
        if remaining <= delay
          sleep([ remaining, 0 ].max)
          raise FilingUnsettledError.new(filing)
        end
        sleep(delay)
        begin
          latest = get_filing.call(filing["id"])
        rescue StandardError
          raise FilingUnsettledError.new(filing)
        end
        raise FilingUnsettledError.new(filing) unless filing?(latest, filing["id"])

        filing = latest
        delay = [ 2 * delay, MAX_POLL ].min
      end
    end

    # The generated methods return the body undecoded, so a malformed one would
    # otherwise replace the filing the caller needs to poll again.
    def filing?(value, filing_id = nil)
      value.is_a?(Hash) && value["id"].is_a?(Integer) && value["status"].is_a?(String) &&
        (filing_id.nil? || value["id"] == filing_id)
    end

    def refused?(error)
      status = error.http_status.to_i
      (status >= 400 && status < 500) || [ ErrorCode::USAGE, ErrorCode::RATE_LIMIT ].include?(error.code)
    end

    def monotonic
      Process.clock_gettime(Process::CLOCK_MONOTONIC)
    end
  end
end
