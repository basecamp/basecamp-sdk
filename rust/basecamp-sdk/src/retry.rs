//! SPEC §7: the three gates, the attempt budget and the backoff curve.

use std::time::Duration;

use crate::route::{Backoff, RetryConfig};

/// The ceiling on a locally computed backoff term, in milliseconds.
pub const MAX_BACKOFF_DELAY_MS: u64 = 30_000;

/// The retry policy of SPEC §7's `DEFAULT_RETRY_CONFIG`.
pub const DEFAULT_RETRY_CONFIG: RetryConfig = RetryConfig {
    max_attempts: 3,
    base_delay_ms: 1000,
    backoff: Backoff::Exponential,
    retry_on: &[429, 503],
};

/// The policy that sends exactly once.
pub const NO_RETRY_CONFIG: RetryConfig = RetryConfig {
    max_attempts: 1,
    base_delay_ms: 0,
    backoff: Backoff::Constant,
    retry_on: &[],
};

/// The download flow's own retry set (SPEC §14): network errors plus these, never 500.
pub const DOWNLOAD_RETRY_ON: &[u16] = &[429, 502, 503, 504];

/// `min(max(1, cap), op_max)`: the client cap floored at one attempt, then bounded by the
/// operation's own ceiling.
pub fn effective_attempts(client_cap: u32, operation_max: u32) -> u32 {
    client_cap.max(1).min(operation_max.max(1))
}

/// The backoff term for the `retry_index`-th retry (0 for the first), saturating at
/// [`MAX_BACKOFF_DELAY_MS`]. The exponential curve's crossing is found in the log domain —
/// `retry_index >= log2(ceiling / base)` lands on the ceiling — so no intermediate ever
/// overflows and the term tracks `base × 2^i` exactly below it.
pub fn backoff_ms(config: &RetryConfig, retry_index: u32) -> u64 {
    let base = config.base_delay_ms;
    if base == 0 {
        return 0;
    }
    match config.backoff {
        Backoff::Constant => base.min(MAX_BACKOFF_DELAY_MS),
        Backoff::Linear => base
            .saturating_mul(u64::from(retry_index).saturating_add(1))
            .min(MAX_BACKOFF_DELAY_MS),
        Backoff::Exponential => {
            if base >= MAX_BACKOFF_DELAY_MS {
                return MAX_BACKOFF_DELAY_MS;
            }
            // Doublings from `base` to the ceiling, rounded up: the first index at or past
            // it is on the ceiling. Bounded by log2(30_000) < 15, so the shift is always
            // in range.
            let crossing = (MAX_BACKOFF_DELAY_MS / base).ilog2()
                + u32::from(!(MAX_BACKOFF_DELAY_MS / base).is_power_of_two());
            if retry_index >= crossing {
                MAX_BACKOFF_DELAY_MS
            } else {
                (base << retry_index).min(MAX_BACKOFF_DELAY_MS)
            }
        }
    }
}

/// The backoff term plus `random(0, max_jitter)`.
pub fn backoff_with_jitter(
    config: &RetryConfig,
    retry_index: u32,
    max_jitter: Duration,
) -> Duration {
    let term = Duration::from_millis(backoff_ms(config, retry_index));
    let jitter = match u64::try_from(max_jitter.as_millis()) {
        Ok(0) | Err(_) => Duration::ZERO,
        Ok(millis) => Duration::from_millis(rand::random_range(0..=millis)),
    };
    term + jitter
}

#[cfg(test)]
mod tests {
    use super::*;

    fn exponential(base: u64) -> RetryConfig {
        RetryConfig {
            max_attempts: 3,
            base_delay_ms: base,
            backoff: Backoff::Exponential,
            retry_on: &[429, 503],
        }
    }

    #[test]
    fn attempts_are_the_floored_cap_under_the_operation_ceiling() {
        assert_eq!(effective_attempts(3, 3), 3);
        assert_eq!(effective_attempts(3, 2), 2);
        assert_eq!(effective_attempts(10, 3), 3);
        assert_eq!(effective_attempts(1, 3), 1);
        assert_eq!(effective_attempts(0, 3), 1);
        assert_eq!(effective_attempts(0, 0), 1);
    }

    #[test]
    fn exponential_backoff_tracks_the_curve_then_sits_on_the_ceiling() {
        let config = exponential(1000);
        assert_eq!(backoff_ms(&config, 0), 1000);
        assert_eq!(backoff_ms(&config, 1), 2000);
        assert_eq!(backoff_ms(&config, 4), 16_000);
        assert_eq!(backoff_ms(&config, 5), 30_000);
        assert_eq!(backoff_ms(&config, 63), 30_000);
        assert_eq!(backoff_ms(&config, u32::MAX), 30_000);
    }

    #[test]
    fn a_tiny_base_still_saturates_rather_than_plateauing() {
        let config = exponential(1);
        assert_eq!(backoff_ms(&config, 14), 16_384);
        assert_eq!(backoff_ms(&config, 15), 30_000);
        assert_eq!(backoff_ms(&config, 200), 30_000);
        assert_eq!(backoff_ms(&exponential(30_000), 0), 30_000);
        assert_eq!(backoff_ms(&exponential(40_000), 0), 30_000);
        assert_eq!(backoff_ms(&exponential(0), 3), 0);
    }

    #[test]
    fn linear_and_constant_are_capped_too() {
        let linear = RetryConfig {
            backoff: Backoff::Linear,
            ..exponential(10_000)
        };
        assert_eq!(backoff_ms(&linear, 0), 10_000);
        assert_eq!(backoff_ms(&linear, 2), 30_000);
        assert_eq!(backoff_ms(&linear, u32::MAX), 30_000);
        let constant = RetryConfig {
            backoff: Backoff::Constant,
            ..exponential(45_000)
        };
        assert_eq!(backoff_ms(&constant, 9), 30_000);
    }

    #[test]
    fn jitter_is_bounded() {
        for _ in 0..50 {
            let delay = backoff_with_jitter(&exponential(1000), 0, Duration::from_millis(100));
            assert!(delay >= Duration::from_millis(1000));
            assert!(delay <= Duration::from_millis(1100));
        }
        assert_eq!(
            backoff_with_jitter(&exponential(1000), 0, Duration::ZERO),
            Duration::from_millis(1000)
        );
    }
}
