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

      # What a malformed field was being read FOR. Two sites, two hints: a hint
      # that sends the reader to the wrong half of the composite is worse than a
      # shorter one.
      RECORDING_HINT =
        "The recording summary reads this field to decide what the recording is and which " \
        "project it belongs to, so a value of the wrong type cannot be used."

      DISCOVERY_HINT =
        "The recording summary reads this field to decide which Campfires a chat line could " \
        "be in, so a value of the wrong type cannot be used."

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
      # @raise [Basecamp::ApiError] non-retryable, when a successful response
      #   carries a value the reference's decoder would have refused — a body
      #   that is not an object, a bucket that is not one, an id that is not an
      #   integer. Minted by this composite, not by the read.
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

        read_bucket_id = read_bucket_id(summary)
        # NON-ZERO, not positive. The reference compares whenever its bucket id
        # is not the zero value, so a negative one is a mismatch there and was
        # returned as a match here — this is the one check standing between a
        # pointer and a recording in another project, and it was the only place
        # the earlier sweep of this same defect did not reach.
        if !read_bucket_id.zero? && read_bucket_id != bucket_id
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

      # The read's body, which has to be an object before a single field is read
      # off it.
      #
      # This is the guard one level up from the field checks, and it is the one
      # that matters most, because a String body does not fail — it PASSES. In
      # Ruby <tt>"scalar"["bucket"]</tt> is a substring search that quietly
      # answers nil, so the bucket reads as absent, the cross-bucket comparison
      # never runs, and a recording from another project is returned. That is
      # the same fail-open this composite has now produced from three
      # directions; the other two were a negative bucket id and a non-object
      # bucket. An Array or a number raises TypeError instead, and nil raises
      # NoMethodError — native exceptions out of a public method, where the
      # reference has a decode failure.
      #
      # {Basecamp::Services::MergeSafe#require_hash} describes this same hazard
      # in the same words; the message differs only because the escape it names
      # belongs to the merge-safe writes.
      #
      # NULL IS NOT MALFORMED, and the first version of this guard had that
      # wrong — it said the reference has a decode failure for nil, and the
      # reference has no such thing. Measured: json.Unmarshal of `null` into a
      # struct returns no error and leaves it zero, everywhere and at every
      # depth, so a null body is a zero-valued summary there. An empty object
      # is the same zero: every field reads as absent, and the bucket
      # comparison is skipped exactly as it is for a zero bucket id.
      def read_record(record)
        return record if record.is_a?(Hash)
        return {} if record.nil?

        raise malformed_response("the read returned #{MergeSafe.describe(record)}, not a recording object")
      end

      # The bucket the read came back in, or 0 when it carries none.
      #
      # An absent or null "bucket" is genuinely none: the reference holds a
      # pointer there and reads its zero value. ANYTHING ELSE that is not an
      # object — a number, a string, an array — is a decode failure in the
      # reference, which fails the read rather than reaching the comparison, and
      # so is a malformed response here. Reading it as "none" would skip the
      # comparison, which is how a projection from another project would have
      # been returned.
      def read_bucket_id(summary)
        bucket = summary["bucket"]
        return 0 if bucket.nil?

        unless bucket.is_a?(Hash)
          raise malformed_response("the recording's \"bucket\" is #{MergeSafe.describe(bucket)}, not an object")
        end

        id = Ids.from_wire(bucket["id"])
        return id unless id.nil?

        raise malformed_response("the recording's bucket id is #{MergeSafe.describe(bucket["id"])}, not an integer")
      end

      # The error for a body this composite cannot read.
      #
      # ApiError and not UsageError, non-retryable, for the reason
      # {Basecamp::Services::MergeSafe} gives: the value arrived in a successful
      # response, nothing the caller passed is at fault, and re-requesting
      # cannot repair it. The reference gets this refusal from its decoder; this
      # tier has no decoder, so it is explicit.
      # The hint names what the field is actually read FOR, so the two sites get
      # two hints. Attaching the recording one to a dock item said the value
      # decided "which project it belongs to" when it decides which Campfire to
      # search — a hint that sends the reader to the wrong half of the composite
      # is worse than a shorter one.
      def malformed_response(message, hint: RECORDING_HINT)
        MergeSafe.malformed(message, hint)
      end

      # Picks the read for a pointer. +recording_type+ wins when set.
      #
      # Matched on BYTES. A Go string carries arbitrary bytes, so the reference
      # routes "comment.\xFF" on its "comment" subject like any other; Ruby's
      # String#strip validates the encoding and raised
      # Encoding::CompatibilityError straight out of the public method for both
      # arguments. The tables are ASCII, and an ASCII-only binary string is
      # eql? to its text twin, so the lookups are unaffected.
      def route_recording(event_type:, recording_type:)
        type = recording_type.to_s.b.strip
        unless type.empty?
          return :chat_line if type.start_with?(CHAT_LINE_TYPE_PREFIX)

          kind = RECORDING_TYPES[type]
          return kind if kind

          raise RecordingRoutingError.unknown_recording_type(type)
        end

        subject_type = event_type.to_s.b.strip
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
          record = read_record(@client.messages.get(message_id: id))
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :todo
          # A to-do's content is its plain title; the rich text — where mentions
          # live — is the description.
          record = read_record(@client.todos.get(todo_id: id))
          project(
            record,
            title: first_non_empty(record["title"], record["content"]),
            content: record["description"],
            assignees: record["assignees"]
          )
        when :card
          record = read_record(@client.cards.get(card_id: id))
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
          record = read_record(@client.uploads.get(upload_id: id))
          project(
            record,
            title: first_non_empty(record["title"], record["filename"]),
            content: record["description"]
          )
        when :schedule_entry
          record = read_record(@client.schedules.get_entry(entry_id: id))
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
          record = read_record(@client.todolists.get(id: id))
          project(
            record,
            title: first_non_empty(record["title"], record["name"]),
            content: record["description"]
          )
        when :vault
          project(@client.vaults.get(vault_id: id), content: "")
        when :forward
          record = read_record(@client.forwards.get(forward_id: id))
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :client_approval
          record = read_record(@client.client_approvals.get(approval_id: id))
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :client_correspondence
          record = read_record(@client.client_correspondences.get(correspondence_id: id))
          project(record, title: first_non_empty(record["title"], record["subject"]))
        when :google_document
          record = read_record(@client.google_documents.get_google_document(google_document_id: id))
          project(record, content: record["description"])
        when :cloud_file
          record = read_record(@client.cloud_files.get_cloud_file(cloud_file_id: id))
          project(record, content: record["description"])
        when :card_step
          record = read_record(@client.card_steps.get(step_id: id))
          project(record, content: "", assignees: record["assignees"])
        when :questionnaire
          record = read_record(@client.checkins.get_questionnaire(questionnaire_id: id))
          project(
            record,
            title: first_non_empty(record["title"], record["name"]),
            content: "",
            parent: nil
          )
        when :schedule
          project(@client.schedules.get(schedule_id: id), content: "", parent: nil)
        when :todoset
          record = read_record(@client.todosets.get(todoset_id: id))
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
          record = read_record(@client.card_columns.get(column_id: id))
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
      # Builds the projection.
      #
      # WHERE THIS PORT'S TYPE CHECKING STOPS, stated here because the edge is a
      # decision rather than an oversight. The reference decodes the whole body
      # into a struct, so any field of the wrong type fails its read. This tier
      # has no decoder, and reproducing one field by field would be writing a
      # second decoder by hand against a spec that moves.
      #
      # So the rule is: this composite refuses what would make it ACT wrongly,
      # and passes through what it merely REPORTS — but that line moved three
      # times, each time because a fix made this composite read a field it had
      # only been reporting, so it is now drawn where the REFERENCE draws it
      # instead of where this port happened to need it.
      #
      # TYPED, because the reference holds a plain string and a value of another
      # type is a decode failure there: "status", "type", "title", "content",
      # "app_url". Null normalizes to "" at every one of them, which is what its
      # decoder does. Also refused, for reasons of their own: the body envelope,
      # the bucket and its id (they decide whether the recording is in the
      # caller's project), the dock and listing entries with their ids and names
      # (they decide which Campfires get searched), and "assignees" (it decides
      # whether the key appears at all).
      #
      # Also typed: "id", which the reference holds as a plain 64-bit integer.
      # An earlier version of this paragraph listed it as passed through and
      # called it the place where reproducing the decoder begins. That was
      # wrong twice over — ten shapes reached the projection verbatim that the
      # reference refuses, and the check was already written, four times, on the
      # sibling id fields in this same file.
      #
      # PASSED THROUGH — "parent", "bucket" and "creator", which are nested
      # objects, and "updated_at". The three objects are where reproducing the
      # decoder would actually begin: validating them means writing the type the
      # generated layer deliberately does not have. What IS honoured for them is
      # the reference's emptiness rule — it builds each only when it has an id
      # or a name, so an empty object leaves the key out rather than appearing
      # as "{}". "updated_at" is a deliberate divergence rather than a gap: the
      # reference parses an instant and every port here keeps the API's own
      # string, which Appendix F records.
      #
      # "bucket" appears in both lists above for a reason worth stating rather
      # than tidying: its OBJECT is passed through to the caller, and its ID is
      # read by the cross-bucket check. It is the one member that is both
      # reported and interpreted, which is exactly why it has been the source of
      # four separate defects on this branch.
      #
      # The rule that produced three rounds of findings, stated so the next
      # person does not rediscover it: when a change makes this composite READ a
      # field it used to only report, the field moves into the typed set and
      # this paragraph has to move with it. A
      # malformed one of those reaches the caller as it arrived, where the
      # reference would have failed the read.
      #
      # Checking a few more of them would not close that gap; it would only move
      # the edge somewhere less defensible and make the next reader think the
      # projection is validated. Widening it means validating the WHOLE
      # projection, and that is a decoder — a different change, deliberately not
      # this one.
      def project(record, title: :default, content: :default, assignees: nil, parent: :default)
        record = read_record(record)
        title = record["title"] if title == :default
        content = record["content"] if content == :default
        parent = record["parent"] if parent == :default
        # CONTENT is refused rather than coerced, because it is the one member
        # of the projection this composite INTERPRETS: mentioned_person_ids is
        # derived from it. to_s turned an array or a hash into its Ruby
        # rendering and then scanned that for mentions, which is reading a body
        # the reference would have failed to decode. Title goes through the same
        # check because first_non_empty reaches both.
        content = read_text(content, "content")
        title = read_text(title, "title")

        summary = {
          "id" => read_id(record["id"]),
          "status" => read_text(record["status"], "status"),
          # Typed, not passed through, because the chat-line route READS this to
          # decide whether a line's content can carry a mention. The reference
          # holds a plain string, so an array or an object there is a decode
          # failure — and without this a line whose type was an object came back
          # successfully with its mentions silently cleared.
          "type" => read_text(record["type"], "type"),
          "title" => title.to_s,
          "app_url" => read_text(record["app_url"], "app_url"),
          "parent" => parent,
          "bucket" => record["bucket"],
          "creator" => record["creator"],
          "assignees" => assignees,
          "mentioned_person_ids" => Mentions.mentioned_person_ids(content),
          "content" => content,
          "updated_at" => record["updated_at"]
        }
        # EMPTY, not merely absent. The reference builds each of these only when
        # it has something in it — `if Id != 0 || Name != ""` for a bucket, the
        # same shape for a parent and a creator — so a `{}` in the response
        # leaves the field nil there and `omitempty` drops the key. Ruby emitted
        # the empty object, so a caller testing `summary.key?("bucket")` got a
        # different answer from the contract's.
        summary.delete("parent") unless keep_member?(parent, "title")
        summary.delete("bucket") unless keep_member?(summary["bucket"], "name")
        summary.delete("creator") unless keep_member?(summary["creator"], "name")
        # Absent or empty is genuinely nothing to report — the reference's
        # +omitempty+ leaves an empty slice out of its summary too, so the key
        # goes. Anything else that is not an array of objects is a decode
        # failure there and fails the read here.
        #
        # This comment used to say a malformed "assignees" was read as absent
        # "for the same reason a malformed bucket is". That reason stopped being
        # true when the bucket started raising, and the sentence survived the
        # change it contradicted — which is the whole hazard of writing an
        # invariant down next to the code instead of into a test.
        if assignees.nil? || (assignees.is_a?(Array) && assignees.empty?)
          summary.delete("assignees")
        else
          summary["assignees"] = read_assignees(assignees)
        end
        summary
      end

      # The assignees, which the reference decodes as a slice of people.
      #
      # The members are checked for being objects and are then passed through
      # whole. Their FIELDS are not checked, and that is the deliberate edge of
      # this port's rule: it refuses what it would otherwise read wrongly, and
      # it does not attempt the reference's whole-body decode. A person id is
      # the one place the two genuinely differ — see {Basecamp::Ids.from_wire}.
      def read_assignees(assignees)
        unless assignees.is_a?(Array)
          raise malformed_response("the recording's \"assignees\" is #{MergeSafe.describe(assignees)}, not an array")
        end

        assignees = assignees.map do |assignee|
          # A null member is the ZERO PERSON there, and the reference emits it
          # as an object — so passing nil through handed a consumer something
          # that crashes on `assignee["id"]` where the contract gives 0. An
          # empty hash is the nearest thing this tier has to a zero Person: the
          # member is still present and still indexable. The residual difference
          # is that its id reads as nil rather than 0, which is the same
          # nested-object limit stated on #project.
          next {} if assignee.nil?
          next assignee if assignee.is_a?(Hash)

          raise malformed_response("an assignee is #{MergeSafe.describe(assignee)}, not an object")
        end

        # Returned explicitly rather than leaning on #each handing back its
        # receiver. The projection assigns from this call, so a later edit that
        # ends the method on anything else — an each_with_index, a guard clause,
        # one more line — would put nil into the summary silently.
        assignees
      end

      # The first of these that is a non-empty string.
      #
      # Each candidate is type-checked rather than coerced, for the reason
      # read_text gives: these feed the title and the content, and a to_s here
      # would have hidden exactly what that check exists to catch.
      # The recording's own id.
      #
      # The reference types this as a plain 64-bit integer, so a string, a
      # float, a boolean, an array or an object is a decode failure there — ten
      # shapes that reached the projection verbatim while the boundary comment
      # claimed this field was where reproducing the decoder would begin. It is
      # not: the check is the same one already applied to four sibling id fields
      # in this file. Absent is 0, as its zero value is.
      def read_id(value)
        id = Ids.from_wire(value)
        return id unless id.nil?

        raise malformed_response("the recording's id is #{MergeSafe.describe(value)}, not an integer")
      end

      # Whether a nested member belongs in the projection.
      #
      # The reference builds a bucket, parent or creator only when it has an id
      # or a name, so an EMPTY object leaves the key out of its summary where
      # this port emitted `{}`.
      #
      # A member that is not an object at all is KEPT on purpose, so that the
      # reader which refuses it still sees it. Dropping it here instead removed
      # a malformed bucket before the cross-bucket check could object — which
      # is the check this composite exists to protect, and which an existing
      # test caught within a minute of the rule being written the other way.
      #
      # TWO NAMED FIELDS, not "any value present", and the label differs by
      # member: the reference tests <tt>Id != 0 || Name != ""</tt> for a bucket
      # and a creator, and <tt>Id != 0 || Title != ""</tt> for a parent —
      # uniformly, across all 17 conversions. A predicate over every value
      # instead kept <tt>{"type" => "Project"}</tt> and <tt>{"url" => "u"}</tt>,
      # which the reference drops. That is the per-site rule lesson again: the
      # shared part here is the shape of the test, not the field it reads.
      def keep_member?(member, label)
        return false if member.nil?
        return true unless member.is_a?(Hash)

        id = Ids.from_wire(member["id"])
        # A malformed ID keeps the member too, for the same reason a non-object
        # member is kept: the reader that refuses it has to still see it. This
        # is the SECOND time this predicate has swallowed a malformed bucket
        # before the cross-bucket check could object — first for a bucket that
        # was not an object, now for one whose id is not an integer. Anything
        # that decides whether a key SURVIVES has to leave the malformed cases
        # for the code that reports them.
        return true if id.nil?

        !id.zero? || !member[label].to_s.empty?
      end

      def first_non_empty(*values)
        values.each do |value|
          text = read_text(value, "title or content")
          return text unless text.empty?
        end
        ""
      end

      # A text member of a read, or "" when it carries none.
      #
      # Absent or null is genuinely empty. A String passes through. Anything
      # else is a decode failure in the reference, which holds a plain string
      # for every one of these.
      def read_text(value, name)
        return "" if value.nil?
        return value if value.is_a?(String)

        raise malformed_response("the recording's \"#{name}\" is #{MergeSafe.describe(value)}, not a string")
      end

      # Resolves the line, then projects it with the Campfire it was found under.
      def summarize_chat_line(bucket_id:, line_id:)
        line, campfire_id = resolve_chat_line(bucket_id: bucket_id, line_id: line_id)
        summary = project(line)
        # Read off the PROJECTION, not off the original. project normalizes a
        # null body to a zero-valued summary — the reference decodes `null` as
        # the zero value with no error — and this line went on indexing the raw
        # response, so a chat line that came back null raised NoMethodError out
        # of a public method while every other summary route handled it. The
        # null rule was applied where it was found and not at the one site that
        # reads around it.
        unless RICH_TEXT_CHAT_LINE_TYPES.include?(summary["type"])
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
        # What a spent budget does here has three cases, and they are three
        # different answers rather than one:
        #
        # 1. A source ALREADY CONSULTED is not re-read. It could hand this call
        #    no candidate it may try, so the request cannot help — and a failure
        #    on it would replace a settled verdict with a transient error a
        #    consumer retries forever.
        # 2. A source NEVER CONSULTED is incomplete, and says which one. There
        #    may be candidates there, unsearched, and nothing unsearched is ever
        #    reported absent.
        # 3. A spent budget with BOTH sources consulted is unresolved, not
        #    incomplete. Everything was searched; "look again" would be wrong.
        refreshed = false

        if search.budget_left? && dock.cached?
          again = index.dock_campfires(account_id: account_id, bucket_id: bucket_id, refresh: true) do
            dock_campfire_ids(bucket_id)
          end
          refreshed = true if again.fetched > dock.fetched || !again.cached?
          dock = again
          found = search.try(dock.ids)
          return found if found
        end

        if !search.budget_left?
          unless list_cached
            raise CampfireDiscoveryIncompleteError.new(
              bucket_id: bucket_id, recording_id: line_id,
              reason: "the candidate budget of #{MAX_CAMPFIRE_CANDIDATES} was spent before " \
                      "the account listing was consulted"
            )
          end
        else
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
        end

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

        project = read_record(project)
        dock = project["dock"]
        # An absent dock is genuinely none — a project need not have one, and the
        # reference reads its zero value. Anything else that is not an array is a
        # decode failure there, and reading it as "no Campfires" would report a
        # line absent from a project whose dock was never legible.
        return [] if dock.nil?

        unless dock.is_a?(Array)
          raise malformed_response("the project's \"dock\" is #{MergeSafe.describe(dock)}, not an array",
            hint: DISCOVERY_HINT)
        end

        dock.filter_map do |item|
          # A null element decodes to a ZERO item there, not to an error, and a
          # zero item's name is "" — so it is skipped for the same reason any
          # non-chat item is, rather than failing the read.
          next if item.nil?

          unless item.is_a?(Hash)
            raise malformed_response("a dock item is #{MergeSafe.describe(item)}, not an object",
              hint: DISCOVERY_HINT)
          end
          # Read BEFORE the name filter, because the reference's dock item holds
          # a plain int64 id and a plain string name for EVERY item, whatever it
          # docks — so a malformed id on the schedule fails the read there while
          # a check placed after the filter would never see it. A rule that only
          # runs on the entries that survive an earlier filter is a rule about
          # this port's control flow rather than about the response.
          name = item["name"]
          unless name.nil? || name.is_a?(String)
            raise malformed_response("a dock item's name is #{MergeSafe.describe(name)}, not a string",
              hint: DISCOVERY_HINT)
          end

          # Any NON-ZERO id, as the reference keeps, rather than any positive
          # one — a negative id is a candidate there and dropping it here would
          # search one Campfire fewer. An id of the wrong type is a decode
          # failure there, so it fails this read rather than skipping an entry.
          id = Ids.from_wire(item["id"])
          if id.nil?
            raise malformed_response("a dock item's id is #{MergeSafe.describe(item["id"])}, not an integer",
              hint: DISCOVERY_HINT)
          end

          next unless name == "chat"

          id.zero? ? nil : id
        end
      end

      # The account-wide Campfire listing, grouped by bucket. BC3 has no
      # per-bucket listing, so the whole account is read and filtered. A listing
      # that overflows the cap is not cached and is reported as incomplete.
      def listed_campfire_ids_by_bucket
        campfires = @client.campfires.list(max_items: CampfireIndex::MAX_LISTING)
        listed = campfires.to_a
        if campfires.meta.truncated?
          # The reason states what was OBSERVED and not why. meta.truncated? is
          # set both by the max_items cap this call passes and by the client's
          # max_pages limit leaving a next page unfetched, and the reference
          # conflates the two in exactly the same way (client.go sets hasMore
          # for the page cap). Naming one cause reported a small two-page
          # listing under max_pages: 1 as "exceeds 1000".
          #
          # The VERDICT is the same either way, which is why this stays one
          # error: candidates were left unsearched, so the call is incomplete
          # rather than absent, and re-requesting repairs neither bound.
          raise CampfireIndex::ListingOverflow,
            "the account campfire listing was truncated before it was complete — either past the " \
            "#{CampfireIndex::MAX_LISTING}-item cap or past the client's max_pages limit"
        end

        listed.each_with_object({}) do |campfire, by_bucket|
          campfire = read_record(campfire)

          # The id is read BEFORE the bucket filter, for the reason the dock's is
          # read before its name filter: the reference decodes every listed
          # Campfire, so an id it cannot decode fails the listing whether or not
          # this call would have gone on to want that entry.
          #
          # No id FILTER at all, which is what the reference applies here — its
          # only guard on a listed Campfire is the BUCKET id below, so an id of
          # zero is a candidate too.
          campfire_id = Ids.from_wire(campfire["id"])
          if campfire_id.nil?
            raise malformed_response("a campfire's id is #{MergeSafe.describe(campfire["id"])}, not an integer",
              hint: DISCOVERY_HINT)
          end

          bucket = campfire["bucket"]
          # An absent bucket is genuinely none and cannot be grouped; anything
          # else that is not an object is a decode failure there, and skipping it
          # would drop a candidate rather than report the body.
          next if bucket.nil?

          unless bucket.is_a?(Hash)
            raise malformed_response("a campfire's \"bucket\" is #{MergeSafe.describe(bucket)}, not an object",
              hint: DISCOVERY_HINT)
          end

          bucket_id = Ids.from_wire(bucket["id"])
          if bucket_id.nil?
            raise malformed_response("a campfire's bucket id is #{MergeSafe.describe(bucket["id"])}, not an integer",
              hint: DISCOVERY_HINT)
          end
          next if bucket_id.zero?

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

        # @return [Boolean] whether this call may still try another candidate
        def budget_left?
          @budget.positive?
        end
      end
    end
  end
end
