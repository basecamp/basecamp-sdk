from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from typing import Literal


@dataclass(frozen=True)
class OAuthConfig:
    """OAuth 2 server configuration from discovery endpoint (RFC 8414)."""

    issuer: str
    token_endpoint: str
    # Optional as of BC5 resource-first discovery: device-only authorization
    # servers omit it. Authorization-code consumers MUST assert its presence
    # before use. Absent (``None``) and present-empty are preserved distinctly.
    authorization_endpoint: str | None = None
    device_authorization_endpoint: str | None = None
    registration_endpoint: str | None = None
    scopes_supported: list[str] | None = None
    grant_types_supported: list[str] | None = None
    code_challenge_methods_supported: list[str] | None = None


@dataclass(frozen=True)
class ProtectedResourceMetadata:
    """RFC 9728 protected-resource metadata (hop 1 of resource-first discovery)."""

    resource: str
    # Authorization servers advertised for this resource. Absent (``None``) and
    # present-but-empty (``[]``) are preserved distinctly: BC5 omits the key
    # while dark (RFC 9728 §3.2). Both select Launchpad, but the distinction is
    # meaningful to callers inspecting the metadata directly.
    authorization_servers: list[str] | None = None


class FallbackReason(StrEnum):
    """The ONLY two soft outcomes under which resource-first discovery yields a
    Launchpad fallback rather than a selected config. Every other failure raises
    :class:`~basecamp.oauth.errors.DiscoverySelectionError`.
    """

    RESOURCE_DISCOVERY_FAILED = "resource_discovery_failed"
    NO_AS_ADVERTISED = "no_as_advertised"


@dataclass(frozen=True)
class DiscoveryResult:
    """Result of :func:`~basecamp.oauth.discovery.discover_from_resource`: either
    a selected AS config, or a soft fallback to Launchpad. Hard failures are
    raised, not represented here.

    ``kind`` discriminates the two shapes, and the four fields carry a strict
    per-``kind`` invariant that the static ``| None`` types cannot express (a
    typed caller must branch on ``kind`` first, then read the guaranteed fields):

      - ``kind == "selected"`` → ``config`` and ``issuer`` are non-``None``;
        ``reason`` is ``None``.
      - ``kind == "fallback"`` → ``reason`` is non-``None`` (a
        :class:`FallbackReason`); ``config`` and ``issuer`` are ``None``.

    Use :meth:`selected_config` to read ``config`` with the invariant enforced
    (it narrows to a non-optional :class:`OAuthConfig`, raising if misused).
    """

    kind: Literal["selected", "fallback"]
    config: OAuthConfig | None = None
    issuer: str | None = None
    reason: FallbackReason | None = None

    def selected_config(self) -> OAuthConfig:
        """Return the selected :class:`OAuthConfig`, narrowing away ``None`` for
        typed callers. Raises :class:`ValueError` if this is not a ``selected``
        result — enforcing the documented invariant instead of returning
        ``OAuthConfig | None`` that every caller would have to re-narrow.
        """
        if self.kind != "selected" or self.config is None:
            raise ValueError("DiscoveryResult.selected_config() called on a non-selected result")
        return self.config
