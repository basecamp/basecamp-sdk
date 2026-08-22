# frozen_string_literal: true

module Basecamp
  module Services
    # Merge-safe +update+ and read-modify-write +edit+ for todolists (and
    # todolist groups), prepended onto the generated {TodolistsService} (see
    # the +on_load+ hook in +basecamp.rb+).
    #
    # BC3's +TodolistsController#update+ rebuilds the recordable from only the
    # permitted params, so <tt>PUT /todolists/{id}</tt> is a full replace:
    # a body that omits +description+ ERASES it. The sparse PUT — the natural
    # thing to write — is therefore destructive on the raw endpoint, which
    # stays available as {#replace}.
    #
    # Both compose the public +get+ and +replace+ methods, so hooks observe
    # the two wire operations (+get+ then +replace+), not a synthetic
    # composite.
    #
    # Neither is atomic: there is no conditional-update signal on this
    # endpoint, so a concurrent write between the GET and PUT is
    # overwritten — last write wins for the whole representation. The
    # window is one round-trip. Use +replace+ to overwrite deliberately.
    module TodolistsExtensions
      # A todolist's full writable state, yielded to the +edit+ block. The
      # whole struct is PUT back to the server, so clearing a field means
      # setting it empty (<tt>""</tt>) — there is no third state. The writable
      # set is exactly what BC3 permits: +name+ and +description+.
      TodolistFields = Struct.new(:name, :description, keyword_init: true)

      # Sets the given fields on a todolist and preserves everything else:
      # GETs the current todolist, overlays the explicitly-passed keyword
      # arguments, and PUTs the full representation back. An omitted (+nil+)
      # field is untouched, guaranteed; an explicitly-passed <tt>""</tt>
      # clears.
      #
      # Not atomic — see the module docs for the GET→PUT race. Use {#replace}
      # to overwrite deliberately, or {#edit} to clear fields.
      #
      # @param id [Integer] todolist id
      # @param name [String, nil] new name (nil = keep current)
      # @param description [String, nil] new description (nil = keep current, "" clears)
      # @return [Hash] the updated todolist
      # @raise [Basecamp::UsageError] if the resulting name would be empty
      def update(id:, name: nil, description: nil)
        fields = fields_from_todolist(get(id: id))
        fields.name = name unless name.nil?
        fields.description = description unless description.nil?
        put_fields(id, fields)
      end

      # Applies a read-modify-write block to a todolist: GETs the current
      # todolist, yields its full writable state ({TodolistFields}), and PUTs
      # the whole thing back. Clearing a field means setting it empty
      # (<tt>""</tt>) — an untouched field keeps its current value. If the
      # block raises, the edit aborts and nothing is written.
      #
      # Not atomic — see the module docs for the GET→PUT race.
      #
      # @example
      #   account.todolists.edit(id: 123) do |list|
      #     list.name = "🚨 #{list.name}"
      #     list.description = "" # clearing = setting empty on a full object
      #   end
      #
      # @param id [Integer] todolist id
      # @yieldparam fields [TodolistFields] the todolist's writable state, to mutate in place
      # @return [Hash] the updated todolist
      # @raise [ArgumentError] if no block is given
      # @raise [Basecamp::UsageError] if the block leaves the name empty
      def edit(id:)
        raise ArgumentError, "edit requires a block" unless block_given?

        fields = fields_from_todolist(get(id: id))
        yield fields
        put_fields(id, fields)
      end

      private

      # Derives the full writable state from a GET response.
      #
      # BC3 answers this route with the recordable's flat JSON, and since #544
      # the Smithy model says the same: one flat +Todolist+ structure, no
      # +todolist+/+group+ envelope and no union. A group is a Todolist —
      # +todolists/groups/{index,show}.json.jbuilder+ render
      # +todolists/_todolist.json.jbuilder+ — so both projections arrive here
      # with +name+ and +description+ at the top level and are read the same
      # way. Nothing branches on the +type+ string; the structural
      # discriminator (+groups_url+ for a list, +group_position_url+ for a
      # group) is not writable state and is none of this method's business.
      #
      # The former arm lookup is gone with the union that motivated it: an
      # unmodelled +todolist+/+group+ wrapper is now a malformed response, and
      # unwrapping one would write the wrapper's contents over the record.
      def fields_from_todolist(todolist)
        body = require_hash(todolist)
        TodolistFields.new(
          name: writable_string(body, "name", non_empty: true),
          description: writable_string(body, "description")
        )
      end

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

      # The response must be a Hash before any field is read.
      #
      # Level 1 of the wire-to-written-value path, one level up from the
      # malformed-field guards. Since #544 flattened the shape the path is
      # object -> scalar and has exactly two levels — the body and each writable
      # field — where it used to have three. Two, not none: a flat wire shape
      # says what the API returns, not that anything validates it. The generated
      # +get+ returns <tt>http_get(...).json</tt>, a raw Hash with no decoder
      # behind it, so a successful GET can still hand this method a scalar, an
      # Array or nil.
      #
      # +body["name"]+ raises TypeError on an Integer or Array, and on a String
      # it does not raise at all: it is a substring search. A body of
      # <tt>"no name here"</tt> answers <tt>"name"</tt> for +name+ and +nil+ for
      # +description+, so without this guard the composite would PUT the literal
      # string "name" over the record's real name and clear its description —
      # failing silently, which is why that defect outlived eight review passes
      # on #574. A String has no interior, so there is no third level.
      def require_hash(body)
        unless body.is_a?(Hash)
          raise ApiError.new(
            Security.truncate("GetTodolistOrGroup returned #{describe(body)} where a todolist object was expected"),
            hint: "The merge-safe update/edit read this record's fields before rewriting them, " \
              "so a non-object body cannot be used. Use replace to write the record deliberately."
          )
        end

        body
      end

      # Reads a writable string field, refusing to coerce a malformed one.
      #
      # *Classification is by origin, not by value.* The same empty string is a
      # caller error when the caller passed it and malformed response data when
      # it came off the wire, so each provenance is checked where it is
      # unambiguous: this read step owns the response, and +put_fields+ owns the
      # caller. That is why an empty +name+ here raises ApiError while an empty
      # +name+ the caller supplied raises UsageError — same value, different
      # origin, different fault.
      #
      # *Presence and non-emptiness are two different claims, and only one of
      # them is per-field.* Since #544 +name+ and +description+ are both
      # +@required+ and never null on this shape — +format_api_content+ funnels
      # a blank rich text through +call_pipeline+, which returns <tt>""</tt>
      # rather than nil — so for BOTH a missing key and an explicit +nil+ are
      # malformed and are refused here, before any PUT. Reading either as
      # <tt>""</tt> would put that <tt>""</tt> in the full-replace body and
      # erase the record's real value on a call that never mentioned the field.
      #
      # +non_empty+ is the OTHER claim and holds for +name+ alone: BC3
      # presence-validates the attribute, so no real todolist carries an empty
      # one and <tt>""</tt> off the wire is malformed too. +description+ has no
      # such validation — a description-less list carries <tt>""</tt>, which is
      # the ordinary case, and the canonical group fixture ships one — so an
      # empty description is a real value, preserved and resent verbatim.
      # Conflating the two flags would refuse every description-less record.
      #
      # A wrong type is malformed either way and must NOT be coerced: a plain
      # <tt>|| ""</tt> turns +false+ into <tt>""</tt> and passes arrays, hashes
      # and numbers straight through. This endpoint is full-replace, so either
      # outcome is written back over the real value.
      #
      # Ruby has no typed decoder between the GET and this read, unlike the Go,
      # Swift and Kotlin composites where a wrong-typed field fails at decode,
      # and flattening the shape did not add one: the generated method still
      # returns <tt>http_get(...).json</tt> verbatim. The same shape in the
      # shipped Todos composite is guarded by the MergeSafe checks #576 closed
      # with; the generated validating layer that would retire this guard is
      # tracked in #578.
      def writable_string(body, key, non_empty: false)
        raise_missing_field(key) unless body.key?(key)

        value = body[key]

        if value.nil?
          raise_null_field(key)
        elsif !value.is_a?(String)
          raise ApiError.new(
            Security.truncate("Todolist field #{key.inspect} is not a string: #{describe(value)}"),
            hint: "The merge-safe update/edit resend this field verbatim, so a coerced or " \
              "empty value would overwrite the current one. Use replace to write the record " \
              "deliberately."
          )
        elsif non_empty && value.empty?
          raise ApiError.new(
            "Todolist field #{key.inspect} is empty in the response",
            hint: "#{key} is presence-validated server-side, so an empty one is a malformed " \
              "response. The caller did not ask to clear it."
          )
        else
          value
        end
      end

      def raise_missing_field(key)
        raise ApiError.new(
          "Todolist field #{key.inspect} is missing from the response",
          hint: "#{key} is required on every todolist, so a body without one is a malformed " \
            "response, not an empty value to preserve. The merge-safe update/edit PUT the full " \
            "writable state back, so reading it as empty would erase the real value."
        )
      end

      def raise_null_field(key)
        raise ApiError.new(
          "Todolist field #{key.inspect} is null in the response",
          hint: "#{key} is required and never null, so a null one is a malformed response, not " \
            "an empty value to preserve. The merge-safe update/edit PUT the full writable state " \
            "back, so reading it as empty would erase the real value."
        )
      end

      # PUTs the full writable state via +replace+. Both fields are always
      # sent, description included when empty: the generated layer's
      # +compact_params+ strips nils, so a cleared description travels as
      # <tt>""</tt> rather than JSON null (SPEC section 18 body compaction) —
      # and omitting it would hand the clear back to the server's rebuild
      # instead of stating it.
      #
      # An empty name is refused rather than sent: BC3 presence-validates it,
      # so a blank name is a 422 and never a preserve.
      def put_fields(id, fields)
        name = caller_string(fields.name, "name")
        description = caller_string(fields.description, "description")

        if name.empty?
          raise UsageError, "name must be present; a full write has no nil state and BC3 rejects a blank name with 422"
        end

        replace(id: id, name: name, description: description)
      end

      # Validates a caller-supplied writable value, the mirror of the read step.
      #
      # +writable_string+ owns *response* provenance; this owns *caller*
      # provenance, and the two are one rule seen from opposite ends. +edit+
      # yields a mutable view of the full writable state and Ruby enforces
      # nothing about what comes back — a block assigning +42+ or +[]+ would
      # otherwise walk straight into the full-replace PUT and write it. That is
      # caller misuse, hence UsageError, where the same wrong type arriving from
      # the server is an ApiError. +nil+ is accepted as the empty string: the
      # struct starts nil-valued and clearing by assigning nil is idiomatic.
      def caller_string(value, key)
        if value.nil?
          ""
        elsif value.is_a?(String)
          value
        else
          raise UsageError.new(
            Security.truncate("todolist #{key} must be a String, got #{describe(value)}"),
            hint: "The full writable state is PUT back verbatim, so a non-String would be " \
              "written to the record. Assign a String; use \"\" to clear."
          )
        end
      end
    end
  end
end
