from __future__ import annotations

from basecamp._security import require_origin_root
from basecamp.oauth.authorize import build_authorization_url
from basecamp.oauth.config import (
    DiscoveryResult,
    FallbackReason,
    OAuthConfig,
    ProtectedResourceMetadata,
)
from basecamp.oauth.discovery import (
    LAUNCHPAD_BASE_URL,
    discover,
    discover_from_resource,
    discover_launchpad,
    discover_protected_resource,
)
from basecamp.oauth.errors import DiscoverySelectionError, OAuthError
from basecamp.oauth.exchange import exchange_code, refresh_token
from basecamp.oauth.pkce import PKCE, generate_pkce, generate_state
from basecamp.oauth.token import OAuthToken

__all__ = [
    "OAuthConfig",
    "ProtectedResourceMetadata",
    "DiscoveryResult",
    "FallbackReason",
    "OAuthToken",
    "PKCE",
    "discover",
    "discover_launchpad",
    "discover_protected_resource",
    "discover_from_resource",
    "require_origin_root",
    "generate_pkce",
    "generate_state",
    "build_authorization_url",
    "exchange_code",
    "refresh_token",
    "OAuthError",
    "DiscoverySelectionError",
    "LAUNCHPAD_BASE_URL",
]
