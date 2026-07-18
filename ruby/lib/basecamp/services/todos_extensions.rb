# frozen_string_literal: true

module Basecamp
  module Services
    # Merge-safe +update+ and read-modify-write +edit+ for todos, prepended
    # onto the generated {TodosService} (see the +on_load+ hook in
    # +basecamp.rb+).
    #
    # Both compose the public +get+ and +replace+ methods, so hooks observe
    # the two wire operations (+get+ then +replace+), not a synthetic
    # composite.
    #
    # Neither is atomic: there is no conditional-update signal on this
    # endpoint, so a concurrent write between the GET and PUT is
    # overwritten — last write wins for the whole representation. The
    # window is one round-trip. Use +replace+ to overwrite deliberately.
    module TodosExtensions
      # A todo's full writable state, yielded to the +edit+ block. The
      # whole struct is PUT back to the server, so clearing a field means
      # setting it empty (+""+ for strings and dates, +[]+ for ID lists) —
      # there is no third state. +notify+ is a send directive, not todo
      # state: never populated from the current todo, sent only when true.
      TodoFields = Struct.new(
        :content, :description, :assignee_ids, :completion_subscriber_ids,
        :due_on, :starts_on, :notify,
        keyword_init: true
      )

      # Sets the given fields on a todo and preserves everything else:
      # GETs the current todo, overlays the explicitly-passed keyword
      # arguments, and PUTs the full representation back. An omitted
      # (+nil+) field is untouched, guaranteed; an explicitly-passed empty
      # array clears.
      #
      # Not atomic — see the module docs for the GET→PUT race. Use
      # {#replace} to overwrite deliberately, or {#edit} to clear fields.
      #
      # @param todo_id [Integer] todo id
      # @param content [String, nil] new content (nil = keep current)
      # @param description [String, nil] new description (nil = keep current)
      # @param assignee_ids [Array, nil] complete assignee list ([] clears)
      # @param completion_subscriber_ids [Array, nil] complete subscriber list ([] clears)
      # @param notify [Boolean, nil] notify assignees about this write
      # @param due_on [String, nil] due date YYYY-MM-DD (nil = keep current)
      # @param starts_on [String, nil] start date YYYY-MM-DD (nil = keep current)
      # @return [Hash] the updated todo
      def update(todo_id:, content: nil, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        fields = fields_from_todo(get(todo_id: todo_id))
        fields.content = content unless content.nil?
        fields.description = description unless description.nil?
        fields.assignee_ids = assignee_ids unless assignee_ids.nil?
        fields.completion_subscriber_ids = completion_subscriber_ids unless completion_subscriber_ids.nil?
        fields.due_on = due_on unless due_on.nil?
        fields.starts_on = starts_on unless starts_on.nil?
        fields.notify = notify unless notify.nil?
        put_fields(todo_id, fields)
      end

      # Applies a read-modify-write block to a todo: GETs the current todo,
      # yields its full writable state ({TodoFields}), and PUTs the whole
      # thing back. Clearing a field means setting it empty (+""+ / +[]+) —
      # an untouched field keeps its current value. If the block raises,
      # the edit aborts and nothing is written.
      #
      # Not atomic — see the module docs for the GET→PUT race.
      #
      # @example
      #   account.todos.edit(todo_id: 123) do |t|
      #     t.content = "🚨 #{t.content}"
      #     t.due_on = "" # clearing = setting empty on a full object
      #   end
      #
      # @param todo_id [Integer] todo id
      # @yieldparam fields [TodoFields] the todo's writable state, to mutate in place
      # @return [Hash] the updated todo
      # @raise [ArgumentError] if no block is given
      def edit(todo_id:)
        raise ArgumentError, "edit requires a block" unless block_given?

        fields = fields_from_todo(get(todo_id: todo_id))
        yield fields
        put_fields(todo_id, fields)
      end

      private

      # Derives the full writable state from a GET response.
      def fields_from_todo(todo)
        TodoFields.new(
          content: todo["content"] || "",
          description: todo["description"] || "",
          assignee_ids: (todo["assignees"] || []).map { |p| p["id"] },
          completion_subscriber_ids: (todo["completion_subscribers"] || []).map { |p| p["id"] },
          due_on: todo["due_on"] || "",
          starts_on: todo["starts_on"] || "",
          notify: false
        )
      end

      # PUTs the full writable state via +replace+: content, description,
      # and both ID lists are always sent (empties included, so clears
      # survive); dates only when non-empty (the server clears an omitted
      # date, and +""+ is a format error); notify only when true.
      def put_fields(todo_id, fields)
        %i[assignee_ids completion_subscriber_ids].each do |key|
          raise UsageError, "#{key} must be an array of person IDs; use [] to clear — a full write has no nil state" if fields[key].nil?
        end
        replace(
          todo_id: todo_id,
          content: fields.content,
          description: fields.description,
          assignee_ids: fields.assignee_ids,
          completion_subscriber_ids: fields.completion_subscriber_ids,
          due_on: fields.due_on.to_s.empty? ? nil : fields.due_on,
          starts_on: fields.starts_on.to_s.empty? ? nil : fields.starts_on,
          notify: fields.notify ? true : nil
        )
      end
    end
  end
end
