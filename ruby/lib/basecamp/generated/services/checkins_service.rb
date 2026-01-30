# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Checkins operations
    #
    # @generated from OpenAPI spec
    class CheckinsService < BaseService

      # Get a single answer by id
      # @param project_id [Integer] project id ID
      # @param answer_id [Integer] answer id ID
      # @return [Hash] response data
      def get_answer(project_id:, answer_id:)
        http_get(bucket_path(project_id, "/question_answers/#{answer_id}")).json
      end

      # Update an existing answer
      # @param project_id [Integer] project id ID
      # @param answer_id [Integer] answer id ID
      # @param content [String] content
      # @return [Hash] response data
      def update_answer(project_id:, answer_id:, content:)
        http_put(bucket_path(project_id, "/question_answers/#{answer_id}"), body: compact_params(content: content)).json
      end

      # Get a questionnaire (automatic check-ins container) by id
      # @param project_id [Integer] project id ID
      # @param questionnaire_id [Integer] questionnaire id ID
      # @return [Hash] response data
      def get_questionnaire(project_id:, questionnaire_id:)
        http_get(bucket_path(project_id, "/questionnaires/#{questionnaire_id}")).json
      end

      # List all questions in a questionnaire
      # @param project_id [Integer] project id ID
      # @param questionnaire_id [Integer] questionnaire id ID
      # @return [Enumerator<Hash>] paginated results
      def list_questions(project_id:, questionnaire_id:)
        paginate(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"))
      end

      # Create a new question in a questionnaire
      # @param project_id [Integer] project id ID
      # @param questionnaire_id [Integer] questionnaire id ID
      # @param title [String] title
      # @param schedule [String] schedule
      # @return [Hash] response data
      def create_question(project_id:, questionnaire_id:, title:, schedule:)
        http_post(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"), body: compact_params(title: title, schedule: schedule)).json
      end

      # Get a single question by id
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @return [Hash] response data
      def get_question(project_id:, question_id:)
        http_get(bucket_path(project_id, "/questions/#{question_id}")).json
      end

      # Update an existing question
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @param title [String, nil] title
      # @param schedule [String, nil] schedule
      # @param paused [Boolean, nil] paused
      # @return [Hash] response data
      def update_question(project_id:, question_id:, title: nil, schedule: nil, paused: nil)
        http_put(bucket_path(project_id, "/questions/#{question_id}"), body: compact_params(title: title, schedule: schedule, paused: paused)).json
      end

      # List all answers for a question
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @return [Enumerator<Hash>] paginated results
      def list_answers(project_id:, question_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers.json"))
      end

      # Create a new answer for a question
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @param content [String] content
      # @param group_on [String, nil] group on (YYYY-MM-DD)
      # @return [Hash] response data
      def create_answer(project_id:, question_id:, content:, group_on: nil)
        http_post(bucket_path(project_id, "/questions/#{question_id}/answers.json"), body: compact_params(content: content, group_on: group_on)).json
      end

      # List all people who have answered a question (answerers)
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @return [Enumerator<Hash>] paginated results
      def answerers(project_id:, question_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers/by.json"))
      end

      # Get all answers from a specific person for a question
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @param person_id [Integer] person id ID
      # @return [Enumerator<Hash>] paginated results
      def by_person(project_id:, question_id:, person_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers/by/#{person_id}"))
      end

      # Update notification settings for a check-in question
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @param notify_on_answer [Boolean, nil] Notify when someone answers
      # @param digest_include_unanswered [Boolean, nil] Include unanswered in digest
      # @return [void]
      def update_notification_settings(project_id:, question_id:, notify_on_answer: nil, digest_include_unanswered: nil)
        http_put(bucket_path(project_id, "/questions/#{question_id}/notification_settings.json"), body: compact_params(notify_on_answer: notify_on_answer, digest_include_unanswered: digest_include_unanswered))
        nil
      end

      # Pause a check-in question (stops sending reminders)
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @return [void]
      def pause(project_id:, question_id:)
        http_post(bucket_path(project_id, "/questions/#{question_id}/pause.json"))
        nil
      end

      # Resume a paused check-in question (resumes sending reminders)
      # @param project_id [Integer] project id ID
      # @param question_id [Integer] question id ID
      # @return [void]
      def resume(project_id:, question_id:)
        http_delete(bucket_path(project_id, "/questions/#{question_id}/pause.json"))
        nil
      end

      # Get pending check-in reminders for the current user
      # @return [Enumerator<Hash>] paginated results
      def reminders()
        paginate("/my/question_reminders.json")
      end
    end
  end
end
