from __future__ import annotations

from basecamp.errors import BasecampError, ErrorCode


class WebhookVerificationError(BasecampError):
    def __init__(self, message: str = "invalid webhook signature"):
        super().__init__(message, code=ErrorCode.VALIDATION)
