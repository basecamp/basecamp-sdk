/**
 * Resource-first OAuth discovery tests.
 *
 * Drives the shared, data-only fixtures in conformance/oauth/fixtures with this
 * harness's mock origins substituted for the {{...}} placeholders, so issuer /
 * resource binding stays code-point-exact against the mocked hosts.
 */

import { describe, it, expect, beforeEach } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { http as mswHttp, HttpResponse } from "msw";
import { server } from "../setup.js";
import {
  discover,
  discoverProtectedResource,
  discoverFromResource,
  discoverLaunchpad,
  requireOriginRoot,
  DiscoverySelectionError,
} from "../../src/oauth/index.js";
import { BasecampError } from "../../src/errors.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const FIXTURE_DIR = join(HERE, "../../../conformance/oauth/fixtures");

// Mock origins substituted for fixture placeholders. LAUNCHPAD must be the real
// origin because discoverLaunchpad() targets it.
const ORIGINS = {
  "{{RESOURCE_ORIGIN}}": "https://api.basecamp-test.example",
  "{{ISSUER_ORIGIN}}": "https://issuer.basecamp-test.example",
  "{{LAUNCHPAD_ORIGIN}}": "https://launchpad.37signals.com",
  "{{BC5_ISSUER}}": "https://bc5.basecamp-test.example",
};

function substitute<T>(value: T): T {
  let json = JSON.stringify(value);
  for (const [ph, origin] of Object.entries(ORIGINS)) {
    json = json.split(ph).join(origin);
  }
  return JSON.parse(json) as T;
}

interface Exchange {
  origin?: string;
  status?: number;
  transportError?: boolean;
  body?: unknown;
  oversized?: boolean;
  redirectTo?: string;
}

interface Fixture {
  name: string;
  operation: "discoverFromResource" | "discoverProtectedResource" | "discover";
  resourceOrigin?: string;
  issuerOrigin?: string;
  expectedIssuer?: string;
  hop1?: Exchange;
  hop2?: Exchange;
  expect: {
    outcome: "selected" | "fallback" | "raise";
    selectedIssuer?: string;
    fallbackReason?: string;
    error?: string;
    errorCategory?: string;
    launchpadContacted?: boolean;
  };
}

function loadFixtures(): Fixture[] {
  return readdirSync(FIXTURE_DIR)
    .filter((f) => f.endsWith(".json"))
    .sort()
    .map((f) => JSON.parse(readFileSync(join(FIXTURE_DIR, f), "utf8")) as Fixture);
}

const WELL_KNOWN = {
  resource: "/.well-known/oauth-protected-resource",
  as: "/.well-known/oauth-authorization-server",
};

/** Registers an MSW handler for one mocked hop. */
function handlerFor(url: string, ex: Exchange) {
  return mswHttp.get(url, () => {
    if (ex.transportError) return HttpResponse.error();
    if (ex.redirectTo) {
      return new HttpResponse(null, { status: ex.status ?? 302, headers: { Location: ex.redirectTo } });
    }
    const status = ex.status ?? 200;
    if (ex.body === undefined) return new HttpResponse(null, { status });
    return HttpResponse.json(ex.body as object, { status });
  });
}

