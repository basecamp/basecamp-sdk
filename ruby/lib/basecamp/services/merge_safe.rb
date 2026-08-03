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

      # Reads a list of person records and projects it to their Integer ids.
      #
      # The analogue of {writable_string} for the id-list fields. The +map+ it
      # replaces (<tt>(body[key] || []).map { |p| p["id"] }</tt>) has three ways
      # to go wrong on malformed data: a non-Array has no +map+ (or, for a Hash,
      # maps over its pairs), a non-Hash element raises TypeError on +[]+, and a
      # non-Integer +id+ rides through verbatim into the full-replace PUT — the
      # same corruption as a wrong-typed string, one level down.
      #
      # +true+/+false+ are refused explicitly: they are not Integers in Ruby, so
      # +is_a?(Integer)+ already rejects them, but the message names them as ids
      # rather than as an unexplained type error.
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
      def person_id(element, index, key, record:, escape:)
        unless element.is_a?(Hash)
          raise malformed(
            "#{record} field #{key.inspect}[#{index}] is not an object: #{describe(element)}",
            format(RESEND_HINT, escape: escape)
          )
        end

        id = element["id"]
        if id.nil?
          raise malformed(
            "#{record} field #{key.inspect}[#{index}] has no \"id\"",
            format(RESEND_HINT, escape: escape)
          )
        end

        unless id.is_a?(Integer)
          raise malformed(
            "#{record} field #{key.inspect}[#{index}].id is not an integer: #{describe(id)}",
            format(RESEND_HINT, escape: escape)
          )
        end

        id
      end
    end
  end
end
