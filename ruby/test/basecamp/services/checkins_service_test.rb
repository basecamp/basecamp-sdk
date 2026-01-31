# frozen_string_literal: true

# Tests for the CheckinsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Single-resource paths without .json (get_questionnaire, get_question, etc.)
# - No client-side validation (API validates)

require "test_helper"

class CheckinsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_questionnaire(id: 1)
    {
      "id" => id,
      "name" => "Automatic Check-ins",
      "questions_count" => 3
    }
  end

  def sample_question(id: 1, title: "What did you work on today?")
    {
      "id" => id,
      "title" => title,
      "paused" => false,
      "schedule" => { "frequency" => "every_day", "hour" => 16, "minute" => 0 }
    }
  end

  def sample_answer(id: 1, content: "<p>Making great progress!</p>")
    {
      "id" => id,
      "content" => content,
      "creator" => { "id" => 1, "name" => "Test User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_get_questionnaire
    # Generated service: /questionnaires/{id} without .json
    stub_get("/12345/buckets/100/questionnaires/200", response_body: sample_questionnaire(id: 200))

    questionnaire = @account.checkins.get_questionnaire(project_id: 100, questionnaire_id: 200)

    assert_equal 200, questionnaire["id"]
    assert_equal "Automatic Check-ins", questionnaire["name"]
  end

  def test_list_questions
    stub_get("/12345/buckets/100/questionnaires/200/questions.json",
             response_body: [ sample_question, sample_question(id: 2, title: "Any blockers?") ])

    questions = @account.checkins.list_questions(project_id: 100, questionnaire_id: 200).to_a

    assert_equal 2, questions.length
    assert_equal "What did you work on today?", questions[0]["title"]
  end

  def test_get_question
    # Generated service: /questions/{id} without .json
    stub_get("/12345/buckets/100/questions/200", response_body: sample_question(id: 200))

    question = @account.checkins.get_question(project_id: 100, question_id: 200)

    assert_equal 200, question["id"]
    assert_equal "What did you work on today?", question["title"]
  end

  def test_create_question
    new_question = sample_question(id: 999, title: "New question")
    stub_post("/12345/buckets/100/questionnaires/200/questions.json", response_body: new_question)

    question = @account.checkins.create_question(
      project_id: 100,
      questionnaire_id: 200,
      title: "New question",
      schedule: { frequency: "every_week", days: [ 1 ], hour: 9, minute: 0 }
    )

    assert_equal 999, question["id"]
    assert_equal "New question", question["title"]
  end

  def test_update_question
    # Generated service: /questions/{id} without .json
    updated_question = sample_question(id: 200, title: "Updated question")
    stub_put("/12345/buckets/100/questions/200", response_body: updated_question)

    question = @account.checkins.update_question(
      project_id: 100,
      question_id: 200,
      title: "Updated question",
      paused: false
    )

    assert_equal "Updated question", question["title"]
  end

  def test_list_answers
    stub_get("/12345/buckets/100/questions/200/answers.json",
             response_body: [ sample_answer, sample_answer(id: 2, content: "<p>All good!</p>") ])

    answers = @account.checkins.list_answers(project_id: 100, question_id: 200).to_a

    assert_equal 2, answers.length
    assert_equal "<p>Making great progress!</p>", answers[0]["content"]
  end

  def test_get_answer
    # Generated service: /question_answers/{id} without .json
    stub_get("/12345/buckets/100/question_answers/200", response_body: sample_answer(id: 200))

    answer = @account.checkins.get_answer(project_id: 100, answer_id: 200)

    assert_equal 200, answer["id"]
    assert_equal "<p>Making great progress!</p>", answer["content"]
  end

  def test_create_answer
    new_answer = sample_answer(id: 999, content: "<p>New answer</p>")
    stub_post("/12345/buckets/100/questions/200/answers.json", response_body: new_answer)

    answer = @account.checkins.create_answer(
      project_id: 100,
      question_id: 200,
      content: "<p>New answer</p>",
      group_on: "2024-01-15"
    )

    assert_equal 999, answer["id"]
    assert_equal "<p>New answer</p>", answer["content"]
  end

  def test_update_answer
    # UpdateAnswer now returns void (204 No Content)
    stub_request(:put, "https://3.basecampapi.com/12345/buckets/100/question_answers/200")
      .to_return(status: 204, body: "")

    result = @account.checkins.update_answer(
      project_id: 100,
      answer_id: 200,
      content: "<p>Updated answer</p>"
    )

    assert_nil result
  end
end
