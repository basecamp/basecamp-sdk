# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Checkins operations
    #
    # @generated from OpenAPI spec
    class CheckinsService < BaseService

      # Get pending check-in reminders for the current user
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def reminders(page: nil, max_items: nil)
        wrap_paginated(service: "checkins", operation: "reminders", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/my/question_reminders.json", params: params, operation: "GetQuestionReminders", max_items: max_items)
        end
      end

      # Get a single answer by id
      # @param answer_id [Integer] answer id ID
      # @return [Hash] response data
      def get_answer(answer_id:)
        with_operation(service: "checkins", operation: "get_answer", is_mutation: false, resource_id: answer_id) do
          http_get("/question_answers/#{answer_id}", operation: "GetAnswer").json
        end
      end

      # Update an existing answer
      # @param answer_id [Integer] answer id ID
      # @param content [String] content
      # @param group_on [String, nil] group on (YYYY-MM-DD)
      # @return [void]
      def update_answer(answer_id:, content:, group_on: nil)
        with_operation(service: "checkins", operation: "update_answer", is_mutation: true, resource_id: answer_id) do
          http_put("/question_answers/#{answer_id}", body: compact_params(content: content, group_on: group_on))
          nil
        end
      end

      # Get a questionnaire (automatic check-ins container) by id
      # @param questionnaire_id [Integer] questionnaire id ID
      # @return [Hash] response data
      def get_questionnaire(questionnaire_id:)
        with_operation(service: "checkins", operation: "get_questionnaire", is_mutation: false, resource_id: questionnaire_id) do
          http_get("/questionnaires/#{questionnaire_id}", operation: "GetQuestionnaire").json
        end
      end

      # List all questions in a questionnaire
      # @param questionnaire_id [Integer] questionnaire id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_questions(questionnaire_id:, page: nil, max_items: nil)
        wrap_paginated(service: "checkins", operation: "list_questions", is_mutation: false, resource_id: questionnaire_id) do
          params = compact_query_params(page: page)
          paginate("/questionnaires/#{questionnaire_id}/questions.json", params: params, operation: "ListQuestions", max_items: max_items)
        end
      end

      # Create a new question in a questionnaire
      # @param questionnaire_id [Integer] questionnaire id ID
      # @param title [String] title
      # @param schedule [Hash] schedule
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create_question(questionnaire_id:, title:, schedule:, visible_to_clients: nil)
        with_operation(service: "checkins", operation: "create_question", is_mutation: true, resource_id: questionnaire_id) do
          http_post("/questionnaires/#{questionnaire_id}/questions.json", body: compact_params(title: title, schedule: schedule, visible_to_clients: visible_to_clients)).json
        end
      end

      # Get a single question by id
      # @param question_id [Integer] question id ID
      # @return [Hash] response data
      def get_question(question_id:)
        with_operation(service: "checkins", operation: "get_question", is_mutation: false, resource_id: question_id) do
          http_get("/questions/#{question_id}", operation: "GetQuestion").json
        end
      end

      # Update an existing question
      # @param question_id [Integer] question id ID
      # @param title [String, nil] title
      # @param schedule [Hash, nil] schedule
      # @param paused [Boolean, nil] paused
      # @return [Hash] response data
      def update_question(question_id:, title: nil, schedule: nil, paused: nil)
        with_operation(service: "checkins", operation: "update_question", is_mutation: true, resource_id: question_id) do
          http_put("/questions/#{question_id}", body: compact_params(title: title, schedule: schedule, paused: paused)).json
        end
      end

      # List all answers for a question
      # @param question_id [Integer] question id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_answers(question_id:, page: nil, max_items: nil)
        wrap_paginated(service: "checkins", operation: "list_answers", is_mutation: false, resource_id: question_id) do
          params = compact_query_params(page: page)
          paginate("/questions/#{question_id}/answers.json", params: params, operation: "ListAnswers", max_items: max_items)
        end
      end

      # Create a new answer for a question
      # @param question_id [Integer] question id ID
      # @param content [String] content
      # @param group_on [String, nil] group on (YYYY-MM-DD)
      # @return [Hash] response data
      def create_answer(question_id:, content:, group_on: nil)
        with_operation(service: "checkins", operation: "create_answer", is_mutation: true, resource_id: question_id) do
          http_post("/questions/#{question_id}/answers.json", body: compact_params(content: content, group_on: group_on)).json
        end
      end

      # List all people who have answered a question (answerers)
      # @param question_id [Integer] question id ID
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def answerers(question_id:, max_items: nil)
        wrap_paginated(service: "checkins", operation: "answerers", is_mutation: false, resource_id: question_id) do
          paginate("/questions/#{question_id}/answers/by.json", operation: "ListQuestionAnswerers", max_items: max_items)
        end
      end

      # Get all answers from a specific person for a question
      # @param question_id [Integer] question id ID
      # @param person_id [Integer] person id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def by_person(question_id:, person_id:, page: nil, max_items: nil)
        wrap_paginated(service: "checkins", operation: "by_person", is_mutation: false, resource_id: person_id) do
          params = compact_query_params(page: page)
          paginate("/questions/#{question_id}/answers/by/#{person_id}", params: params, operation: "GetAnswersByPerson", max_items: max_items)
        end
      end

      # Update notification settings for a check-in question
      # @param question_id [Integer] question id ID
      # @param notify_on_answer [Boolean, nil] Notify when someone answers
      # @param digest_include_unanswered [Boolean, nil] Include unanswered in digest
      # @return [Hash] response data
      def update_notification_settings(question_id:, notify_on_answer: nil, digest_include_unanswered: nil)
        with_operation(service: "checkins", operation: "update_notification_settings", is_mutation: true, resource_id: question_id) do
          http_put("/questions/#{question_id}/notification_settings.json", body: compact_params(notify_on_answer: notify_on_answer, digest_include_unanswered: digest_include_unanswered)).json
        end
      end

      # Pause a check-in question (stops sending reminders)
      # @param question_id [Integer] question id ID
      # @return [Hash] response data
      def pause(question_id:)
        with_operation(service: "checkins", operation: "pause", is_mutation: true, resource_id: question_id) do
          http_post("/questions/#{question_id}/pause.json").json
        end
      end

      # Resume a paused check-in question (resumes sending reminders)
      # @param question_id [Integer] question id ID
      # @return [Hash] response data
      def resume(question_id:)
        with_operation(service: "checkins", operation: "resume", is_mutation: true, resource_id: question_id) do
          http_delete("/questions/#{question_id}/pause.json").json
        end
      end
    end
  end
end
