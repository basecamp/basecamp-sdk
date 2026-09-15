# frozen_string_literal: true

module Basecamp
  module Services
    # A compact projection of one recording, resolved from the pointer an
    # account event feed row or a webhook carries — bucket id, recording id, and
    # the event type or recording type — through the typed read that type names.
    # Prepended onto the generated {RecordingsService} (see the +on_load+ hook in
    # +basecamp.rb+).
    #
    # It exists for consumers that must decide something about a recording
    # without paying for its full payload: an agent connector's admission step,
    # an MCP tool answering "what is this?".
    #
    # The SDK has no untyped recording read (BC3 has no such route), so the type
    # is the routing key: +comment.created+ reads a comment, +card.created+ reads
    # a card, and so on — one typed read per type. Chat lines are the exception,
    # because their read needs the Campfire id and the pointer does not carry it;
    # {#summarize} discovers the Campfire first (see the Campfire discovery
    # section below).
    #
    # This is hand-written composition over the generated services (AGENTS.md;
    # SPEC.md section 18, Appendix F). It makes no wire request of its own, and
    # it mints no operation identity: hooks see the constituent reads under their
    # own names (SPEC.md section 18 rule 3), so a +summarize+ of a comment shows
    # up as +comments.get+ and nothing else.
    module RecordingsExtensions
      # Maps the subject of an account event feed type — everything before its
      # final "." — to a read. This is the feed's catalog (bc3
      # +Event::EventType+) minus +boost+, which names no recording type and is
      # refused explicitly rather than left to fall through as unknown.
      EVENT_SUBJECTS = {
        "comment" => :comment,
        "message" => :message,
        "todo" => :todo,
        "card" => :card,
        "chat.line" => :chat_line
      }.freeze

      # Chat line subtypes (+Chat::Lines::Text+, +::RichText+, +::Code+,
      # +::Upload+, +::Integration+) all read through the same route, so they are
      # matched by prefix rather than listed.
      CHAT_LINE_TYPE_PREFIX = "Chat::Lines::"

      # Maps BC3's recording type strings to a read. It is the routing contract,
      # and it is a DELIBERATE set, not an exhaustive one: the recording types
      # the account event feed's trigger matrix names (comment, message, to-do,
      # card, chat line), plus the content and tool recordings a consumer
      # reasoning about those is likely to hold an id for.
      #
      # A type outside this set is +unknown_recording_type+ by design, whether or
      # not the SDK has an id-only read for it — timesheet entries and gauge
      # needles do, and are not routed; +Client::Reply+ and +Forward::Reply+
      # cannot be, since their reads need a parent id the pointer does not carry.
      # Widening the set is a product decision, not a gap: add the type here, its
      # projection in +read_summary+, a routing row in the native test, and a
      # case in +conformance/tests/recording_summary.json+, the fixture this
      # implements.
      RECORDING_TYPES = {
        "Comment" => :comment,
        "Message" => :message,
        "Todo" => :todo,
        "Kanban::Card" => :card,
        "Document" => :document,
        "Upload" => :upload,
        "Schedule::Entry" => :schedule_entry,
        "Question" => :question,
        "Question::Answer" => :question_answer,
        "Todolist" => :todolist,
        "Vault" => :vault,
        "Inbox::Forward" => :forward,
        "Client::Approval" => :client_approval,
        "Client::Correspondence" => :client_correspondence,
        "GoogleDocument" => :google_document,
        "CloudFile" => :cloud_file,
        "Kanban::Step" => :card_step,
        "Questionnaire" => :questionnaire,
        "Schedule" => :schedule,
        "Todoset" => :todoset,
        "Message::Board" => :message_board,
        "Kanban::Board" => :card_table,
        "Kanban::Column" => :card_column,
        "Inbox" => :inbox,
        "Chat::Transcript" => :campfire
      }.freeze

      # The chat line subtypes that carry rich text — the two that declare
      # +rich_text_attribute :content+ in BC3, and so the only two whose content
      # can hold a mention. A Text line's content is HTML-escaped on the way out
      # (+content_helper.rb+, +format_chat_line_with+), a Code line's is served
      # verbatim — a snippet that happens to contain a bc-attachment tag — and an
      # Upload line has no content.
      RICH_TEXT_CHAT_LINE_TYPES = [ "Chat::Lines::RichText", "Chat::Lines::Integration" ].freeze

      # Bounds how many Campfires one {#summarize} call tries, across both
      # discovery sources and the refresh. A project has one Campfire and a
      # handful of pings; a bucket past this bound is not a shape BC3 produces,
      # and the call reports {Basecamp::CampfireDiscoveryIncompleteError} rather
      # than calling the rest absent.
      MAX_CAMPFIRE_CANDIDATES = 50

      # Resolves a recording pointer into a compact projection through the typed
      # read its type names.
      #
      # The projection is a Hash with string keys: +id+, +status+, +type+,
      # +title+, +app_url+, +content+, +updated_at+ and +mentioned_person_ids+
      # always; +parent+, +bucket+, +creator+ and +assignees+ when the type has
      # them; +campfire_id+ for a chat line — the Campfire it was found under,
      # which is the reply destination for a chat trigger.
      #
      # @param bucket_id [Integer] the project the recording lives in. Required:
      #   it scopes the Campfire discovery for chat lines, and the read is
      #   checked against it so a pointer from one project can never resolve to a
      #   recording in another.
      # @param recording_id [Integer] the recording's id
      # @param event_type [String, nil] the account event feed type that named
      #   the recording — "comment.created", "card.assignment_changed",
      #   "chat.line.created". The segment before the action names the recording
      #   type. Used when +recording_type+ is empty.
      # @param recording_type [String, nil] the recording's own type as BC3
      #   spells it — "Comment", "Kanban::Card", "Chat::Lines::Text". When set it
      #   takes precedence over +event_type+, being the more exact of the two.
      # @return [Hash] the projection
      # @raise [Basecamp::UsageError] when the pointer is incomplete
      # @raise [Basecamp::RecordingRoutingError] before any request, when the
      #   pointer names no routable type
      # @raise [Basecamp::UnresolvedRecordingError] when every visible Campfire
      #   answered 404 for a chat line
      # @raise [Basecamp::CampfireDiscoveryIncompleteError] when candidates were
      #   left unsearched
      # @raise [Basecamp::BucketMismatchError] when the read returned a recording
      #   from another bucket
      # @raise [Basecamp::Error] the read's own error otherwise — a 404 is a
      #   {Basecamp::NotFoundError}, as from the typed read itself
      def summarize(bucket_id:, recording_id:, event_type: nil, recording_type: nil)
        bucket_id = Ids.integer(bucket_id, "bucket id")
        recording_id = Ids.integer(recording_id, "recording id")
        unless bucket_id.positive? && recording_id.positive?
          raise UsageError.new("bucket id and recording id are required")
        end

        kind = route_recording(event_type: event_type, recording_type: recording_type)
        summary = read_summary(kind, bucket_id: bucket_id, recording_id: recording_id)

        # A malformed "bucket" member is read as absent rather than raising:
        # Hash#dig through a non-Hash is a TypeError, and a bad projection must
        # not turn into an exception class no caller expects.
        read_bucket_id = summary["bucket"].is_a?(Hash) ? summary["bucket"]["id"].to_i : 0
        if read_bucket_id.positive? && read_bucket_id != bucket_id
          raise BucketMismatchError.new(
            bucket_id: bucket_id, actual_bucket_id: read_bucket_id, recording_id: recording_id
          )
        end

        summary
      end

      # The recording types {#summarize} routes by +recording_type+, sorted, with
      # the +Chat::Lines+ subtypes represented by their shared prefix. The set is
      # deliberate rather than exhaustive — see {RECORDING_TYPES} — and any other
      # type is +unknown_recording_type+ by design.
      #
      # @return [Array<String>]
      def summarizable_recording_types
        (RECORDING_TYPES.keys + [ "#{CHAT_LINE_TYPE_PREFIX}*" ]).sort
      end

      # The account event feed subjects {#summarize} routes by +event_type+ — an
      # event type is "<subject>.<action>", and any action on a listed subject
      # routes to that subject's read — sorted. "boost" is absent on purpose.
      #
      # @return [Array<String>]
      def summarizable_event_types
        EVENT_SUBJECTS.keys.map { |subject| "#{subject}.*" }.sort
      end

      private

      # Picks the read for a pointer. +recording_type+ wins when set.
      def route_recording(event_type:, recording_type:)
        type = recording_type.to_s.strip
        unless type.empty?
          return :chat_line if type.start_with?(CHAT_LINE_TYPE_PREFIX)

          kind = RECORDING_TYPES[type]
          return kind if kind

          raise RecordingRoutingError.unknown_recording_type(type)
        end

        subject_type = event_type.to_s.strip
        raise RecordingRoutingError.unknown_recording_type(subject_type) if subject_type.empty?

        # A feed type is "<subject>.<action>"; the subject names the recording
        # type. A string with no action is not a feed type and is not routed.
        separator = subject_type.rindex(".")
        if separator.nil? || !separator.positive? || separator == subject_type.length - 1
          raise RecordingRoutingError.unknown_recording_type(subject_type)
        end

        subject = subject_type[0, separator]
        raise RecordingRoutingError.no_recording_type(subject_type) if subject == "boost"

        EVENT_SUBJECTS[subject] || raise(RecordingRoutingError.unknown_recording_type(subject_type))
      end

      # Performs the one typed read a kind names and projects it. Every branch is
      # a single public generated method: no path is built here and no verb is
      # chosen here.
      def read_summary(kind, bucket_id:, recording_id:)
        id = recording_id

        case kind
        when :comment
          project(@client.comments.get(comment_id: id))
        when :message
          record = @client.messages.get(message_id: id)
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :todo
          # A to-do's content is its plain title; the rich text — where mentions
          # live — is the description.
          record = @client.todos.get(todo_id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["content"]),
            content: record["description"],
            assignees: record["assignees"]
          )
        when :card
          record = @client.cards.get(card_id: id)
          project(
            record,
            content: first_non_empty(record["content"], record["description"]),
            assignees: record["assignees"]
          )
        when :chat_line
          summarize_chat_line(bucket_id: bucket_id, line_id: id)
        when :document
          project(@client.documents.get(document_id: id))
        when :upload
          record = @client.uploads.get(upload_id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["filename"]),
            content: record["description"]
          )
        when :schedule_entry
          record = @client.schedules.get_entry(entry_id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["summary"]),
            content: record["description"]
          )
        when :question
          project(@client.checkins.get_question(question_id: id), content: "")
        when :question_answer
          project(@client.checkins.get_answer(answer_id: id))
        when :todolist
          record = @client.todolists.get(id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["name"]),
            content: record["description"]
          )
        when :vault
          project(@client.vaults.get(vault_id: id), content: "")
        when :forward
          record = @client.forwards.get(forward_id: id)
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :client_approval
          record = @client.client_approvals.get(approval_id: id)
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :client_correspondence
          record = @client.client_correspondences.get(correspondence_id: id)
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :google_document
          record = @client.google_documents.get_google_document(google_document_id: id)
          project(record, content: record["description"])
        when :cloud_file
          record = @client.cloud_files.get_cloud_file(cloud_file_id: id)
          project(record, content: record["description"])
        when :card_step
          record = @client.card_steps.get(step_id: id)
          project(record, content: "", assignees: record["assignees"])
        when :questionnaire
          record = @client.checkins.get_questionnaire(questionnaire_id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["name"]),
            content: "",
            parent: nil
          )
        when :schedule
          project(@client.schedules.get(schedule_id: id), content: "", parent: nil)
        when :todoset
          record = @client.todosets.get(todoset_id: id)
          project(
            record,
            title: first_non_empty(record["title"], record["name"]),
            content: "",
            parent: nil
          )
        when :message_board
          project(@client.message_boards.get(board_id: id), content: "", parent: nil)
        when :card_table
          project(@client.card_tables.get(card_table_id: id), content: "", parent: nil)
        when :card_column
          record = @client.card_columns.get(column_id: id)
          project(record, content: record["description"])
        when :inbox
          project(@client.forwards.get_inbox(inbox_id: id), content: "", parent: nil)
        when :campfire
          project(@client.campfires.get(campfire_id: id), content: "", parent: nil)
        else
          # Unreachable while every value in RECORDING_TYPES and EVENT_SUBJECTS
          # has a branch. It is here so that adding a routing row and forgetting
          # the projection refuses the pointer by name rather than returning nil
          # and failing somewhere else.
          raise RecordingRoutingError.unknown_recording_type(kind)
        end
      end

      # Projects one read recording into the summary shape.
      #
      # Absent members are omitted rather than carried as nil, so a consumer
      # reads the same shape whatever the type: a comment has no assignees, a
      # vault no content. +mentioned_person_ids+ is the exception — always
      # present, so a consumer reads [] rather than a missing key.
      def project(record, title: :default, content: :default, assignees: nil, parent: :default)
        title = record["title"] if title == :default
        content = record["content"] if content == :default
        parent = record["parent"] if parent == :default
        content = content.to_s

        summary = {
          "id" => record["id"],
          "status" => record["status"],
          "type" => record["type"],
          "title" => title.to_s,
          "app_url" => record["app_url"].to_s,
          "parent" => parent,
          "bucket" => record["bucket"],
          "creator" => record["creator"],
          "assignees" => assignees,
          "mentioned_person_ids" => Mentions.mentioned_person_ids(content),
          "content" => content,
          "updated_at" => record["updated_at"]
        }
        summary.delete("parent") if parent.nil?
        summary.delete("bucket") if summary["bucket"].nil?
        summary.delete("creator") if summary["creator"].nil?
        # A malformed "assignees" member is read as absent for the same reason a
        # malformed "bucket" is: a bad projection must not become an exception
        # class no caller expects.
        summary.delete("assignees") unless assignees.is_a?(Array) && !assignees.empty?
        summary
      end

      def first_non_empty(*values)
        values.find { |value| !value.to_s.empty? }.to_s
      end

      # Resolves the line, then projects it with the Campfire it was found under.
      def summarize_chat_line(bucket_id:, line_id:)
        line, campfire_id = resolve_chat_line(bucket_id: bucket_id, line_id: line_id)
        summary = project(line)
        unless RICH_TEXT_CHAT_LINE_TYPES.include?(line["type"])
          # A plain-text or code line's content is text BC3 never read as markup,
          # so a literal "<bc-attachment>" in it mentions nobody.
          summary["mentioned_person_ids"] = []
        end
        summary["campfire_id"] = campfire_id
        summary
      end

      # Campfire discovery for chat lines.
      #
      # The loop tries the line under each candidate until one answers, within
      # one total budget of {MAX_CAMPFIRE_CANDIDATES} per call.
      #
      # Two failure shapes are kept apart on purpose. A candidate that answers
      # anything but 404 — 401, 403, 5xx, a network error — stops the loop and is
      # raised as that error: the read failed, and trying the next Campfire would
      # only hide it. A 404 means "not here", so the loop moves on. Only when
      # every candidate said "not here" is the line unresolved
      # ({Basecamp::UnresolvedRecordingError}) — and before concluding that, the
      # cached sources are refreshed (subject to the floor) so a Campfire created
      # after the cache filled is tried too. Discovery that could not be
      # completed — a listing cut off at its cap, a bucket with more candidates
      # than the budget — is {Basecamp::CampfireDiscoveryIncompleteError}, never
      # "unresolved": nothing unsearched is ever reported absent.
      #
      # What HTTP cannot tell apart: BC3 answers 404 both for a line that is not
      # in a Campfire and for a Campfire the caller may no longer see. See
      # {Basecamp::UnresolvedRecordingError} for what that means for a consumer.
      def resolve_chat_line(bucket_id:, line_id:)
        index = @client.campfire_index
        account_id = @client.account_id
        search = ChatLineSearch.new(campfires: @client.campfires, line_id: line_id)

        # Pass 1: what the sources already hold — the dock (read if it must be),
        # then the listing only if it is cached. A listing fetch is the
        # expensive, slow request, and it is not made until the dock — including
        # its refresh — has had its say, so a listing that is down, over its cap,
        # or stalled never stands between a project's line and the one project
        # read that finds it.
        dock = index.dock_campfires(account_id: account_id, bucket_id: bucket_id) do
          dock_campfire_ids(bucket_id)
        end
        found = search.try(dock.ids)
        return found if found

        listed = index.cached_listed_campfires(account_id: account_id, bucket_id: bucket_id)
        list_cached = !listed.nil?
        if list_cached
          found = search.try(listed.ids)
          return found if found
        end

        # Pass 2: re-read the dock if it was served from cache (the floor may
        # decline), then fetch or refresh the listing. Whatever comes back is the
        # current snapshot of that source, whoever loaded it — another caller may
        # have populated or refreshed it in the meantime — so it always replaces
        # the pass-1 one; "refreshed" is whether a source the conclusion had
        # consulted is now newer than when it was consulted.
        #
        # Not when the budget is already spent: a re-read could return no
        # candidate this call may try, so it would cost a request that cannot
        # help — and a failure on it would replace the deterministic "incomplete"
        # verdict with a transient error a consumer retries forever.
        refreshed = false
        raise budget_exhausted(bucket_id, line_id) if search.skipped?

        if dock.cached?
          again = index.dock_campfires(account_id: account_id, bucket_id: bucket_id, refresh: true) do
            dock_campfire_ids(bucket_id)
          end
          refreshed = true if again.fetched > dock.fetched || !again.cached?
          dock = again
          found = search.try(dock.ids)
          return found if found
        end

        raise budget_exhausted(bucket_id, line_id) if search.skipped?

        begin
          again = index.listed_campfires(
            account_id: account_id, bucket_id: bucket_id, refresh: list_cached
          ) { listed_campfire_ids_by_bucket }
        rescue CampfireIndex::ListingOverflow => e
          raise CampfireDiscoveryIncompleteError.new(
            bucket_id: bucket_id, recording_id: line_id, reason: e.message
          )
        end
        refreshed = true if list_cached && (again.fetched > listed.fetched || !again.cached?)
        listed = again
        found = search.try(listed.ids)
        return found if found

        raise budget_exhausted(bucket_id, line_id) if search.skipped?

        stale = if refreshed
          search.tried.reject { |id| dock.ids.include?(id) || listed.ids.include?(id) }
        else
          []
        end
        raise UnresolvedRecordingError.new(
          bucket_id: bucket_id,
          recording_id: line_id,
          campfire_ids: search.tried,
          refreshed: refreshed,
          stale_campfire_ids: stale
        )
      end

      # The Campfire ids a bucket's project dock names. A bucket that is not a
      # project (a 404 on the project read) has none; any other failure of the
      # read is raised.
      def dock_campfire_ids(bucket_id)
        project = begin
          @client.projects.get(project_id: bucket_id)
        rescue NotFoundError
          return []
        end

        dock = project["dock"]
        return [] unless dock.is_a?(Array)

        dock.filter_map do |item|
          next unless item.is_a?(Hash) && item["name"] == "chat"

          id = item["id"].to_i
          id.positive? ? id : nil
        end
      end

      # The account-wide Campfire listing, grouped by bucket. BC3 has no
      # per-bucket listing, so the whole account is read and filtered. A listing
      # that overflows the cap is not cached and is reported as incomplete.
      def listed_campfire_ids_by_bucket
        campfires = @client.campfires.list(max_items: CampfireIndex::MAX_LISTING)
        listed = campfires.to_a
        if campfires.meta.truncated?
          raise CampfireIndex::ListingOverflow,
            "campfire listing exceeds #{CampfireIndex::MAX_LISTING}"
        end

        listed.each_with_object({}) do |campfire, by_bucket|
          bucket = campfire["bucket"]
          next unless bucket.is_a?(Hash)

          bucket_id = bucket["id"].to_i
          next unless bucket_id.positive?

          # Normalized exactly as the dock's ids are. Otherwise a listing id
          # that arrived as a string could never match a dock-sourced integer in
          # the search's "already tried" set, and would spend budget twice.
          campfire_id = campfire["id"].to_i
          next unless campfire_id.positive?

          (by_bucket[bucket_id] ||= []) << campfire_id
        end
      end

      def budget_exhausted(bucket_id, line_id)
        CampfireDiscoveryIncompleteError.new(
          bucket_id: bucket_id,
          recording_id: line_id,
          reason: "more than #{MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket"
        )
      end

      # One {#summarize} call's discovery state: the candidates already tried,
      # what is left of the budget, and whether a candidate was left untried for
      # want of it.
      class ChatLineSearch
        # @return [Array<Integer>] candidates that answered 404, in order
        attr_reader :tried

        def initialize(campfires:, line_id:, budget: MAX_CAMPFIRE_CANDIDATES)
          @campfires = campfires
          @line_id = line_id
          @budget = budget
          @tried = []
          @skipped = false
        end

        # Reads the line under each candidate not yet tried.
        #
        # @param candidates [Array<Integer>]
        # @return [Array(Hash, Integer), nil] the line and its Campfire on a hit;
        #   nil on a miss, with the candidates recorded in {#tried}
        # @raise [Basecamp::Error] any answer but 404, as that read's own error
        def try(candidates)
          Array(candidates).each do |campfire_id|
            next if @tried.include?(campfire_id)

            if @budget <= 0
              @skipped = true
              return nil
            end

            @budget -= 1
            begin
              line = @campfires.get_line(campfire_id: campfire_id, line_id: @line_id)
              return [ line, campfire_id ]
            rescue NotFoundError
              @tried << campfire_id
            end
          end
          nil
        end

        # @return [Boolean] whether a candidate was left untried for want of budget
        def skipped?
          @skipped
        end
      end
    end
  end
end
