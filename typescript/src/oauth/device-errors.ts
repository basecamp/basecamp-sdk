/**
 * Terminal errors for the RFC 8628 device authorization grant.
 *
 * A single {@link DeviceFlowError} carries a {@link DeviceFlowReason}; the parent
 * BasecampError category is DERIVED from the reason (SPEC.md §16), so callers can
 * branch on either `.reason` (precise) or `.code`/`.exitCode` (coarse).
 */

import { BasecampError, type ErrorCode, type BasecampErrorOptions } from "../errors.js";

/**
 * Why a device flow terminated.
 *
 * - `access_denied` — the user declined the authorization → auth
 * - `expired` — the device/user code expired before approval → auth
 * - `transport` — a network failure ended the flow → network (retryable)
 * - `unavailable` — the selected AS cannot do device flow → validation
 * - `cancelled` — the caller cancelled (aborted) the flow → usage
 */
export type DeviceFlowReason =
  | "access_denied"
  | "expired"
  | "transport"
  | "unavailable"
  | "cancelled";

/** Maps a device-flow reason to its parent BasecampError category. */
function categoryFor(reason: DeviceFlowReason): ErrorCode {
  switch (reason) {
    case "access_denied":
    case "expired":
      return "auth_required";
    case "transport":
      return "network";
    case "unavailable":
      return "validation";
    case "cancelled":
      return "usage";
  }
}

/**
 * Terminal device-flow error. Its `code` (and thus `exitCode`) is derived from
 * `reason`; `transport` is retryable.
 */
export class DeviceFlowError extends BasecampError {
  readonly reason: DeviceFlowReason;

  constructor(reason: DeviceFlowReason, message: string, options?: BasecampErrorOptions) {
    // The reason derives retryability authoritatively: spread caller options
    // FIRST so a caller-supplied `retryable` cannot flip the reason's verdict
    // (only `transport` is retryable).
    super(categoryFor(reason), message, {
      ...options,
      retryable: reason === "transport",
    });
    this.name = "DeviceFlowError";
    this.reason = reason;
  }

  /** Serializes like BasecampError, plus the precise device-flow reason. */
  toJSON(): Record<string, unknown> {
    return { ...super.toJSON(), reason: this.reason };
  }
}
