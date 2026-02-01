# frozen_string_literal: true

require "json"

module Basecamp
  # Configuration for the Basecamp API client.
  #
  # @example Creating config with defaults
  #   config = Basecamp::Config.new
  #
  # @example Creating config with custom values
  #   config = Basecamp::Config.new(
  #     base_url: "https://3.basecampapi.com",
  #     timeout: 60,
  #     max_retries: 3
  #   )
  #
  # @example Loading config from environment
  #   config = Basecamp::Config.from_env
  class Config
    # @return [String] API base URL
    attr_accessor :base_url

    # @return [Integer] request timeout in seconds
    attr_accessor :timeout

    # @return [Integer] maximum retry attempts for GET requests
    attr_accessor :max_retries

    # @return [Float] initial backoff delay in seconds
    attr_accessor :base_delay

    # @return [Float] maximum jitter to add to delays in seconds
    attr_accessor :max_jitter

    # @return [Integer] maximum pages to fetch in paginated requests
    attr_accessor :max_pages

    # Default values
    DEFAULT_BASE_URL = "https://3.basecampapi.com"
    DEFAULT_TIMEOUT = 30
    DEFAULT_MAX_RETRIES = 5
    DEFAULT_BASE_DELAY = 1.0
    DEFAULT_MAX_JITTER = 0.1
    DEFAULT_MAX_PAGES = 10_000

    # Creates a new configuration with the given options.
    #
    # @param base_url [String] API base URL
    # @param timeout [Integer] request timeout in seconds
    # @param max_retries [Integer] maximum retry attempts
    # @param base_delay [Float] initial backoff delay
    # @param max_jitter [Float] maximum jitter
    # @param max_pages [Integer] maximum pages to fetch
    def initialize(
      base_url: DEFAULT_BASE_URL,
      timeout: DEFAULT_TIMEOUT,
      max_retries: DEFAULT_MAX_RETRIES,
      base_delay: DEFAULT_BASE_DELAY,
      max_jitter: DEFAULT_MAX_JITTER,
      max_pages: DEFAULT_MAX_PAGES
    )
      @base_url = normalize_url(base_url)
      @timeout = timeout
      @max_retries = max_retries
      @base_delay = base_delay
      @max_jitter = max_jitter
      @max_pages = max_pages

      unless @base_url == normalize_url(DEFAULT_BASE_URL) || localhost?(@base_url)
        Basecamp::Security.require_https!(@base_url, "base URL")
      end
      validate!
    end

    # Creates a Config from environment variables.
    #
    # Environment variables:
    # - BASECAMP_BASE_URL: API base URL
    # - BASECAMP_TIMEOUT: Request timeout in seconds
    # - BASECAMP_MAX_RETRIES: Maximum retry attempts
    #
    # @return [Config]
    def self.from_env
      new(
        base_url: ENV.fetch("BASECAMP_BASE_URL", DEFAULT_BASE_URL),
        timeout: ENV.fetch("BASECAMP_TIMEOUT", DEFAULT_TIMEOUT).to_i,
        max_retries: ENV.fetch("BASECAMP_MAX_RETRIES", DEFAULT_MAX_RETRIES).to_i
      )
    end

    # Loads configuration from a JSON file, with environment overrides.
    #
    # @param path [String] path to JSON config file
    # @return [Config]
    def self.from_file(path)
      data = JSON.parse(File.read(path))
      config = new(
        base_url: data["base_url"] || DEFAULT_BASE_URL,
        timeout: data["timeout"] || DEFAULT_TIMEOUT,
        max_retries: data["max_retries"] || DEFAULT_MAX_RETRIES
      )
      config.load_from_env
      config
    rescue Errno::ENOENT
      from_env
    end

    # Loads environment variable overrides into this config.
    # @return [self]
    def load_from_env
      @base_url = normalize_url(ENV["BASECAMP_BASE_URL"]) if ENV["BASECAMP_BASE_URL"]
      @timeout = ENV["BASECAMP_TIMEOUT"].to_i if ENV["BASECAMP_TIMEOUT"]
      @max_retries = ENV["BASECAMP_MAX_RETRIES"].to_i if ENV["BASECAMP_MAX_RETRIES"]
      Basecamp::Security.require_https!(@base_url, "base URL") unless localhost?(@base_url)
      validate!
      self
    end

    # Returns the default global config directory.
    # @return [String]
    def self.global_config_dir
      config_dir = ENV["XDG_CONFIG_HOME"] || File.join(Dir.home, ".config")
      File.join(config_dir, "basecamp")
    end

    private

    def validate!
      raise ArgumentError, "timeout must be positive" unless @timeout.is_a?(Numeric) && @timeout > 0
      raise ArgumentError, "max_retries must be non-negative" unless @max_retries.is_a?(Integer) && @max_retries >= 0
      raise ArgumentError, "max_pages must be positive" unless @max_pages.is_a?(Integer) && @max_pages > 0
    end

    def normalize_url(url)
      url&.chomp("/")
    end

    def localhost?(url)
      Basecamp::Security.localhost?(url)
    end
  end
end
