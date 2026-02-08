# frozen_string_literal: true

require "test_helper"

class Basecamp::Webhooks::VerifyTest < Minitest::Test
  def test_round_trip
    payload = '{"id":1,"kind":"todo_created"}'
    secret = "test-secret-key"

    signature = Basecamp::Webhooks::Verify.compute_signature(payload: payload, secret: secret)
    assert Basecamp::Webhooks::Verify.valid?(payload: payload, signature: signature, secret: secret)
  end

  def test_rejects_wrong_signature
    payload = '{"id":1}'
    secret = "test-secret"

    assert_not Basecamp::Webhooks::Verify.valid?(payload: payload, signature: "wrong", secret: secret)
  end

  def test_rejects_empty_secret
    assert_not Basecamp::Webhooks::Verify.valid?(payload: "data", signature: "sig", secret: "")
    assert_not Basecamp::Webhooks::Verify.valid?(payload: "data", signature: "sig", secret: nil)
  end

  def test_rejects_empty_signature
    assert_not Basecamp::Webhooks::Verify.valid?(payload: "data", signature: "", secret: "secret")
    assert_not Basecamp::Webhooks::Verify.valid?(payload: "data", signature: nil, secret: "secret")
  end

  def test_compute_signature_deterministic
    payload = "test"
    secret = "key"
    sig1 = Basecamp::Webhooks::Verify.compute_signature(payload: payload, secret: secret)
    sig2 = Basecamp::Webhooks::Verify.compute_signature(payload: payload, secret: secret)
    assert_equal sig1, sig2
  end
end
