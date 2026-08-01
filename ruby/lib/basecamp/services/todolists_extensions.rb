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

      # Derives the full writable state from a GET response. BC3 answers this
      # route with the recordable's flat JSON; the +todolist+/+group+ envelope
      # in the Smithy model is a spec convention, not the wire shape (see
      # AGENTS.md, "Smithy Spec vs Actual API Responses"). Unwrapped anyway so
      # either shape reads correctly — it costs one lookup.
      def fields_from_todolist(todolist)
        body = todolist["todolist"] || todolist["group"] || todolist
        TodolistFields.new(
          name: writable_string(body, "name", required: true),
          description: writable_string(body, "description")
        )
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
      # A +required+ field (one the schema marks non-nullable, as +name+ is)
      # must arrive as a non-empty String: missing, +nil+ and <tt>""</tt> are all
      # malformed, because BC3 presence-validates +name+ so no real todolist has
      # one. An optional field (+description+) treats missing and +nil+ as
      # genuinely empty — there is nothing to preserve and <tt>""</tt> is what
      # the server already holds.
      #
      # A wrong type is malformed either way and must NOT be coerced: a plain
      # <tt>|| ""</tt> turns +false+ into <tt>""</tt> and passes arrays, hashes
      # and numbers straight through. This endpoint is full-replace, so either
      # outcome is written back over the real value.
      #
      # Ruby has no typed decoder between the GET and this read, unlike the Go,
      # Swift and Kotlin composites where a wrong-typed field fails at decode.
      # The same shape is live in the shipped Todos composite; tracked in #576.
      def writable_string(body, key, required: false)
        value = body[key]

        if value.nil?
          raise_missing_field(key) if required
          ""
        elsif !value.is_a?(String)
          raise ApiError.new(
            "Todolist field #{key.inspect} is not a string: #{value.inspect}",
            hint: "The merge-safe update/edit resend this field verbatim, so a coerced or " \
              "empty value would overwrite the current one. Use replace to write the record " \
              "deliberately."
          )
        elsif required && value.empty?
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
          hint: "#{key} is required and presence-validated server-side, so a todolist without " \
            "one is a malformed response, not an empty value to preserve."
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
            "todolist #{key} must be a String, got #{value.class}: #{value.inspect}",
            hint: "The full writable state is PUT back verbatim, so a non-String would be " \
              "written to the record. Assign a String; use \"\" to clear."
          )
        end
      end
    end
  end
end
