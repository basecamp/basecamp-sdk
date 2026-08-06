"""Tests for the uploads.download(upload_id) convenience (sync + async)."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import LimitExceededError, UsageError

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def load_fixture(rel: str) -> dict:
    return json.loads((_FIXTURES / rel).read_text(encoding="utf-8"))


def _metadata(upload_id: int = 1069479400, *, download_url, filename="report.pdf") -> dict:
    """Upload metadata sourced from the validated fixture; callers override download_url/filename."""
    return {
        **load_fixture("uploads/get.json"),
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


class TestListVersions:
    """The endpoint returns EVENTS, not Uploads — the retype that closes #649."""

    @respx.mock
    def test_versions_carry_the_file_each_one_recorded(self):
        versions = json.loads((_FIXTURES / "uploads" / "versions.json").read_text(encoding="utf-8"))
        respx.get("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(200, json=versions)
        )

        c = Client(access_token="test-token")
        result = c.for_account("12345").uploads.list_versions(upload_id=77)

        assert len(result) == 3
        assert result[0]["action"] == "blob_changed"
        assert result[0]["upload"]["filename"] == "company-logo.png"
        assert result[0]["upload"]["byte_size"] == 184829

    @respx.mock
    def test_exactly_one_version_is_current(self):
        versions = json.loads((_FIXTURES / "uploads" / "versions.json").read_text(encoding="utf-8"))
        respx.get("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(200, json=versions)
        )

        c = Client(access_token="test-token")
        result = c.for_account("12345").uploads.list_versions(upload_id=77)

        assert sum(1 for v in result if v.get("upload", {}).get("current")) == 1
        assert result[0]["upload"]["current"] is True

    @respx.mock
    def test_tolerates_a_version_whose_recordable_is_gone(self):
        versions = json.loads((_FIXTURES / "uploads" / "versions.json").read_text(encoding="utf-8"))
        respx.get("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(200, json=versions)
        )

        c = Client(access_token="test-token")
        result = c.for_account("12345").uploads.list_versions(upload_id=77)

        assert result[2]["action"] == "created"
        assert "upload" not in result[2]


class TestCreateVersion:
    @respx.mock
    def test_posts_the_attachable_sgid(self):
        route = respx.post("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(201, json={"id": 77, "filename": "company-logo.png"})
        )

        c = Client(access_token="test-token")
        result = c.for_account("12345").uploads.create_version(upload_id=77, attachable_sgid="sgid-abc")

        assert result["id"] == 77
        assert json.loads(route.calls[0].request.content)["attachable_sgid"] == "sgid-abc"

    # Presence-aware: omitted carries the previous description forward, ""
    # clears. _compact strips None only, so "" survives to the wire — which is
    # exactly why "" is the SDK's clear spelling rather than None.
    @respx.mock
    def test_omits_an_unaddressed_description(self):
        route = respx.post("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(201, json={"id": 77})
        )

        c = Client(access_token="test-token")
        c.for_account("12345").uploads.create_version(upload_id=77, attachable_sgid="sgid-abc")

        body = json.loads(route.calls[0].request.content)
        assert "description" not in body
        assert "base_name" not in body

    @respx.mock
    def test_sends_an_explicit_blank_description_to_clear_it(self):
        route = respx.post("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(201, json={"id": 77})
        )

        c = Client(access_token="test-token")
        c.for_account("12345").uploads.create_version(upload_id=77, attachable_sgid="sgid-abc", description="")

        body = json.loads(route.calls[0].request.content)
        assert "description" in body
        assert body["description"] == ""

    @respx.mock
    def test_passes_notify_and_subscriptions(self):
        route = respx.post("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(201, json={"id": 77})
        )

        c = Client(access_token="test-token")
        c.for_account("12345").uploads.create_version(
            upload_id=77, attachable_sgid="sgid-abc", notify="custom", subscriptions=[1049715915]
        )

        body = json.loads(route.calls[0].request.content)
        assert body["notify"] == "custom"
        assert body["subscriptions"] == [1049715915]

    # A replacement copies bytes into a new blob and keeps the old one, so it
    # always grows recorded storage. 507 is a limit, never a transient failure.
    @respx.mock
    def test_storage_limit_is_limit_exceeded_and_not_retried(self):
        route = respx.post("https://3.basecampapi.com/12345/uploads/77/versions.json").mock(
            return_value=httpx.Response(507, json={"error": "The storage limit for this account has been reached."})
        )

        c = Client(access_token="test-token")
        with pytest.raises(LimitExceededError) as excinfo:
            c.for_account("12345").uploads.create_version(upload_id=77, attachable_sgid="sgid-abc")

        err = excinfo.value
        assert err.code == "limit_exceeded"
        assert err.exit_code == 10
        assert err.retryable is False
        assert "storage limit" in str(err)
        assert route.call_count == 1