describe("resource-first discovery fixtures", () => {
  let launchpadContacted = false;

  beforeEach(() => {
    launchpadContacted = false;
    // Track ANY request to a Launchpad well-known endpoint — both the AS metadata
    // and the protected-resource metadata — so a hard case that wrongly hit either
    // Launchpad endpoint is caught. The orchestrator itself never contacts Launchpad.
    server.use(
      mswHttp.get(`${ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}${WELL_KNOWN.as}`, () => {
        launchpadContacted = true;
        return HttpResponse.json({
          issuer: ORIGINS["{{LAUNCHPAD_ORIGIN}}"],
          authorization_endpoint: `${ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}/authorization/new`,
          token_endpoint: `${ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}/authorization/token`,
        });
      }),
      mswHttp.get(`${ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}${WELL_KNOWN.resource}`, () => {
        launchpadContacted = true;
        return HttpResponse.json({ resource: ORIGINS["{{LAUNCHPAD_ORIGIN}}"] });
      })
    );
  });

  for (const raw of loadFixtures()) {
    // Oversized streaming is exercised by a dedicated test below (MSW cannot
    // easily stream a body larger than the cap through json()).
    if (raw.hop2?.oversized || raw.hop1?.oversized) continue;

    it(`${raw.name}`, async () => {
      const fx = substitute(raw);

      // Bracketed IPv6 origins can't be pattern-matched by MSW's path parser, so
      // the IPv6 origin-root accept case is verified at the parser boundary (the
      // point of the fixture: the transport parser accepts it where a regex fails).
      if (fx.resourceOrigin?.includes("[") && fx.expect.outcome === "selected") {
        expect(requireOriginRoot(fx.resourceOrigin)).toBe(fx.expect.selectedIssuer);
        return;
      }

      if (fx.hop1) {
        // Register the mock at the NORMALIZED origin: the SDK builds the well-known
        // URL from the normalized origin even when the caller's spelling differs
        // (e.g. a trailing slash or explicit :443), so the raw string wouldn't match.
        const resourceOrigin = requireOriginRoot(fx.resourceOrigin!);
        server.use(handlerFor(`${resourceOrigin}${WELL_KNOWN.resource}`, fx.hop1));
      }
      if (fx.hop2) {
        const issuerOrigin = fx.hop2.origin ?? fx.issuerOrigin!;
        server.use(handlerFor(`${issuerOrigin}${WELL_KNOWN.as}`, fx.hop2));
      }

      const run = async () => {
        switch (fx.operation) {
          case "discoverFromResource":
            return discoverFromResource(fx.resourceOrigin!, { expectedIssuer: fx.expectedIssuer });
          case "discoverProtectedResource":
            return discoverProtectedResource(fx.resourceOrigin!);
          case "discover":
            return discover(fx.issuerOrigin!);
        }
      };

      if (fx.expect.outcome === "raise") {
        let thrown: unknown;
        try {
          await run();
          expect.fail("expected a throw");
        } catch (err) {
          thrown = err;
        }
        expect(thrown).toBeInstanceOf(BasecampError);
        if (fx.expect.error === "usage") {
          expect((thrown as BasecampError).code).toBe("usage");
        } else if (fx.operation === "discoverFromResource") {
          expect(thrown).toBeInstanceOf(DiscoverySelectionError);
          expect((thrown as DiscoverySelectionError).reason).toBe(fx.expect.error);
        } else {
          // discover / discoverProtectedResource hard failures are api_error.
          expect((thrown as BasecampError).code).toBe("api_error");
        }
        // Cross-SDK coarse-category assertion.
        if (fx.expect.errorCategory) {
          expect((thrown as BasecampError).code).toBe(fx.expect.errorCategory);
        }
      } else if (fx.expect.outcome === "fallback") {
        const result = (await run()) as { kind: string; reason?: string };
        expect(result.kind).toBe("fallback");
        expect(result.reason).toBe(fx.expect.fallbackReason);
        // The orchestrator returns a soft fallback WITHOUT contacting Launchpad;
        // the consumer performs the Launchpad login. Do that here so the
        // launchpadContacted assertion below reflects the full flow (a
        // regression that skipped the Launchpad request would then be caught).
        if (fx.operation === "discoverFromResource") {
          await discoverLaunchpad();
        }
      } else {
        // selected
        const result = await run();
        if (fx.operation === "discoverFromResource") {
          const r = result as { kind: string; issuer: string };
          expect(r.kind).toBe("selected");
          if (fx.expect.selectedIssuer) expect(r.issuer).toBe(fx.expect.selectedIssuer);
        }
        // discover / discoverProtectedResource: absence of a throw is success.
      }

      // Assert the exact launchpadContacted expectation when a fixture states one,
      // so a soft-fallback fixture that expects Launchpad to be contacted keeps its
      // regression signal too — not only the `false` (never-contacted) case.
      if (fx.expect.launchpadContacted !== undefined) {
        expect(launchpadContacted).toBe(fx.expect.launchpadContacted);
      }
    });
  }

  it("device-only AS omits authorization_endpoint but carries device capability", async () => {
    const issuer = ORIGINS["{{ISSUER_ORIGIN}}"];
    server.use(
      mswHttp.get(`${issuer}${WELL_KNOWN.as}`, () =>
        HttpResponse.json({
          issuer,
          token_endpoint: `${issuer}/oauth/token`,
          device_authorization_endpoint: `${issuer}/oauth/device`,
          grant_types_supported: ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"],
        })
      )
    );
    const config = await discover(issuer);
    expect(config.authorizationEndpoint).toBeUndefined();
    expect(config.deviceAuthorizationEndpoint).toBe(`${issuer}/oauth/device`);
    expect(config.grantTypesSupported).toContain("urn:ietf:params:oauth:grant-type:device_code");
  });
});

