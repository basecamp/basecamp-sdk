from __future__ import annotations

from basecamp.errors import BasecampError, ErrorCode

_OAUTH_TYPE_TO_CODE: dict[str, str] = {
    "validation": ErrorCode.VALIDATION,
    "auth": ErrorCode.AUTH,
    "network": ErrorCode.NETWORK,
    "api_error": ErrorCode.API,
}


class OAuthError(BasecampError):
    """OAuth-specific error with a type classifier.

    Types: "validation", "auth", "network", "api_error"
    """

    def __init__(self, oauth_type: str, message: str, **kwargs):
        code = _OAUTH_TYPE_TO_CODE.get(oauth_type, ErrorCode.API)
        super().__init__(message, code=code, **kwargs)
        self.oauth_type = oauth_type
