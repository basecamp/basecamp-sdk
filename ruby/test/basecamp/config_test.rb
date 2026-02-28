# frozen_string_literal: true

require "test_helper"

class ConfigTest < Minitest::Test
  def test_default_values
    config = Basecamp::Config.new

    assert_equal "https://3.basecampapi.com", config.base_url
    assert_equal 30, config.timeout
    assert_equal 3, config.max_retries
    assert_equal 1.0, config.base_delay
    assert_equal 0.1, config.max_jitter
    assert_equal 10_000, config.max_pages
  end

  def test_custom_values
    config = Basecamp::Config.new(
      base_url: "https://custom.api.com/",
      timeout: 60,
      max_retries: 10
    )

    assert_equal "https://custom.api.com", config.base_url # trailing slash removed
    assert_equal 60, config.timeout
    assert_equal 10, config.max_retries
  end

  def test_from_env
    ENV["BASECAMP_BASE_URL"] = "https://env.api.com"
    ENV["BASECAMP_TIMEOUT"] = "45"
    ENV["BASECAMP_MAX_RETRIES"] = "7"

    config = Basecamp::Config.from_env

    assert_equal "https://env.api.com", config.base_url
    assert_equal 45, config.timeout
    assert_equal 7, config.max_retries
  ensure
    ENV.delete("BASECAMP_BASE_URL")
    ENV.delete("BASECAMP_TIMEOUT")
    ENV.delete("BASECAMP_MAX_RETRIES")
  end

  def test_load_from_env_overrides_existing
    config = Basecamp::Config.new(timeout: 30)
    ENV["BASECAMP_TIMEOUT"] = "90"

    config.load_from_env

    assert_equal 90, config.timeout
  ensure
    ENV.delete("BASECAMP_TIMEOUT")
  end

  def test_global_config_dir
    dir = Basecamp::Config.global_config_dir

    assert dir.end_with?("basecamp")
    assert dir.include?(".config") || ENV.fetch("XDG_CONFIG_HOME", nil)
  end
end
