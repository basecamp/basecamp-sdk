from __future__ import annotations

import time
from typing import Any

from basecamp import _security
from basecamp._pagination import ListMeta, ListResult, parse_next_link, parse_total_count
from basecamp.errors import ApiError
from basecamp.hooks import OperationInfo, OperationResult, safe_hook


def _normalize_person_ids(obj: Any) -> None:
    """Normalize Person-shaped objects in API responses.

    See _base.py for full docstring.
    """
    if isinstance(obj, list):
        for item in obj:
            _normalize_person_ids(item)
    elif isinstance(obj, dict):
        if "personable_type" in obj and isinstance(obj.get("id"), str):
            raw_id = obj["id"]
            try:
                obj["id"] = int(raw_id)
            except ValueError:
                obj["system_label"] = raw_id
                obj["id"] = 0
        for val in obj.values():
            if isinstance(val, (dict, list)):
                _normalize_person_ids(val)


class AsyncBaseService:
    """Base class for async service classes."""

    def __init__(self, client) -> None:
        self._client = client
        self._account_id = client.account_id
        self._hooks = client.hooks

    async def _request(
        self,
        info: OperationInfo,
        method: str,
        path: str,
        *,
        json_body: dict | None = None,
        params: dict | None = None,
        operation: str | None = None,
    ) -> dict:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            if method == "GET":
                response = await self._client.http.get(self._client.account_path(path), params=params)
            elif method == "POST":
                response = await self._client.http.post(
                    self._client.account_path(path), json_body=json_body, operation=operation
                )
            elif method == "PUT":
                response = await self._client.http.put(
                    self._client.account_path(path), json_body=json_body, operation=operation
                )
            elif method == "DELETE":
                response = await self._client.http.delete(self._client.account_path(path), operation=operation)
            else:
                raise ValueError(f"Unsupported method: {method}")
            result = response.json()
            _normalize_person_ids(result)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_void(
        self,
        info: OperationInfo,
        method: str,
        path: str,
        *,
        json_body: dict | None = None,
        operation: str | None = None,
    ) -> None:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            if method == "POST":
                await self._client.http.post(self._client.account_path(path), json_body=json_body, operation=operation)
            elif method == "PUT":
                await self._client.http.put(self._client.account_path(path), json_body=json_body, operation=operation)
            elif method == "DELETE":
                await self._client.http.delete(self._client.account_path(path), operation=operation)
            else:
                raise ValueError(f"Unsupported method: {method}")
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_multipart_void(
        self,
        info: OperationInfo,
        method: str,
        path: str,
        *,
        field: str,
        content: bytes,
        filename: str,
        content_type: str,
        operation: str | None = None,
    ) -> None:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            await self._client.http.request_multipart(
                method,
                self._client.account_path(path),
                field=field,
                content=content,
                filename=filename,
                content_type=content_type,
                operation=operation,
            )
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_raw(
        self,
        info: OperationInfo,
        path: str,
        *,
        content: bytes,
        content_type: str,
        params: dict | None = None,
        operation: str | None = None,
    ) -> dict:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            response = await self._client.http.post_raw(
                self._client.account_path(path),
                content=content,
                content_type=content_type,
                params=params,
                operation=operation,
            )
            result = response.json()
            _normalize_person_ids(result)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_paginated(
        self,
        info: OperationInfo,
        path: str,
        *,
        params: dict | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            result = await self._paginate(path, params=params, max_items=max_items)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_paginated_key(
        self,
        info: OperationInfo,
        path: str,
        key: str,
        *,
        params: dict | None = None,
    ) -> ListResult:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            result = await self._paginate_key(path, key, params=params)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _request_paginated_wrapped(
        self,
        info: OperationInfo,
        path: str,
        key: str,
        *,
        params: dict | None = None,
    ) -> dict:
        start = time.monotonic()
        safe_hook(self._hooks.on_operation_start, info)
        try:
            result = await self._paginate_wrapped(path, key, params=params)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self._hooks.on_operation_end, info, OperationResult(duration_ms=duration_ms, error=e))
            raise

    async def _paginate(self, path: str, *, params: dict | None = None, max_items: int | None = None) -> ListResult:
        base_url = self._client.http._build_url(self._client.account_path(path))
        url = base_url
        all_items: list = []
        total_count = 0
        truncated = False

        for page in range(1, self._client.config.max_pages + 1):
            safe_hook(self._hooks.on_paginate, url, page)
            response = await self._client.http.get(url, params=params if page == 1 else None)
            _security.check_body_size(response.content, _security.MAX_RESPONSE_BODY_BYTES)

            if page == 1:
                total_count = parse_total_count(dict(response.headers))

            try:
                items = response.json()
                _normalize_person_ids(items)
            except Exception as e:
                raise ApiError(f"Failed to parse paginated response (page {page}): {_security.truncate(str(e))}") from e

            all_items.extend(items)

            if max_items and len(all_items) >= max_items:
                all_items = all_items[:max_items]
                truncated = True
                break

            next_url = parse_next_link(response.headers.get("link"))
            if not next_url:
                break

            next_url = _security.resolve_url(url, next_url)
            if not _security.same_origin(next_url, base_url):
                raise ApiError(f"Pagination Link header points to different origin: {_security.truncate(next_url)}")

            url = next_url
        else:
            truncated = True

        return ListResult(all_items, ListMeta(total_count=total_count, truncated=truncated))

    async def _paginate_key(self, path: str, key: str, *, params: dict | None = None) -> ListResult:
        base_url = self._client.http._build_url(self._client.account_path(path))
        url = base_url
        all_items: list = []
        total_count = 0

        for page in range(1, self._client.config.max_pages + 1):
            safe_hook(self._hooks.on_paginate, url, page)
            response = await self._client.http.get(url, params=params if page == 1 else None)
            _security.check_body_size(response.content, _security.MAX_RESPONSE_BODY_BYTES)

            if page == 1:
                total_count = parse_total_count(dict(response.headers))

            try:
                data = response.json()
                _normalize_person_ids(data)
            except Exception as e:
                raise ApiError(f"Failed to parse paginated response (page {page}): {_security.truncate(str(e))}") from e

            items = data.get(key, [])
            all_items.extend(items)

            next_url = parse_next_link(response.headers.get("link"))
            if not next_url:
                break

            next_url = _security.resolve_url(url, next_url)
            if not _security.same_origin(next_url, base_url):
                raise ApiError(f"Pagination Link header points to different origin: {_security.truncate(next_url)}")

            url = next_url

        return ListResult(all_items, ListMeta(total_count=total_count))

    async def _paginate_wrapped(self, path: str, key: str, *, params: dict | None = None) -> dict:
        base_url = self._client.http._build_url(self._client.account_path(path))

        safe_hook(self._hooks.on_paginate, base_url, 1)
        first_response = await self._client.http.get(base_url, params=params)
        _security.check_body_size(first_response.content, _security.MAX_RESPONSE_BODY_BYTES)

        total_count = parse_total_count(dict(first_response.headers))

        try:
            first_data = first_response.json()
            _normalize_person_ids(first_data)
        except Exception as e:
            raise ApiError(f"Failed to parse paginated response (page 1): {_security.truncate(str(e))}") from e

        wrapper = {k: v for k, v in first_data.items() if k != key}
        all_items = list(first_data.get(key, []))

        next_link = parse_next_link(first_response.headers.get("link"))
        url = base_url
        page = 1

        while next_link and page < self._client.config.max_pages:
            page += 1
            next_url = _security.resolve_url(url, next_link)
            if not _security.same_origin(next_url, base_url):
                raise ApiError(f"Pagination Link header points to different origin: {_security.truncate(next_url)}")

            safe_hook(self._hooks.on_paginate, next_url, page)
            response = await self._client.http.get(next_url)
            _security.check_body_size(response.content, _security.MAX_RESPONSE_BODY_BYTES)

            try:
                data = response.json()
                _normalize_person_ids(data)
            except Exception as e:
                raise ApiError(f"Failed to parse paginated response (page {page}): {_security.truncate(str(e))}") from e

            all_items.extend(data.get(key, []))
            next_link = parse_next_link(response.headers.get("link"))
            url = next_url

        wrapper[key] = ListResult(all_items, ListMeta(total_count=total_count))
        return wrapper

    def _compact(self, **kwargs: Any) -> dict:
        return {k: v for k, v in kwargs.items() if v is not None}

    def _bucket_path(self, project_id: int | str, path: str) -> str:
        return f"/buckets/{project_id}{path}"
