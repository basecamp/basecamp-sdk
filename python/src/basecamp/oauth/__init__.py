from __future__ import annotations

from basecamp._security import require_origin_root
from basecamp.oauth.authorize import build_authorization_url
from basecamp.oauth.config import (
    DiscoveryResult,
    FallbackReason,
    OAuthConfig,
    ProtectedResourceMetadata,
)
from basecamp.oauth.device import (
    DEVICE_CODE_GRANT_TYPE,
    perform_device_login,
    poll_device_token,
    request_device_authorization,
)
from basecamp.oauth.device_authorization import DeviceAuthorization
from basecamp.oauth.discovery import (
    LAUNCHPAD_BASE_URL,
    discover,
    discover_from_resource,
    discover_launchpad,
    discover_protected_resource,
)
from basecamp.oauth.errors import DeviceFlowError, DiscoverySelectionError, OAuthError
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
    "DeviceFlowError",
    "DeviceAuthorization",
    "DEVICE_CODE_GRANT_TYPE",
    "request_device_authorization",
    "poll_device_token",
    "perform_device_login",
    "LAUNCHPAD_BASE_URL",
]
