# frozen_string_literal: true

module Basecamp
  module Services
    # Service for automatic check-in operations.
    #
    # Checkins (also called Automatic Check-ins) are scheduled questions
    # that get sent to team members. The questionnaire contains questions,
    # and each question can have multiple answers from different people.
    #
    # @example List questions
    #   account.checkins.list_questions(project_id: 123, questionnaire_id: 456).each do |q|
    #     puts "#{q["title"]} - #{q["paused"] ? "(paused)" : ""}"
    #   end
    #
    # @example Create an answer
    #   answer = account.checkins.create_answer(
    #     project_id: 123,
    #     question_id: 456,
    #     content: "<p>Making great progress!</p>"
    #   )
    class CheckinsService < BaseService
      # Gets a questionnaire by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param questionnaire_id [Integer, String] questionnaire ID
      # @return [Hash] questionnaire data
      def get_questionnaire(project_id:, questionnaire_id:)
        http_get(bucket_path(project_id, "/questionnaires/#{questionnaire_id}.json")).json
      end

      # Lists all questions in a questionnaire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param questionnaire_id [Integer, String] questionnaire ID
      # @return [Enumerator<Hash>] questions
      def list_questions(project_id:, questionnaire_id:)
        paginate(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"))
      end

      # Gets a question by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param question_id [Integer, String] question ID
      # @return [Hash] question data
      def get_question(project_id:, question_id:)
        http_get(bucket_path(project_id, "/questions/#{question_id}.json")).json
      end

      # Creates a new question in a questionnaire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param questionnaire_id [Integer, String] questionnaire ID
      # @param title [String] question text
      # @param schedule [Hash] schedule configuration with frequency, days, hour, minute
      # @return [Hash] created question
      def create_question(project_id:, questionnaire_id:, title:, schedule:)
        body = {
          title: title,
          schedule: schedule
        }
        http_post(bucket_path(project_id, "/questionnaires/#{questionnaire_id}/questions.json"), body: body).json
      end

      # Updates an existing question.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param question_id [Integer, String] question ID
      # @param title [String, nil] new question text
      # @param schedule [Hash, nil] new schedule configuration
      # @param paused [Boolean, nil] whether the question is paused
      # @return [Hash] updated question
      def update_question(project_id:, question_id:, title: nil, schedule: nil, paused: nil)
        body = compact_params(
          title: title,
          schedule: schedule,
          paused: paused
        )
        http_put(bucket_path(project_id, "/questions/#{question_id}.json"), body: body).json
      end

      # Lists all answers for a question.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param question_id [Integer, String] question ID
      # @return [Enumerator<Hash>] answers
      def list_answers(project_id:, question_id:)
        paginate(bucket_path(project_id, "/questions/#{question_id}/answers.json"))
      end

      # Gets an answer by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param answer_id [Integer, String] answer ID
      # @return [Hash] answer data
      def get_answer(project_id:, answer_id:)
        http_get(bucket_path(project_id, "/question_answers/#{answer_id}.json")).json
      end

      # Creates a new answer for a question.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param question_id [Integer, String] question ID
      # @param content [String] answer content in HTML
      # @param group_on [String, nil] date to group the answer with (ISO 8601)
      # @return [Hash] created answer
      def create_answer(project_id:, question_id:, content:, group_on: nil)
        body = compact_params(
          content: content,
          group_on: group_on
        )
        http_post(bucket_path(project_id, "/questions/#{question_id}/answers.json"), body: body).json
      end

      # Updates an existing answer.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param answer_id [Integer, String] answer ID
      # @param content [String] updated answer content in HTML
      # @return [void]
      def update_answer(project_id:, answer_id:, content:)
        http_put(bucket_path(project_id, "/question_answers/#{answer_id}.json"), body: { content: content })
        nil
      end
    end
  end
end
