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

    # @return [Integer] total request attempts for GET requests, including the
    #   initial request (0 sends no requests at all and raises)
    attr_accessor :max_retries

    # @return [Float] initial backoff delay in seconds
    attr_accessor :base_delay

    # @return [Float] maximum jitter to add to delays in seconds
    attr_accessor :max_jitter

    # @return [Integer] maximum pages to fetch in paginated requests
    attr_reader :max_pages

    # A validating writer, not attr_accessor: Http reads the config live at
    # every page boundary, so `config.max_pages = -1` after construction
    # replaced the validated cap with a value validate! would have refused —
    # the same second door TypeScript's compile-time-only `readonly` left
    # open. The config deliberately stays mutable (from_file → load_from_env
    # re-validation is builder-style by design); only the cap's invariant
    # must hold at assignment.
    #
    # @param value [Integer] maximum pages to fetch in paginated requests
    def max_pages=(value)
      validate_max_pages!(value)
      @max_pages = value
    end

    # Default values
    DEFAULT_BASE_URL = "https://3.basecampapi.com"
    DEFAULT_TIMEOUT = 30
    DEFAULT_MAX_RETRIES = 3
    DEFAULT_BASE_DELAY = 1.0
    DEFAULT_MAX_JITTER = 0.1
    DEFAULT_MAX_PAGES = 10_000

    # Ceiling on the backoff term (SPEC §7, "Backoff Ceiling"), in seconds.
    # Jitter is added after the clamp, so the longest single backoff sleep is
    # this plus +max_jitter+.
    MAX_BACKOFF_DELAY = 30.0

    # Smallest exponent +e+ with <tt>base_delay * 2**e >= MAX_BACKOFF_DELAY</tt>.
    #
    # Derived from the *configured* base rather than assumed. A fixed exponent
    # cap plus a trailing +min(..., MAX_BACKOFF_DELAY)+ looks equivalent and is
    # not: for a small enough base the capped product never reaches the ceiling,
    # so the delay plateaus below it forever. At +base_delay = 1e-30+ a cap of
    # 64 pins every attempt from 65 on at ~1.84e-11s — a tight retry loop, which
    # is the failure SPEC §7's ceiling exists to prevent, not an instance of it.
    #
    # Computed in the LOG domain rather than as
    # <tt>MAX_BACKOFF_DELAY / base_delay</tt>. That ratio coerces to
    # +Float::INFINITY+ for any base below ~1.67e-307, and falling back to a
    # fixed 1023 then saturates *early*: +base_delay = 1e-307+ reaches only
    # ~8.99s at exponent 1023, so returning the 30s ceiling there overstates the
    # specified term instead of tracking it. The log form has no such cliff, so
    # the numeric backstop is gone entirely rather than merely made rarer.
    #
    # @param base_delay [Float] initial backoff delay in seconds, strictly positive
    # @return [Integer] the exponent at which the term reaches the ceiling
    def self.saturating_exponent(base_delay)
      # log2 is correctly rounded but the subtraction is not, so the estimate can
      # land one either side of the true boundary. Both corrections are bounded
      # and evaluate the term with Math.ldexp, which scales directly and never
      # forms 2**e.
      exponent = [ (Math.log2(MAX_BACKOFF_DELAY) - Math.log2(base_delay)).ceil, 0 ].max
      exponent -= 1 while exponent > 0 && Math.ldexp(base_delay, exponent - 1) >= MAX_BACKOFF_DELAY
      exponent += 1 while Math.ldexp(base_delay, exponent) < MAX_BACKOFF_DELAY
      exponent
    end

    # Exponential backoff for a 1-based attempt, saturating at MAX_BACKOFF_DELAY.
    #
    # The clamp is load-bearing rather than defensive. Ruby's +**+ promotes
    # instead of overflowing, so +base_delay * (2**(attempt - 1))+ on a long
    # failure streak coerces to +Float::INFINITY+ — and +sleep(Float::INFINITY)+
    # never returns. A retry that never happens is not backoff.
    #
    # The exponent is compared against the point where the term reaches the
    # ceiling *before* the power is evaluated, so no intermediate leaves the
    # Float range and the term saturates AT the ceiling for every positive base
    # — the same contract Go, Kotlin and Swift get from comparing their
    # multiplier against <tt>MAX_BACKOFF_DELAY / base</tt> before multiplying.
    #
    # Below that point the term is scaled with +Math.ldexp+, which computes
    # <tt>base * 2**e</tt> directly. +2**e+ would be an unbounded Integer that
    # coerces to +Float::INFINITY+ long before the *product* leaves the Float
    # range, which is what forced the fixed cap this replaces.
    #
    # @param base_delay [Float] initial backoff delay in seconds
    # @param attempt [Integer] 1-based attempt number
    # @return [Float] the backoff term in seconds
    def self.saturating_backoff(base_delay, attempt)
      if base_delay <= 0
        0.0
      else
        exponent = [ attempt - 1, 0 ].max
        if exponent >= saturating_exponent(base_delay)
          MAX_BACKOFF_DELAY
        else
          [ Math.ldexp(base_delay, exponent), MAX_BACKOFF_DELAY ].min.to_f
        end
      end
    end

    # Creates a new configuration with the given options.
    #
    # @param base_url [String] API base URL
    # @param timeout [Integer] request timeout in seconds
    # @param max_retries [Integer] total request attempts for GET requests, including the initial request
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
    # - BASECAMP_MAX_RETRIES: Total request attempts for GET requests, including the initial request
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
      # UTF-8 regardless of process locale — JSON is UTF-8 (RFC 8259), and
      # LC_ALL=C would otherwise read as US-ASCII.
      data = JSON.parse(File.read(path, encoding: "UTF-8"))
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
      validate_max_pages!(@max_pages)
    end

    # Shared by validate! and the max_pages= writer. Integer-only, so a
    # boolean or Float::INFINITY is refused along with zero and negatives.
    def validate_max_pages!(value)
      raise ArgumentError, "max_pages must be positive" unless value.is_a?(Integer) && value > 0
    end

    def normalize_url(url)
      url&.chomp("/")
    end

    def localhost?(url)
      Basecamp::Security.localhost?(url)
    end
  end
end
