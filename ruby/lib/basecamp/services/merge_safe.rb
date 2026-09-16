# frozen_string_literal: true

module Basecamp
  module Services
    # Response guards shared by the merge-safe composites.
    #
    # A merge-safe +update+/+edit+ GETs a record, reads each writable field,
    # and PUTs the *full* representation back. The endpoint is full-replace, so
    # every value read here is written — including one the caller never
    # mentioned. If the read step coerces or forwards a malformed value instead
    # of refusing it, that value lands on the record.
    #
    # Two failure modes, the same defect wearing different clothes:
    #
    # * *erasure* — <tt>|| ""</tt> turns +false+ into <tt>""</tt>, wiping the
    #   field;
    # * *corruption* — everything else falsey-in-other-languages (+0+, +[]+,
    #   <tt>{}</tt>) and every truthy non-string (+42+, +true+,
    #   <tt>["x"]</tt>) is forwarded verbatim, writing a number, boolean, array
    #   or hash where a String belongs.
    #
    # Ruby's +||+ treats only +nil+ and +false+ as falsy, so it erases in one
    # case and corrupts in the rest. Testing only for erasure is what let this
    # class survive five review passes, so both are refused here.
    #
    # *The rule: a composite is safe exactly when a typed decoder sits between
    # the GET and the field read.* Go (+json.Unmarshal+), Swift (+Codable+) and
    # Kotlin (kotlinx.serialization) get one for free from their models. Ruby
    # does not — the generated services return a raw Hash, so nothing rejects a
    # wrong-typed field and the check has to be explicit. That is why these
    # guards exist in Ruby, Python and TypeScript and nowhere else (#576).
    #
    # Todolists carries its own copy of these guards (#574). #544 flattened the
    # shape those guards read — dropping the envelope-arm rung, not the guards —
    # but did not unify them here. A generated validating layer (#578) is the
    # intended end state for all of them.
    module MergeSafe
      RESEND_HINT = "The merge-safe update/edit resend this field verbatim, so a coerced or " \
        "empty value would overwrite the current one. Use %<escape>s to write the record " \
        "deliberately."

      module_function

      # Renders a value for an error message without ever throwing.
      #
      # The guard's own error path must not fail while explaining a failure:
      # +inspect+ is arbitrary user code and can raise. The class name is always
      # available; the rendering is a bonus, capped per SPEC section 9 and
      # dropped if it fails.
      def describe(value)
        kind = value.class.to_s
        begin
          Security.truncate("#{kind} #{value.inspect}")
        rescue StandardError
          kind
        end
      end

      # Builds the malformed-response error.
      #
      # ApiError, not UsageError: the value arrived in a successful API
      # response, so nothing the caller passed is at fault. Non-retryable,
      # because re-requesting cannot repair a malformed body.
      def malformed(message, hint)
        ApiError.new(Security.truncate(message), hint: hint, retryable: false)
      end

      # The response must be a Hash before any field is read.
      #
      # One level up from the malformed-field guards: a successful GET can
      # return a scalar, an Array, or nil. <tt>body["due_on"]</tt> raises
      # TypeError on an Integer or Array and returns a silent nil substring
      # match on a String, so a malformed envelope would surface as a native
      # TypeError instead of the documented statusless +api_error+.
      def require_hash(body, record:, operation:, escape:)
        return body if body.is_a?(Hash)

        raise malformed(
          "#{operation} returned #{describe(body)} where a #{record.downcase} object was expected",
          "The merge-safe update/edit read this record's fields before rewriting them, so a " \
            "non-object body cannot be used. Use #{escape} to write the record deliberately."
        )
      end

      # Reads a writable string field, refusing to coerce a malformed one.
      #
      # A missing key or an explicit +nil+ is genuinely empty — there is nothing
      # to preserve and <tt>""</tt> is what the server already holds. An actual
      # String passes verbatim. Anything else is a malformed response and is
      # refused *before* the PUT, naming the offending field.
      def writable_string(body, key, record:, escape:)
        value = body[key]

        if value.nil?
          ""
        elsif value.is_a?(String)
          value
        else
          raise malformed(
            "#{record} field #{key.inspect} is not a string: #{describe(value)}",
            format(RESEND_HINT, escape: escape)
          )
        end
      end

      # Reads a writable string the record is *required* to carry.
      #
      # {writable_string} treats an absent key or an explicit +nil+ as genuinely
      # empty, which is right for an optional field — <tt>""</tt> is what the
      # server already holds. It is wrong for a required one. Where the spec
      # marks a response member <tt>@required</tt> and BC3 can never render it
      # blank, an absent, nil or blank value in a 2xx body is a *malformed
      # response*, not an empty field. Coalescing it to <tt>""</tt> and sending
      # that in the full-replace PUT would blank the real value on a call that
      # never mentioned it — #576's defect exactly.
      #
      # Two records rely on this today and for the same reason: +Document#title+
      # is <tt>super.presence || "Untitled"</tt> and +Schedule::Entry#summary+ is
      # <tt>super.presence || "Untitled"</tt>, so neither can come back blank
      # from a healthy server.
      #
      # The wrong-type branch is delegated to {writable_string}, so a required
      # field and an optional one report a non-string identically.
      def required_writable_string(body, key, record:, escape:)
        value = body[key]
        if value.nil? || (value.is_a?(String) && value.strip.empty?)
          raise malformed(
            %(#{record} field "#{key}" is required but the response carried #{describe(value)}),
            "The merge-safe update/edit resend this field verbatim, so a missing or blank " \
              "value would blank the current one. Use #{escape} to write the record deliberately."
          )
        end

        writable_string(body, key, record: record, escape: escape)
      end

      # Reads a writable boolean the record is *required* to carry.
      #
      # The boolean analogue of {required_writable_string}, and it cannot be
      # expressed with a truthiness test: the value this guard most needs to
      # admit is +false+, which every <tt>||</tt> idiom would treat as missing
      # and replace with a default. +Schedule::Entry#all_day+ is NOT NULL with a
      # +false+ default in BC3 and every partial emits it, so absent or nil is a
      # malformed response — and defaulting it to +false+ would silently convert
      # an all-day event into a midnight-to-midnight timed one on a call that
      # only changed the summary.
      #
      # +0+/+1+ are refused rather than coerced, for the same reason
      # {writable_string} refuses +42+: JSON has a boolean type and the server
      # uses it.
      def required_writable_boolean(body, key, record:, escape:)
        value = body[key]

        if value.nil?
          raise malformed(
            %(#{record} field "#{key}" is required but the response carried #{describe(value)}),
            "The merge-safe update/edit resend this field verbatim, so a missing value would " \
              "replace the current one with a default. Use #{escape} to write the record deliberately."
          )
        end

        unless [ true, false ].include?(value)
          raise malformed(
            "#{record} field #{key.inspect} is not a boolean: #{describe(value)}",
            format(RESEND_HINT, escape: escape)
          )
        end

        value
      end

      # Reads an *optional* writable boolean, refusing to coerce a malformed one.
      #
      # {writable_string}'s boolean sibling, standing in the same relation to
      # {required_writable_boolean} that +writable_string+ does to
      # +required_writable_string+: a missing key or an explicit +nil+ is
      # genuinely "not set" and returns +false+, because that is what the server
      # already holds.
      #
      # +ScheduleEntry#highlighted+ is the case it exists for. The entry partial
      # emits it unconditionally, but the reduced calendar partial behind
      # GetUpcomingSchedule does not, and both render through the same schema —
      # so the member is optional and absence is legitimate rather than
      # malformed.
      #
      # What still cannot be tolerated is the *wrong type*: a <tt>"yes"</tt> or a
      # +1+ must be refused, not coerced, because a caller who assigns the seeded
      # value straight back sends whatever it was seeded with. That branch is
      # delegated to {required_writable_boolean}, so an optional boolean and a
      # required one report a non-boolean identically.
      def writable_boolean(body, key, record:, escape:)
        if body[key].nil?
          false
        else
          required_writable_boolean(body, key, record: record, escape: escape)
        end
      end

      # Reads a list of person records and projects it to their Integer ids.
      #
      # The analogue of {writable_string} for the id-list fields. The +map+ it
      # replaces (<tt>(body[key] || []).map { |p| p["id"] }</tt>) has three ways
      # to go wrong on malformed data: a non-Array has no +map+ (or, for a Hash,
      # maps over its pairs), a non-Hash element raises TypeError on +[]+, and a
      # wrong-typed +id+ rides through verbatim into the full-replace PUT — the
      # same corruption as a wrong-typed string, one level down.
      #
      # What it must NOT do is refuse an id the reference ACCEPTS, which is the
      # other half of the same defect and the one this guard had (#912). A
      # person's id is the single field in the generated model the reference
      # decodes flexibly, and +fieldsFromTodo+ appends what that produced with
      # no filter of any kind — 0 included. The shapes are live rather than
      # theoretical: across +spec/fixtures+, 3 of 7 +assignees+ people carry no
      # +personable_type+, which is exactly what keeps {Basecamp::Http}'s
      # pre-decode normalizer from having already turned their string ids into
      # Integers. The rule itself lives in {Basecamp::Ids.person_from_wire},
      # where the read path already reads one, so the two cannot drift.
      def writable_id_list(body, key, record:, escape:)
        value = body[key]
        return [] if value.nil?

        unless value.is_a?(Array)
          raise malformed(
            "#{record} field #{key.inspect} is not an array: #{describe(value)}",
            format(RESEND_HINT, escape: escape)
          )
        end

        value.each_with_index.map do |element, index|
          person_id(element, index, key, record: record, escape: escape)
        end
      end

      # Validates one element of an id-list field and returns its id.
      #
      # Three element shapes are accepted that a strict reading would refuse,
      # each because the reference accepts it:
      #
      # * a +nil+ element is 0 — a JSON null in the array decodes to the
      #   reference's zero Person, whose id is 0 and is appended like any other;
      # * an element with NO "id" is 0, the zero value, with no error;
      # * an element whose "id" is a String is whatever ParseInt makes of it,
      #   and a String that is not a number at all is the system actor 0.
      #
      # An explicit <tt>"id" => nil</tt> is NOT the same as an absent one and
      # still fails the read, which is why the absent case is tested with
      # +key?+ rather than by asking for the value: see
      # {Basecamp::Ids.person_from_wire} for the decoder line that splits them.
      def person_id(element, index, key, record:, escape:)
        return 0 if element.nil?

        unless element.is_a?(Hash)
          raise malformed(
            "#{record} field #{key.inspect}[#{index}] is not an object: #{describe(element)}",
            format(RESEND_HINT, escape: escape)
          )
        end

        return 0 unless element.key?("id")

        id = Ids.person_from_wire(element["id"])
        if id.nil?
          raise malformed(
            "#{record} field #{key.inspect}[#{index}].id is not a person id: #{describe(element["id"])}",
            format(RESEND_HINT, escape: escape)
          )
        end

        id
      end
    end
  end
end
