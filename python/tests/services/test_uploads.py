"""Tests for the uploads.download(upload_id) convenience (sync + async)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import UsageError


def _metadata(upload_id: int = 1069479400, *, download_url, filename="report.pdf") -> dict:
    """Minimal upload metadata payload; callers override download_url/filename."""
    return {
        "id": upload_id,
        "filename": filename,
        "download_url": download_url,
    }


class TestSyncDownload:
    @respx.mock
    def test_delegates_through_download_url(self):
        metadata_route = respx.get("https://3.basecampapi.com/12345/uploads/1069479400").mock(
            return_value=httpx.Response(
                200,
                json=_metadata(
                    download_url="https://storage.example/12345/blobs/abc/download/report.pdf",
                    filename="report.pdf",
                ),
            )
        )
        # Hop 1: auth'd, origin-rewritten to base_url. Responds 302.
        hop1_route = respx.get("https://3.basecampapi.com/12345/blobs/abc/download/report.pdf").mock(
            return_value=httpx.Response(
                302,
                headers={"Location": "https://signed.example/bucket/xyz?sig=abc"},
            )
        )
        # Hop 2: signed URL, no auth.
        hop2_route = respx.get("https://signed.example/bucket/xyz?sig=abc").mock(
            return_value=httpx.Response(
                200,
                content=b"pdf-bytes",
                headers={"content-type": "application/pdf", "content-length": "9"},
            )
        )

        c = Client(access_token="test-token")
        account = c.for_account("12345")
        result = account.uploads.download(upload_id=1069479400)

        assert metadata_route.called
        assert hop1_route.called
        assert hop2_route.called
        assert result.body == b"pdf-bytes"
        assert result.content_type == "application/pdf"
        # Filename from metadata wins over URL-derived
        assert result.filename == "report.pdf"
        # First-hop (metadata) must be authenticated
        assert metadata_route.calls[0].request.headers.get("authorization") == "Bearer test-token"
        # Auth'd download hop also carries the bearer
        assert hop1_route.calls[0].request.headers.get("authorization") == "Bearer test-token"
        # Signed S3 hop must not carry auth
        assert hop2_route.calls[0].request.headers.get("authorization") is None

    def test_raises_when_metadata_missing_download_url(self):
        with respx.mock() as router:
            metadata_route = router.get("https://3.basecampapi.com/12345/uploads/1069479400").mock(
                return_value=httpx.Response(200, json=_metadata(download_url=None, filename="report.pdf"))
            )

            c = Client(access_token="test-token")
            account = c.for_account("12345")
            with pytest.raises(UsageError) as exc_info:
                account.uploads.download(upload_id=1069479400)

            assert "1069479400" in str(exc_info.value)
            assert "download_url" in str(exc_info.value)
            # Only the metadata request fires — no download hop should be attempted.
            assert metadata_route.call_count == 1
            assert len(router.calls) == 1


class TestAsyncDownload:
    @pytest.mark.asyncio
    @respx.mock
    async def test_delegates_through_download_url(self):
        metadata_route = respx.get("https://3.basecampapi.com/12345/uploads/1069479400").mock(
            return_value=httpx.Response(
                200,
                json=_metadata(
                    download_url="https://storage.example/12345/blobs/abc/download/report.pdf",
                    filename="report.pdf",
                ),
            )
        )
        hop1_route = respx.get("https://3.basecampapi.com/12345/blobs/abc/download/report.pdf").mock(
            return_value=httpx.Response(
                302,
                headers={"Location": "https://signed.example/bucket/xyz?sig=abc"},
            )
        )
        hop2_route = respx.get("https://signed.example/bucket/xyz?sig=abc").mock(
            return_value=httpx.Response(
                200,
                content=b"pdf-bytes",
                headers={"content-type": "application/pdf", "content-length": "9"},
            )
        )

        c = AsyncClient(access_token="test-token")
        account = c.for_account("12345")
        result = await account.uploads.download(upload_id=1069479400)

        assert metadata_route.called
        assert hop1_route.called
        assert hop2_route.called
        assert result.body == b"pdf-bytes"
        assert result.content_type == "application/pdf"
        assert result.filename == "report.pdf"
        assert metadata_route.calls[0].request.headers.get("authorization") == "Bearer test-token"
        assert hop1_route.calls[0].request.headers.get("authorization") == "Bearer test-token"
        assert hop2_route.calls[0].request.headers.get("authorization") is None

    @pytest.mark.asyncio
    async def test_raises_when_metadata_missing_download_url(self):
        with respx.mock() as router:
            metadata_route = router.get("https://3.basecampapi.com/12345/uploads/1069479400").mock(
                return_value=httpx.Response(200, json=_metadata(download_url=None, filename="report.pdf"))
            )

            c = AsyncClient(access_token="test-token")
            account = c.for_account("12345")
            with pytest.raises(UsageError) as exc_info:
                await account.uploads.download(upload_id=1069479400)

            assert "1069479400" in str(exc_info.value)
            assert "download_url" in str(exc_info.value)
            assert metadata_route.call_count == 1
            assert len(router.calls) == 1


class TestDescriptionAttachments:
    @respx.mock
    def test_get_preserves_dimension_float_and_none(self):
        """An Upload's rich-text description is paired with a
        description_attachments array. Pixel dimensions arrive float-spelled
        (1024.0) for images and null for non-image blobs. The service returns the
        parsed response dict, so httpx/json preserves both the float and None
        verbatim — Python performs no int coercion. The generated TypedDict types
        these honestly as ``NotRequired[Optional[int | float]]`` (the schema is
        nullable and the FlexInt dimension may arrive as a float), so both the
        float value and None below are within the declared type. See SPEC.md §10
        Type Fidelity.
        """
        upload = {
            "id": 77,
            "filename": "logo.png",
            "description": "Company logo",
            "download_url": "https://3.basecampapi.com/12345/blobs/abc/download/logo.png",
            "description_attachments": [
                {
                    "id": 1069480020,
                    "sgid": "BAh-img",
                    "filename": "brand-guide.png",
                    "content_type": "image/png",
                    "byte_size": 512000,
                    "download_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/download/brand-guide.png",
                    "width": 1024.0,
                    "height": 768,
                    "previewable": True,
                    "preview_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/previews/brand-guide.png",
                    "thumbnail_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/thumbnails/brand-guide.png",
                },
                {
                    "id": 1069480021,
                    "sgid": "BAh-pdf",
                    "filename": "specs.pdf",
                    "content_type": "application/pdf",
                    "byte_size": 2097152,
                    "download_url": "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/download/specs.pdf",
                    "width": None,
                    "height": None,
                    "previewable": False,
                    "preview_url": "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/previews/specs.pdf",
                    "thumbnail_url": "https://3.basecampapi.com/12345/buckets/1/blobs/pdf/thumbnails/specs.pdf",
                },
            ],
        }
        respx.get("https://3.basecampapi.com/12345/uploads/77").mock(return_value=httpx.Response(200, json=upload))

        c = Client(access_token="test-token")
        result = c.for_account("12345").uploads.get(upload_id=77)
        attachments = result["description_attachments"]
        assert len(attachments) == 2

        # Float-spelled 1024.0 is preserved verbatim, as a Python float — the
        # runtime performs no integer coercion (unlike Go's FlexInt).
        assert attachments[0]["width"] == 1024
        assert isinstance(attachments[0]["width"], float)
        assert attachments[0]["height"] == 768
        # None is preserved verbatim, matching the TypedDict's
        # NotRequired[Optional[int | float]] width/height type.
        assert attachments[1]["width"] is None
        assert attachments[1]["height"] is None
