/**
 * Builds SignedGlobalID payloads for the mention tests.
 *
 * The helpers under test decode what Rails' `SignedGlobalID` emits, so the
 * tests mint the real thing rather than pasting opaque base64: a Marshal 4.8
 * envelope in either of the two layouts, or the JSON spelling of one, signed
 * with a digest that is deliberately arbitrary — nothing client-side can verify
 * it, which is the whole point of the trust boundary the helpers document.
 */

// -----------------------------------------------------------------------------
// Marshal 4.8 writer (the subset an envelope uses)
// -----------------------------------------------------------------------------

/** Marshal's packed integer, for the small values an envelope carries. */
function packedInt(n: number): number[] {
  if (n === 0) return [0];
  if (n > 0 && n < 123) return [n + 5];
  if (n < 0 && n > -124) return [n - 5 + 256];
  throw new Error(`sgid helper: ${n} is outside the compact packed-integer range`);
}

/**
 * A Ruby String with its `:E` encoding ivar — the shape a SignedGlobalID
 * envelope's keys and values take. The symbol is written out in full each time
 * rather than linked, which a reader must accept either way.
 */
function marshalString(value: string): number[] {
  const bytes = [...new TextEncoder().encode(value)];
  return [
    0x49, // 'I' — object with instance variables
    0x22, // '"' — String
    ...packedInt(bytes.length),
    ...bytes,
    ...packedInt(1), // one ivar
    0x3a, // ':' — Symbol
    ...packedInt(1),
    0x45, // 'E'
    0x54, // true
  ];
}

const MARSHAL_NIL = [0x30];

function marshalHash(pairs: [string, number[]][]): number[] {
  const body = pairs.flatMap(([key, value]) => [...marshalString(key), ...value]);
  return [0x04, 0x08, 0x7b, ...packedInt(pairs.length), ...body];
}

// -----------------------------------------------------------------------------
// Payload encoding
// -----------------------------------------------------------------------------

function base64Url(bytes: number[] | Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** Appends a digest, the way Rails' MessageVerifier does. */
function sign(payload: string, digest = "0123456789abcdef0123456789abcdef01234567"): string {
  return `${payload}--${digest}`;
}

export interface SGIDOptions {
  /** Defaults to `"attachable"`, the only purpose BC3 honours in rich text. */
  purpose?: string;
  /** Leave the digest off, as an unsigned envelope would. */
  unsigned?: boolean;
  /** Override the digest — e.g. one that itself contains `--`. */
  digest?: string;
}

/** The older Marshal layout: `{gid:, purpose:, expires_at:}`. */
export function legacySGID(gid: string, options: SGIDOptions = {}): string {
  const payload = base64Url(
    marshalHash([
      ["gid", marshalString(gid)],
      ["purpose", marshalString(options.purpose ?? "attachable")],
      ["expires_at", MARSHAL_NIL],
    ]),
  );
  return options.unsigned ? payload : sign(payload, options.digest);
}

/** The current Marshal layout: `{_rails: {data:, exp:, pur:}}`. */
export function railsSGID(gid: string, options: SGIDOptions = {}): string {
  const inner = marshalHash([
    ["data", marshalString(gid)],
    ["exp", MARSHAL_NIL],
    ["pur", marshalString(options.purpose ?? "attachable")],
  ]).slice(2); // the inner hash carries no 0x04 0x08 version prefix
  const payload = base64Url(marshalHash([["_rails", inner]]));
  return options.unsigned ? payload : sign(payload, options.digest);
}

/** The JSON spelling Rails' JSON message serializer emits. */
export function jsonSGID(gid: string, options: SGIDOptions = {}): string {
  const json = JSON.stringify({
    _rails: { data: gid, exp: null, pur: options.purpose ?? "attachable" },
  });
  const payload = base64Url(new TextEncoder().encode(json));
  return options.unsigned ? payload : sign(payload, options.digest);
}

/** The attachable sgid for a person, in the layout BC3 currently mints. */
export function personSGID(personId: number, options: SGIDOptions = {}): string {
  return legacySGID(`gid://bc3/Person/${personId}?expires_in=`, options);
}