describe("issuer-binding classification (structural marker, not message text)", () => {
  const RESOURCE = "https://api.marker-test.example";
  const BC5 = "https://bc5.marker-test.example";

  function mountResource(servers: string[]) {
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: RESOURCE, authorization_servers: servers })
      )
    );
  }

  it("classifies a committed-issuer binding mismatch as issuer_mismatch", async () => {
    mountResource([BC5]);
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: "https://impostor.example", token_endpoint: `${BC5}/t` })
      )
    );
    const err = await discoverFromResource(RESOURCE).catch((e) => e);
    expect(err).toBeInstanceOf(DiscoverySelectionError);
    expect((err as DiscoverySelectionError).reason).toBe("issuer_mismatch");
  });

  it("classifies a non-mismatch committed-AS fault as as_fetch_failed", async () => {
    // Missing token_endpoint yields an api_error whose message says nothing
    // about a mismatch. Message-based classification would misroute it; the
    // structural marker keeps it as as_fetch_failed.
    mountResource([BC5]);
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: BC5 })
      )
    );
    const err = await discoverFromResource(RESOURCE).catch((e) => e);
    expect(err).toBeInstanceOf(DiscoverySelectionError);
    expect((err as DiscoverySelectionError).reason).toBe("as_fetch_failed");
  });

  it("does not leak the marker through the public discover() surface", async () => {
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: "https://impostor.example", token_endpoint: `${BC5}/t` })
      )
    );
    const err = await discover(BC5).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err).not.toBeInstanceOf(DiscoverySelectionError);
    expect((err as BasecampError).code).toBe("api_error");
  });
});

describe("SSRF hardening", () => {
  it("rejects an over-cap discovery body via the bounded read (not a post-hoc check)", async () => {
    const issuer = "https://issuer.ssrf-test.example";
    // A well-formed but oversized document: valid JSON padded far past the cap.
    const oversized = `{"issuer":"${issuer}","token_endpoint":"${issuer}/t","pad":"${"x".repeat(256 * 1024)}"}`;

    server.use(
      mswHttp.get(`${issuer}${WELL_KNOWN.as}`, () =>
        new HttpResponse(oversized, { status: 200, headers: { "Content-Type": "application/json" } })
      )
    );

    // Cap at 8 KiB: the streaming reader cancels once the accumulated bytes
    // exceed the cap, so the full 256 KiB body is never buffered.
    await expect(discover(issuer, { maxBodyBytes: 8 * 1024 })).rejects.toMatchObject({
      code: "api_error",
    });
  });

  it("ignores Infinity/NaN/negative maxBodyBytes and applies the default cap", async () => {
    const issuer = "https://issuer.badcap-test.example";
    // Padded well past the 1 MiB default cap. A caller-supplied Infinity/NaN/
    // negative must NOT disable the bound — it falls back to the default so the
    // oversized body is still rejected.
    const oversized =
      `{"issuer":"${issuer}","token_endpoint":"${issuer}/t","pad":"${"x".repeat(2 * 1024 * 1024)}"}`;

    server.use(
      mswHttp.get(`${issuer}${WELL_KNOWN.as}`, () =>
        new HttpResponse(oversized, { status: 200, headers: { "Content-Type": "application/json" } })
      )
    );

    for (const bad of [Infinity, Number.NaN, -1]) {
      await expect(
        discover(issuer, { maxBodyBytes: bad })
      ).rejects.toMatchObject({ code: "api_error" });
    }
  });

  it("does not follow a redirect on a discovery fetch", async () => {
    const issuer = "https://issuer.redirect-test.example";
    let attackerContacted = false;
    server.use(
      mswHttp.get(`${issuer}${WELL_KNOWN.as}`, () =>
        new HttpResponse(null, {
          status: 302,
          headers: { Location: "https://attacker.example.com/.well-known/oauth-authorization-server" },
        })
      ),
      mswHttp.get("https://attacker.example.com/.well-known/oauth-authorization-server", () => {
        attackerContacted = true;
        return HttpResponse.json({ issuer: "https://attacker.example.com", token_endpoint: "x" });
      })
    );

    await expect(discover(issuer)).rejects.toMatchObject({ code: "api_error" });
    expect(attackerContacted).toBe(false);
  });
});

