# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Checkins operations
    #
    # @generated from OpenAPI spec
    class CheckinsService < BaseService

      # Get a single answer by id
      def get_answer(project_id:, answer_id:)
        http_get(bucket_path(project_id, "/question_answers/#{answer_id}")).json
      end

      # Update an existing answer
      def update_answer(project_id:, answer_id:, **body)
        http_put(bucket_path(project_id, "/question_answers/#{answer_id}"), body: body).json
      end

      # Get a questionnaire (automatic check-ins container) by id
      def get_questionnaire(project_id:, questionnaire_id:)
        http_get(bucket_path(project_id, "/questionnaires/#{questionnaire_id}")).json
      end

      # List all questions in a questionnaire
      def list_questions(project_id:, questionnaire_id:)
        paginate(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"))
      end

      # Create a new question in a questionnaire
      def create_question(project_id:, questionnaire_id:, **body)
        http_post(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"), body: body).json
      end

      # Get a single question by id
      def get_question(project_id:, question_id:)
        http_get(bucket_path(project_id, "/questions/#{question_id}")).json
      end

      # Update an existing question
      def update_question(project_id:, question_id:, **body)
        http_put(bucket_path(project_id, "/questions/#{question_id}"), body: body).json
      end

      # List all answers for a question
      def list_answers(project_id:, question_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers.json"))
      end

      # Create a new answer for a question
      def create_answer(project_id:, question_id:, **body)
        http_post(bucket_path(project_id, "/questions/#{question_id}/answers.json"), body: body).json
      end

      # List all people who have answered a question (answerers)
      def answerers(project_id:, question_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers/by.json"))
      end

      # Get all answers from a specific person for a question
      def by_person(project_id:, question_id:, person_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers/by/#{person_id}"))
      end

      # Update notification settings for a check-in question
      def update_notification_settings(project_id:, question_id:, **body)
        http_put(bucket_path(project_id, "/questions/#{question_id}/notification_settings.json"), body: body)
        nil
      end

      # Pause a check-in question (stops sending reminders)
      def pause(project_id:, question_id:)
        http_post(bucket_path(project_id, "/questions/#{question_id}/pause.json"))
        nil
      end

      # Resume a paused check-in question (resumes sending reminders)
      def resume(project_id:, question_id:)
        http_delete(bucket_path(project_id, "/questions/#{question_id}/pause.json"))
        nil
      end

      # Get pending check-in reminders for the current user
      def reminders()
        paginate("/my/question_reminders.json")
      end
    end
  end
end
