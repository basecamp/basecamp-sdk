from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class DeviceAuthorization:
    """RFC 8628 §3.2 device authorization response.

    Returned by :func:`~basecamp.oauth.device.request_device_authorization`. The
    ``device_code`` is polled at the token endpoint; ``user_code`` and
    ``verification_uri`` (optionally ``verification_uri_complete``) are shown to
    the user. ``interval`` defaults to 5 seconds when the server omits it.
    """

    device_code: str
    user_code: str
    verification_uri: str
    expires_in: int
    interval: int
    verification_uri_complete: str | None = None