describe("requireOriginRoot userinfo rejection", () => {
  // Rejection keys off the PRESENCE of userinfo, not its truthiness: the
  // WHATWG URL parser normalizes delimiter-only userinfo ("https://@host") to
  // empty username/password and drops it from href, so a field-only check
  // would silently accept — and normalize away — a malformed caller origin.
  it.each(["https://user@host", "https://@example.com", "https://:@host"])(
    "rejects %s",
    (raw) => {
      expect(() => requireOriginRoot(raw)).toThrow(/userinfo/);
    }
  );

  // A bare trailing "?"/"#" is normalized to empty search/hash by WHATWG URL, so
  // the parsed fields miss it — the raw scan must still reject it, or a malformed
  // caller origin (or a Launchpad look-alike) slips through as a clean origin.
  it.each(["https://launchpad.37signals.com?", "https://launchpad.37signals.com#"])(
    "rejects bare query/fragment delimiter %s",
    (raw) => {
      expect(() => requireOriginRoot(raw)).toThrow(/query or fragment/);
    }
  );

  // WHATWG accepts port 0 and keeps it in the origin; the origin-root profile
  // rejects any port outside 1–65535, matching the other SDKs.
  it("rejects port 0", () => {
    expect(() => requireOriginRoot("https://host:0")).toThrow(/invalid port/);
  });

  // WHATWG normalizes a dangling ":" away (url.port === ""); the raw-authority
  // scan must still reject it.
  it("rejects a dangling port delimiter", () => {
    expect(() => requireOriginRoot("https://host:")).toThrow(/invalid port/);
  });

  // WHATWG strips C0 controls / surrounding whitespace and converts backslashes;
  // the up-front raw scan must reject these spellings before they are cleaned.
  it.each(["https:\\\\host", "https://host\n", "https://host ", "https://ho st"])(
    "rejects a WHATWG-normalized spelling %j",
    (raw) => {
      expect(() => requireOriginRoot(raw)).toThrow(/invalid characters/);
    }
  );

  // WHATWG recovers a missing/extra-slash authority into a clean origin; require
  // an explicit "://" + non-empty authority.
  it.each(["https:host", "https:///host"])(
    "rejects a missing-authority spelling %j",
    (raw) => {
      expect(() => requireOriginRoot(raw)).toThrow(/origin root/);
    }
  );

  // WHATWG resolves dot-segments to "/", so the raw path must be scanned.
  it.each(["https://api.example/a/..", "https://api.example/%2e%2e"])(
    "rejects a dot-segment path %j",
    (raw) => {
      expect(() => requireOriginRoot(raw)).toThrow(/origin root \(no path\)/);
    }
  );
});

describe("resource metadata strictness (#369 review)", () => {
  const RESOURCE = "https://api.strict-test.example";
  const BC5 = "https://bc5.strict-test.example";

  it("rejects a present null authorization_servers as resource_discovery_failed", async () => {
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: RESOURCE, authorization_servers: null })
      )
    );
    const result = await discoverFromResource(RESOURCE);
    expect(result).toMatchObject({ kind: "fallback", reason: "resource_discovery_failed" });
  });

  it("treats duplicate advertisements of one issuer as a single issuer, not ambiguous", async () => {
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: RESOURCE, authorization_servers: [BC5, BC5] })
      ),
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: BC5, token_endpoint: `${BC5}/token` })
      )
    );
    const result = (await discoverFromResource(RESOURCE)) as { kind: string; issuer?: string };
    expect(result.kind).toBe("selected");
    expect(result.issuer).toBe(BC5);
  });

  it("binds AS metadata against the advertised issuer string, not the normalized origin", async () => {
    // The advertised issuer carries a trailing slash, which normalizes away for
    // routing. Binding must still be code-point-exact against the ADVERTISED
    // string: AS metadata echoing "https://bc5…/" must bind, not mismatch.
    const advertised = `${BC5}/`;
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: RESOURCE, authorization_servers: [advertised] })
      ),
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: advertised, token_endpoint: `${BC5}/token` })
      )
    );
    const result = (await discoverFromResource(RESOURCE)) as { kind: string; issuer?: string };
    expect(result.kind).toBe("selected");
    expect(result.issuer).toBe(advertised);
  });

  it("binds the resource identifier against the raw caller string (default port)", async () => {
    // ":443" normalizes away for the fetch URL, but the metadata resource is bound
    // code-point-exact against the ORIGINAL caller identifier (RFC 9728 §3.3).
    server.use(
      mswHttp.get(`${RESOURCE}/.well-known/oauth-protected-resource`, () =>
        HttpResponse.json({ resource: `${RESOURCE}:443` })
      )
    );
    const meta = await discoverProtectedResource(`${RESOURCE}:443`);
    expect(meta.resource).toBe(`${RESOURCE}:443`);
  });

  it("rejects scopes_supported that is not an array of strings", async () => {
    server.use(
      mswHttp.get(`${BC5}/.well-known/oauth-authorization-server`, () =>
        HttpResponse.json({ issuer: BC5, token_endpoint: `${BC5}/token`, scopes_supported: "read write" })
      )
    );
    await expect(discover(BC5)).rejects.toThrow(/scopes_supported must be an array of strings/);
  });
});
