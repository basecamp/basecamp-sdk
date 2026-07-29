# frozen_string_literal: true

require "faraday"
require "json"
require "timeout"
require "uri"

module Basecamp
  module Oauth
    # RFC 8628 device authorization grant — request, poll, and orchestrate.
    #
    # {request_device_authorization} obtains a device/user code pair;
    # {poll_device_token} runs the §3.5 polling loop against the token endpoint;
    # {perform_device_login} guards capability on an already-selected {Config},
    # surfaces the code through a display hook, and polls. All device-auth and
    # token requests are TLS-guarded (SPEC.md §9). The polling clock and sleeper
    # are injectable so tests run without real delays.
    module DeviceFlow
      # URN grant type for the device authorization grant.
      DEVICE_CODE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code"

      # Default polling interval when the server omits +interval+ (RFC 8628 §3.2).
      DEFAULT_INTERVAL_SECONDS = 5

      # +slow_down+ bumps the interval by this many seconds, sustained (§3.5).
      SLOW_DOWN_INCREMENT_SECONDS = 5

      # Default per-request timeout for every device-flow HTTP round-trip. Also the
      # fallback Fetcher.normalize_timeout uses for an invalid device timeout, so an
      # invalid value can't silently borrow discovery's shorter budget.
      DEVICE_REQUEST_TIMEOUT = 30

      # Granularity (seconds) for polling the +cancelled+ probe while waiting.
      CANCEL_POLL_INTERVAL_SECONDS = 0.1

      # Cap on exponential backoff after connection timeouts.
      MAX_BACKOFF_SECONDS = 60

      # Ceiling for +expires_in+/+interval+: 2147483 s (~24.8 days) is the
      # largest whole-second duration whose millisecond form fits a 32-bit
      # signed timer. Shared across all five SDKs (SPEC.md) — an unbounded
      # value such as 1e100 is a malformed response, not a schedulable deadline.
      MAX_DEVICE_SECONDS = 2_147_483

      # Ceiling for an OAuth token's +expires_in+ (2_147_483_647 s ~= 68 years):
      # cross-runtime safe and vastly beyond any realistic token lifetime.
      # Unlike +MAX_DEVICE_SECONDS+ this bounds +Time+ arithmetic rather than a
      # timer, so a non-finite value (+1e400+ parses to +Float::INFINITY+, which
      # would raise a raw +FloatDomainError+) or an absurd one is a malformed
      # response — never a schedulable deadline. Shared across all five SDKs.
      MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647

      # Monotonic clock (seconds). Injectable so tests can advance time.
      DEFAULT_CLOCK = -> { Process.clock_gettime(Process::CLOCK_MONOTONIC) }

      # Real sleeper (seconds). Injectable so tests assert the wait schedule.
      DEFAULT_SLEEPER = ->(seconds) { sleep(seconds) }

      # Cooperative cancellation probe. Injectable; default never cancels.
      DEFAULT_CANCELLED = -> { false }

      class << self
        # Requests a device/user code pair (RFC 8628 §3.1–3.2).
        #
        # POSTs +client_id+ and, only when set, +scope+ (an omitted scope lets the
        # server apply its default, +read+). Validates that the codes are present,
        # +expires_in+ is positive, and +interval+ (default 5) is positive.
        #
        # @param device_authorization_endpoint [String] the endpoint from discovery
        # @param client_id [String] the public client id (e.g. +basecamp-cli+)
        # @param scope [String, nil] requested scope; omitted from the request when nil
        # @param http_client [Faraday::Connection, nil] HTTP client (default if nil)
        # @param timeout [Integer] request timeout in seconds
        # @param max_body_bytes [Integer] bounded read cap in bytes
        # @return [DeviceAuthorization]
        # @raise [OauthError] +validation+ on a missing client id or a redirect-
        #   following injected client; +api_error+ on a non-2xx response, oversized
        #   body, or invalid metadata
        # @raise [DeviceFlowError] +:transport+ on a network failure
        def request_device_authorization(
          device_authorization_endpoint:, client_id:, scope: nil,
          http_client: nil, timeout: DEVICE_REQUEST_TIMEOUT, max_body_bytes: Fetcher::DEFAULT_MAX_BODY_BYTES
        )
          Basecamp::Security.require_https_unless_localhost!(device_authorization_endpoint, "device authorization endpoint")
          raise OauthError.new("validation", "Client ID is required for device authorization") if client_id.to_s.empty?
          # SSRF: an injected client must not chase an attacker-controlled Location,
          # exactly as the discovery hops require (SPEC.md §16).
          Fetcher.ensure_redirects_suppressed!(http_client) if http_client

          params = { "client_id" => client_id }
          # Omit scope entirely when unset OR blank so the server applies its
          # default (read) — Ruby treats "" as truthy, so guard on emptiness too.
          params["scope"] = scope unless scope.nil? || scope.empty?

          # Normalize ONCE at operation entry and thread the SAME value to both the
          # client construction and the request, so a non-finite/non-positive input
          # cannot leave the socket timeout unbounded on the default-built client.
          # The body cap gets the same discipline: an invalid value (nil, Infinity,
          # negative) would disable the streaming memory bound entirely.
          timeout = Fetcher.normalize_timeout(timeout, default: DEVICE_REQUEST_TIMEOUT)
          max_body_bytes = Fetcher.normalize_body_cap(max_body_bytes)
          client = http_client || build_client(timeout)
          status, body = begin
            post_form(
              client, device_authorization_endpoint, params,
              timeout: timeout, max_body_bytes: max_body_bytes,
              # A non-2xx device-auth response is a hard failure whose body is unused.
              skip_status: ->(s) { !(200..299).cover?(s) }
            )
          rescue Faraday::Error => e
            raise DeviceFlowError.new(:transport, "Device authorization request failed: #{e.message}")
          end

          parse_device_authorization(status, body)
        end

        # Polls the token endpoint until the user approves, denies, or the codes
        # expire (RFC 8628 §3.4–3.5).
        #
        # Waits at least +interval+ seconds between polls against a MONOTONIC
        # deadline. Handles +authorization_pending+ (keep polling), sustained
        # +slow_down+ (+5s for this and every later poll), +access_denied+ and
        # +expired_token+ (terminal), connection timeouts (exponential backoff),
        # and cooperative cancellation.
        #
        # @param token_endpoint [String] the token endpoint from discovery
        # @param client_id [String] the public client id
        # @param device_code [String] the device code from {request_device_authorization}
        # @param interval [Integer] polling interval in seconds
        # @param expires_in [Numeric] code lifetime in seconds until the monotonic
        #   deadline; may be fractional (perform_device_login passes the remaining
        #   lifetime after deducting display-hook time)
        # @param clock [#call] monotonic clock returning seconds
        # @param sleeper [#call] receives the wait in seconds
        # @param cancelled [#call] cancellation probe; a truthy result ends the flow
        # @param http_client [Faraday::Connection, nil] HTTP client (default if nil)
        # @param timeout [Integer] per-request timeout in seconds
        # @param max_body_bytes [Integer] bounded read cap in bytes
        # @return [Token]
        # @raise [DeviceFlowError] +:access_denied+, +:expired+, +:transport+, or
        #   +:cancelled+
        # @raise [OauthError] +api_error+ on an unrecognized token error, oversized
        #   body, or +validation+ on a redirect-following injected client; +usage+
        #   on an out-of-range +interval+/+expires_in+
        def poll_device_token(
          token_endpoint:, client_id:, device_code:, interval:, expires_in:,
          clock: DEFAULT_CLOCK, sleeper: DEFAULT_SLEEPER, cancelled: DEFAULT_CANCELLED,
          http_client: nil, timeout: DEVICE_REQUEST_TIMEOUT, max_body_bytes: Fetcher::DEFAULT_MAX_BODY_BYTES,
          deadline_at: nil
        )
          Basecamp::Security.require_https_unless_localhost!(token_endpoint, "token endpoint")
          Fetcher.ensure_redirects_suppressed!(http_client) if http_client

          # Caller-input sanity for this public entry point (usage, not the RFC
          # response validation request_device_authorization applies): a nil or
          # non-numeric duration would raise NoMethodError/TypeError below, a
          # non-finite expires_in builds a deadline that NEVER passes (an unbounded
          # poll loop), and an oversized/non-positive interval is not a schedulable
          # wait. Fractional values are accepted — perform_device_login legitimately
          # passes a fractional remaining lifetime after deducting display-hook
          # time. Mirrors the Go/TS/Python/Kotlin caller guards.
          unless valid_device_seconds?(expires_in)
            raise OauthError.new(
              "usage",
              "poll_device_token: expires_in must be a positive number of seconds " \
              "no greater than #{MAX_DEVICE_SECONDS}"
            )
          end
          # The polling interval is additionally whole seconds (RFC 8628),
          # matching the response validation and the integer-typed Go/Kotlin
          # APIs — a fractional interval (0.001) would otherwise permit ~1000
          # polls per second. real? in valid_device_seconds? screens Complex
          # before the modulo runs.
          unless valid_device_seconds?(interval) && (interval % 1).zero?
            raise OauthError.new(
              "usage",
              "poll_device_token: interval must be a positive whole number of seconds " \
              "no greater than #{MAX_DEVICE_SECONDS}"
            )
          end

          interval_seconds = interval
          backoff_seconds = interval_seconds
          # An absolute issuance-anchored deadline (perform_device_login
          # passes issued_at + expires_in) beats re-anchoring: clock time
          # elapsing between the caller's remaining-lifetime computation and
          # this entry — a process suspension above all — must never be handed
          # back to the code. It can only SHORTEN the validated lifetime.
          deadline =
            if deadline_at.nil?
              sample_clock(clock, "poll_device_token") + expires_in
            else
              unless deadline_at.is_a?(Numeric) && deadline_at.real? \
                  && deadline_at.to_f.finite? && deadline_at <= sample_clock(clock, "poll_device_token") + expires_in
                raise OauthError.new(
                  "usage",
                  "poll_device_token deadline_at must be a finite monotonic timestamp " \
                  "no later than expires_in seconds from now"
                )
              end
              deadline_at
            end

          # Normalize ONCE, outside the polling loop, and reuse for the client and
          # every per-poll request (see request_device_authorization). The body cap
          # gets the same discipline — an invalid value would disable the bound.
          timeout = Fetcher.normalize_timeout(timeout, default: DEVICE_REQUEST_TIMEOUT)
          max_body_bytes = Fetcher.normalize_body_cap(max_body_bytes)
          client = http_client || build_client(timeout)
          params = {
            "grant_type" => DEVICE_CODE_GRANT_TYPE,
            "device_code" => device_code,
            "client_id" => client_id
          }

          loop do
            raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

            # Check the monotonic deadline BEFORE waiting, then clamp the wait so a
            # long interval or timeout backoff can never overshoot expiry. The
            # per-request timeout (set on every request in +post_form+) bounds a
            # stalled socket, so nothing here blows past the deadline. The wait is
            # the LARGER of the server-driven interval and the transient timeout
            # backoff — the two schedules stay separate so a backoff can drain
            # back down to the server interval once round-trips resume.
            now = sample_clock(clock, "poll_device_token")
            raise DeviceFlowError.new(:expired, "Device code expired before authorization completed") if now >= deadline

            wait_cancellable([ [ interval_seconds, backoff_seconds ].max, deadline - now ].min, cancelled, sleeper)

            raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

            post_remaining = deadline - sample_clock(clock, "poll_device_token")
            raise DeviceFlowError.new(:expired, "Device code expired before authorization completed") if post_remaining <= 0

            outcome = begin
              # Bound the request by the REMAINING code lifetime as well as the
              # per-request timeout: near expiry, a stalled token POST must not
              # hold the flow past the monotonic deadline for the full budget.
              post_device_token(client, token_endpoint, params,
                timeout: [ timeout, post_remaining ].min, max_body_bytes: max_body_bytes)
            rescue Faraday::TimeoutError
              # A connection timeout is transient: back off exponentially and
              # keep polling rather than ending the flow. Only the backoff grows —
              # the server-driven interval is left untouched so it can govern
              # again once a round-trip completes. The next wait is still clamped
              # to the deadline at the top of the loop.
              backoff_seconds = [ backoff_seconds * 2, MAX_BACKOFF_SECONDS ].min
              next
            rescue Faraday::Error => e
              # Cancellation-beats-classification on the error path too: a
              # cancel that flipped while the doomed request was in flight must
              # surface as cancelled, not as the transport fault it raised.
              raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

              raise DeviceFlowError.new(:transport, "Device token poll failed: #{e.message}")
            end

            # Re-check cancellation the moment the round-trip completes: the
            # sync POST cannot observe the probe while in flight (bounded only
            # by its timeout), so a cancel raised mid-request must surface
            # here — never a token returned after the caller asked to stop.
            raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

            # ANY completed HTTP round-trip resets the timeout backoff to the
            # current server-driven interval.
            backoff_seconds = interval_seconds

            kind, value, status = outcome
            return value if kind == :token

            case value
            when "authorization_pending"
              next
            when "slow_down"
              interval_seconds += SLOW_DOWN_INCREMENT_SECONDS
              # Re-sync the backoff to the GROWN interval (the reset above used the
              # pre-increment value) so a later timeout doubles from the new
              # interval, not the stale one.
              backoff_seconds = interval_seconds
            when "access_denied"
              raise DeviceFlowError.new(:access_denied, "The authorization request was denied")
            when "expired_token"
              raise DeviceFlowError.new(:expired, "Device code expired before authorization completed")
            else
              raise OauthError.new("api_error", "Device token request failed: #{value}", http_status: status)
            end
          end
        end

        # Runs the full device authorization grant against an already-selected
        # {Config} (RFC 8628; SPEC.md §16).
        #
        # The capability guard requires BOTH +device_authorization_endpoint+ AND
        # the device_code grant in +grant_types_supported+; otherwise it raises
        # +:unavailable+ before any request is issued.
        #
        # @param config [Config] the already-selected authorization-server config
        # @param client_id [String] the public client id
        # @param display [#call] receives the {DeviceAuthorization} once, before polling
        # @param scope [String, nil] requested scope; omitted when nil
        # @param clock [#call] monotonic clock returning seconds
        # @param sleeper [#call] receives the wait in seconds
        # @param cancelled [#call] cancellation probe
        # @param http_client [Faraday::Connection, nil] HTTP client (default if nil)
        # @param timeout [Integer] request timeout in seconds
        # @return [Token]
        # @raise [DeviceFlowError] +:unavailable+ when the config cannot do device
        #   flow; other reasons on denial/expiry/transport/cancellation
        # @raise [OauthError] +usage+ when +display+ is not callable (checked
        #   before any request — DeviceFlowError subclasses OauthError, so a
        #   rescue of the former alone would miss this)
        def perform_device_login(
          config:, client_id:, display:, scope: nil,
          clock: DEFAULT_CLOCK, sleeper: DEFAULT_SLEEPER, cancelled: DEFAULT_CANCELLED,
          http_client: nil, timeout: DEVICE_REQUEST_TIMEOUT, max_body_bytes: Fetcher::DEFAULT_MAX_BODY_BYTES
        )
          unless device_grant_available?(config)
            raise DeviceFlowError.new(
              :unavailable,
              "The selected authorization server does not support the device authorization grant"
            )
          end

          # A non-callable display is a usage error, not a late NoMethodError:
          # it is the only mechanism surfacing the verification code, so
          # dereferencing it AFTER the request would mint a code nobody can
          # approve. Reject before any network activity (matching Go).
          unless display.respond_to?(:call)
            raise OauthError.new("usage", "perform_device_login requires a callable display hook")
          end

          # Honor a cancellation raised BEFORE the flow does any work: the
          # sync authorization POST cannot observe the probe in flight, so
          # without this entry check an already-cancelled flow still performs
          # the request and invokes the display hook.
          raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

          auth = begin
            request_device_authorization(
              device_authorization_endpoint: config.device_authorization_endpoint,
              client_id: client_id, scope: scope,
              http_client: http_client, timeout: timeout, max_body_bytes: max_body_bytes
            )
          rescue DeviceFlowError, OauthError
            # Cancellation-beats-classification on the error path too: a cancel
            # that flipped while the authorization request was in flight wins
            # over whatever fault the doomed request raised.
            raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

            raise
          end

          # Anchor the code's lifetime at ISSUANCE — the response's arrival,
          # per SPEC §16 — before the display hook, so a slow display eats
          # into the deadline instead of resetting it. Expiry past this point
          # is arbitrated by the server (expired_token), so receipt-anchoring
          # fails safe.
          issued_at = sample_clock(clock, "perform_device_login")

          # Re-check after the round-trip AND after the anchor sample (itself
          # a cancellation-capable callback seam), before surfacing the code:
          # a cancel set in either window must not reach the display hook.
          raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

          display.call(auth)
          remaining = auth.expires_in - (sample_clock(clock, "perform_device_login") - issued_at)
          # Cancellation raised DURING the display hook OR during the clock
          # sample just above (the clock is a cancellation-capable callback
          # seam, exactly like the issuance anchor) wins over expiry:
          # checked after the sample and before the expiry branch, matching
          # the TS orchestrator's ordering.
          raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

          if remaining <= 0
            raise DeviceFlowError.new(:expired, "Device code expired before authorization completed")
          end

          poll_device_token(
            token_endpoint: config.token_endpoint,
            client_id: client_id, device_code: auth.device_code,
            interval: auth.interval, expires_in: remaining,
            # The EXACT issuance-anchored deadline: a clock advance between
            # the remaining computation above and the poller's entry must not
            # extend the code lifetime.
            deadline_at: issued_at + auth.expires_in,
            clock: clock, sleeper: sleeper, cancelled: cancelled,
            http_client: http_client, timeout: timeout, max_body_bytes: max_body_bytes
          )
        end

        private

          # Capability guard: BOTH a present endpoint AND the advertised grant
          # type. +grant_types_supported+ must be an Array checked for exact
          # membership — a String (or a superstring containing the grant URN)
          # must never satisfy the guard via +String#include?+ substring matching.
          # A blank endpoint is treated as absent.
          def device_grant_available?(config)
            has_endpoint = !config.device_authorization_endpoint.to_s.strip.empty?
            grants = config.grant_types_supported
            has_endpoint && grants.is_a?(Array) && grants.include?(DEVICE_CODE_GRANT_TYPE)
          end

          # A positive, finite, real number of seconds within the shared device
          # ceiling. +real?+ gates out Complex before +finite?+/+positive?+ (which
          # Complex does not define), matching {Fetcher.valid_timeout?}.
          # EVERY sample of the injected clock seam is validated: a String or
          # Complex sample would raise a raw TypeError out of the deadline
          # arithmetic, and a NaN sample makes every deadline comparison
          # permanently false — polling an authorization_pending endpoint
          # forever. A malformed sample is the typed usage fault instead.
          def sample_clock(clock, entry)
            value = clock.call
            unless value.is_a?(Numeric) && value.real? && value.to_f.finite?
              raise OauthError.new("usage", "#{entry} clock must return a finite number of seconds")
            end
            value
          end

          def valid_device_seconds?(value)
            value.is_a?(Numeric) && value.real? && value.finite? && value.positive? && value <= MAX_DEVICE_SECONDS
          end

          def build_client(timeout)
            Fetcher.build_client(timeout)
          end

          # Waits +seconds+ while observing cancellation DURING the wait. A plain
          # +sleep+ is not interruptible, so a cancellation set mid-wait would not be
          # noticed until the whole (possibly grown +slow_down+) interval elapses.
          # With the default no-op probe a single sleep preserves the exact wait
          # schedule; only a real probe needs the finer-grained interrupt polling —
          # matching the ctx/AbortSignal/coroutine cancellation Go/TS/Kotlin waits have.
          def wait_cancellable(seconds, cancelled, sleeper)
            if cancelled.equal?(DEFAULT_CANCELLED)
              sleeper.call(seconds)
              return
            end

            remaining = seconds
            while remaining.positive?
              raise DeviceFlowError.new(:cancelled, "Device flow cancelled") if cancelled.call

              step = [ remaining, CANCEL_POLL_INTERVAL_SECONDS ].min
              sleeper.call(step)
              remaining -= step
            end
          end

          # POSTs a form body and reads the response under the same bounded/
          # streaming cap as discovery (SPEC.md §9): the +on_data+ proc aborts the
          # read the moment the accumulated size exceeds the cap, so an oversized
          # response is never fully buffered. Returns +[status, body]+. The
          # per-request timeout is always set here — even on an injected client —
          # so a stalled socket can't hang the poll. A real adapter streams to
          # +on_data+ (leaving +response.body+ empty); a test double that ignores
          # the block falls back to the buffered body, still size-capped.
          def post_form(client, url, params, timeout:, max_body_bytes:, skip_status: nil)
            # +timeout+ is already normalized by the caller (request/poll entry). The
            # wall-clock deadline bounds the WHOLE read: req.options.timeout below
            # bounds only each socket read and resets on every on_data chunk, so a
            # slow-drip peer could otherwise hang a device request past the timeout /
            # code expiry while staying under the cap. +skip_status+ stops the read
            # for a status whose body the caller doesn't use (non-2xx / 3xx).
            deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout
            chunks, on_data = Fetcher.bounded_reader(max_body_bytes, deadline: deadline, skip_status: skip_status)
            # Timeout.timeout wraps the WHOLE call: the per-read timeout resets
            # on every socket read, so a peer dripping HEADER bytes under it
            # would otherwise hold the POST open indefinitely — on_data (a
            # body callback) never runs during the header phase, leaving
            # nothing else to enforce the wall clock on an injected client.
            # The window is the REMAINING budget, not a fresh +timeout+: time
            # spent before dispatch (descheduling included) already counts
            # against the deadline, so the request can never run past it.
            remaining = deadline - Process.clock_gettime(Process::CLOCK_MONOTONIC)
            raise Faraday::TimeoutError, "request budget exhausted before dispatch" if remaining <= 0

            response = Timeout.timeout(remaining, Faraday::TimeoutError) do
              client.post(url) do |req|
                req.headers["Content-Type"] = "application/x-www-form-urlencoded"
                req.headers["Accept"] = "application/json"
                req.body = URI.encode_www_form(params)
                req.options.timeout = timeout
                req.options.open_timeout = timeout
                req.options.on_data = on_data
              end
            end

            # Status-first backstop on the completed response. The +on_data+ SkipBody
            # fast-path only fires when the adapter streams AND passes +env+ (Faraday
            # >= 2.5). A buffered adapter that ignores +on_data+, an older Faraday
            # (2.0–2.4) that omits +env+, or a header-only response reaches here with
            # the skip-status body un-skipped — re-apply +skip_status+ to the final
            # +response.status+ so a 3xx/non-2xx fault is classified by status for
            # every client shape and supported Faraday version, never buffered into
            # a size-cap error the caller must then untangle.
            #
            # Known residual: Faraday exposes no headers-time callback, so a response
            # whose headers arrive but whose body then stalls PAST the read timeout
            # never returns from +client.post+ — it surfaces as a bounded transport
            # timeout (request path: :transport; poll path: backoff, capped by code
            # expiry) rather than this status-first classification. That degradation
            # is bounded and redirect-safe (3xx Locations are never followed); exact
            # headers-first semantics for it belong to the transport primitive shared
            # with discovery, tracked as a pre-go-live hardening follow-up.
            return [ response.status, "" ] if skip_status && skip_status.call(response.status)

            # Timeout.timeout's interrupt can be delivered late: a response
            # whose final byte lands just before the deadline but whose block
            # returns just after it would otherwise be accepted past the wall
            # clock. Status-first classification above outranks this re-check
            # (a completed skip-status is definitive); everything else past
            # the deadline is refused as the same transport-shaped timeout.
            raise Faraday::TimeoutError, "response completed after the deadline" \
              if Process.clock_gettime(Process::CLOCK_MONOTONIC) > deadline

            body =
              if chunks.empty?
                raw = response.body.to_s
                raise Fetcher::BodyTooLarge if raw.bytesize > max_body_bytes

                raw
              else
                chunks.join
              end

            [ response.status, body.dup.force_encoding(Encoding::UTF_8) ]
          rescue Fetcher::SkipBody => e
            # The body was intentionally not drained (its status is unused) — return
            # it empty and let the caller classify by status.
            [ e.status, "" ]
          rescue Fetcher::BodyTooLarge
            raise OauthError.new("api_error", "Device flow response exceeds size cap")
          rescue Fetcher::ReadDeadlineExceeded
            # Surface as a Faraday timeout so the caller's existing transport/timeout
            # rescues classify it (request → :transport, poll → backoff) — a slow-drip
            # read is a transport timeout, not an api_error.
            raise Faraday::TimeoutError, "Device flow read exceeded the timeout deadline"
          end

          def parse_device_authorization(status, body)
            # Check status BEFORE parsing (as discovery does): a non-2xx here is a
            # hard failure with no OAuth error semantics, so a non-JSON error body
            # (common for 500/502) must surface as "failed with status …", not a
            # misleading parse error. The token poll is different — it parses non-2xx
            # bodies to read authorization_pending / slow_down.
            unless (200..299).cover?(status)
              raise OauthError.new(
                "api_error",
                "Device authorization failed with status #{status}",
                http_status: status
              )
            end

            data = parse_json_object(body, status, "device authorization")
            build_device_authorization(data, status)
          end

          # Parse a JSON object, type-checking rather than trusting truthiness: a
          # non-object body (array, number, string) is malformed and must not be
          # indexed as if it were a Hash.
          def parse_json_object(body, status, label)
            data = JSON.parse(body)
            unless data.is_a?(Hash)
              raise OauthError.new("api_error", "Invalid #{label} response: not a JSON object", http_status: status)
            end

            data
          rescue JSON::ParserError
            # cause: nil — the parser error's message embeds the offending
            # input, and these bodies carry device codes and access tokens:
            # full_message and cause-aware loggers must not disclose them.
            raise OauthError.new("api_error", "Failed to parse #{label} response", http_status: status), cause: nil
          end

          # Every validation error carries the (2xx) status so a malformed success
          # body is diagnosable as such — uniform with the token raises and the
          # other SDKs.
          def build_device_authorization(data, status)
            device_code = data["device_code"]
            user_code = data["user_code"]
            verification_uri = data["verification_uri"]
            # Type-check the required strings — don't coerce via +.to_s+, which
            # would silently accept a numeric or boolean field as a "present" code.
            unless [ device_code, user_code, verification_uri ].all? { |field| field.is_a?(String) && !field.empty? }
              raise OauthError.new(
                "api_error",
                "Invalid device authorization response: missing or malformed required fields",
                http_status: status
              )
            end

            complete = data["verification_uri_complete"]
            unless complete.nil? || complete.is_a?(String)
              raise OauthError.new(
                "api_error",
                "Invalid device authorization response: verification_uri_complete must be a string",
                http_status: status
              )
            end

            expires_in = data["expires_in"]
            unless positive_integer_seconds?(expires_in)
              raise OauthError.new(
                "api_error",
                "Invalid device authorization response: expires_in must be a positive integer " \
                "no greater than #{MAX_DEVICE_SECONDS}",
                http_status: status
              )
            end

            DeviceAuthorization.new(
              device_code: device_code,
              user_code: user_code,
              verification_uri: verification_uri,
              verification_uri_complete: complete,
              # Coerce an integer-valued Float (900.0) to Integer so the model always
              # carries whole seconds, matching +build_token+ and the other SDKs.
              expires_in: expires_in.to_i,
              interval: resolve_interval(data["interval"], status)
            )
          end

          # Default 5 when absent; any present value must be a positive integer
          # number of seconds (RFC 8628). Integer-valued floats (5.0) are fine;
          # fractional values (2.5) are malformed and rejected for cross-SDK parity.
          def resolve_interval(raw, status)
            if raw.nil?
              DEFAULT_INTERVAL_SECONDS
            elsif positive_integer_seconds?(raw)
              # Coerce an integer-valued Float (5.0) to Integer for whole-second parity.
              raw.to_i
            else
              raise OauthError.new(
                "api_error",
                "Invalid device authorization response: interval must be a positive integer " \
                "no greater than #{MAX_DEVICE_SECONDS}",
                http_status: status
              )
            end
          end

          # RFC 8628 durations are integer seconds. Accept a positive Numeric with
          # no fractional part (5 or 5.0) up to MAX_DEVICE_SECONDS; reject
          # fractional (2.5), oversized (1e100), and non-numeric values.
          def positive_integer_seconds?(value)
            value.is_a?(Numeric) && value.positive? && value <= MAX_DEVICE_SECONDS && (value % 1).zero?
          end

          # Returns +[:token, Token, status]+ on success or +[:error, code, status]+
          # when the server reports an OAuth error. Raises +api_error+ on a
          # malformed, redirecting, or 2xx-but-tokenless response.
          def post_device_token(client, token_endpoint, params, timeout:, max_body_bytes:)
            status, body = post_form(
              client, token_endpoint, params,
              timeout: timeout, max_body_bytes: max_body_bytes,
              # A 3xx token response is a redirect fault whose body is unused; a 4xx
              # body IS read (it carries authorization_pending/slow_down).
              skip_status: ->(s) { !(s == 200 || (400..499).cover?(s)) }
            )

            # A redirect is never a valid token-endpoint outcome: it is not
            # followed (redirect-following clients are rejected up front), and
            # its body must not be classified as an OAuth error — a 3xx carrying
            # {"error":"authorization_pending"} would otherwise poll forever.
            if (300..399).cover?(status)
              raise OauthError.new(
                "api_error",
                "Device token request failed: unexpected redirect (status #{status})",
                http_status: status
              )
            end

            # Every remaining status outside 200 and 4xx is terminal WITHOUT
            # its body (only a 200 carries the token and only a 4xx the OAuth
            # error code) — the skip_status above already refused the body, so
            # a 201/500 that stalls while streaming can never time out
            # mid-read and be retried as a transient failure until expiry.
            unless status == 200 || (400..499).cover?(status)
              raise OauthError.new(
                "api_error",
                "Device token request failed with status #{status}",
                http_status: status
              )
            end

            data = parse_json_object(body, status, "device token")

            # Exactly HTTP 200, not any 2xx: RFC 8628/6749 token responses are
            # 200, and SPEC.md §16 pins the contract. A nonstandard 201/202
            # carrying an access_token must not prematurely complete polling —
            # it falls through to the OAuth-error path below and terminates as
            # api_error (http_<status>).
            if status == 200
              access_token = data["access_token"]
              # Require a genuine non-empty String — not a truthy/coercible value.
              unless access_token.is_a?(String) && !access_token.empty?
                raise OauthError.new("api_error", "Device token response missing access_token", http_status: status)
              end

              [ :token, build_token(data, status), status ]
            else
              # Recognize OAuth protocol error codes ONLY on a 4xx (RFC 8628
              # §3.5 error responses are 400-class): a nonstandard 2xx
              # (201/202) or a 5xx carrying a crafted authorization_pending
              # body must not keep the loop polling — only a 200 can produce a
              # token and only a 4xx a protocol state. Everything else falls
              # back to http_<status>, which the loop terminates as api_error.
              error = data["error"]
              recognized = (400..499).cover?(status) && error.is_a?(String) && !error.empty?
              # Truncate at extraction (SPEC §9's 500-unit cap): the server
              # controls this string and an unrecognized value is interpolated
              # into the api_error message. Real protocol codes are short, so
              # classification is unaffected.
              [ :error, recognized ? Basecamp::Security.truncate(error) : "http_#{status}", status ]
            end
          end

          # Constructs the {Token}, type-checking every optional field first:
          # {Token#initialize} performs +Time+ arithmetic on +expires_in+, so a
          # malformed value (a String, a non-finite +Float::INFINITY+ from
          # +1e400+, a value past {MAX_TOKEN_LIFETIME_SECONDS}) must surface as
          # +api_error+ rather than escape as a TypeError or FloatDomainError.
          # +token_type+/+refresh_token+/+scope+ must be Strings when present.
          # Absent/nil +expires_in+ stays allowed (no expiry).
          def build_token(data, status)
            expires_in = data["expires_in"]
            unless valid_token_expires_in?(expires_in)
              raise OauthError.new(
                "api_error",
                "Invalid device token response: expires_in must be a finite positive whole number " \
                  "no greater than #{MAX_TOKEN_LIFETIME_SECONDS} seconds",
                http_status: status
              )
            end
            # Coerce an integer-valued Float (3600.0) to Integer so {Token} always
            # carries whole seconds, matching the other SDKs' coercion.
            expires_in = expires_in.to_i unless expires_in.nil?

            token_type = data["token_type"]
            unless token_type.nil? || (token_type.is_a?(String) && !token_type.empty?)
              raise OauthError.new(
                "api_error",
                "Invalid device token response: token_type must be a non-empty string",
                http_status: status
              )
            end

            refresh_token = data["refresh_token"]
            unless refresh_token.nil? || refresh_token.is_a?(String)
              raise OauthError.new(
                "api_error",
                "Invalid device token response: refresh_token must be a string",
                http_status: status
              )
            end

            scope = data["scope"]
            unless scope.nil? || scope.is_a?(String)
              raise OauthError.new(
                "api_error",
                "Invalid device token response: scope must be a string",
                http_status: status
              )
            end

            Token.new(
              access_token: data["access_token"],
              refresh_token: refresh_token,
              token_type: token_type || "Bearer",
              expires_in: expires_in,
              scope: scope
            )
          end

          # A token +expires_in+ is valid when absent/nil or a finite, positive,
          # WHOLE-second Numeric within {MAX_TOKEN_LIFETIME_SECONDS}. An
          # integer-valued float (+3600.0+) is accepted; a fractional value
          # (+1.5+) is rejected — matching the device-duration rule and Go/Kotlin,
          # whose integer/Long typing already rejects a fractional lifetime.
          # +Float::INFINITY+ (from a JSON +1e400+) is Numeric and positive but not
          # finite, so +finite?+ rejects it before it can poison deadline math.
          def valid_token_expires_in?(value)
            return true if value.nil?

            value.is_a?(Numeric) && value.finite? && value.positive? &&
              value <= MAX_TOKEN_LIFETIME_SECONDS && (value % 1).zero?
          end
      end
    end
  end
end
