from __future__ import annotations

from dataclasses import replace

from basecamp.download import DownloadResult
from basecamp.errors import UsageError
from basecamp.generated.services.uploads import AsyncUploadsService as _GeneratedAsyncUploadsService
from basecamp.generated.services.uploads import UploadsService as _GeneratedUploadsService


class UploadsService(_GeneratedUploadsService):
    """Sync uploads service with hand-written download() convenience."""

    def download(self, *, upload_id: int) -> DownloadResult:
        """Download an upload's file content in one call.

        Fetches the upload metadata, then delegates to
        :meth:`Client.download_url` so the authenticated-hop + 302-follow flow
        lives in one place.

        :param upload_id: The upload's numeric id.
        :return: A :class:`DownloadResult` whose ``filename`` prefers
            ``upload["filename"]`` from metadata, falling back to URL-derived.
        :raises UsageError: If the upload has no ``download_url``.
        """
        upload = self.get(upload_id=upload_id)
        url = upload.get("download_url")
        if not url:
            raise UsageError(f"upload {upload_id} has no download_url")
        result = self._client.download_url(url)
        filename = upload.get("filename")
        return replace(result, filename=filename) if filename else result


class AsyncUploadsService(_GeneratedAsyncUploadsService):
    """Async uploads service with hand-written download() convenience."""

    async def download(self, *, upload_id: int) -> DownloadResult:
        upload = await self.get(upload_id=upload_id)
        url = upload.get("download_url")
        if not url:
            raise UsageError(f"upload {upload_id} has no download_url")
        result = await self._client.download_url(url)
        filename = upload.get("filename")
        return replace(result, filename=filename) if filename else result
