# Projects Fixtures

JSON fixtures extracted from the canonical projects docs in `basecamp/bc3/doc/api/sections/projects.md` for golden tests.

## Fixture to Operation Mapping

| Fixture | Smithy Operation | HTTP | Description |
|---------|-----------------|------|-------------|
| list.json | ListProjects | GET /projects.json | Array of 2 projects |
| recent.json | ListRecentProjects | GET /my/recent_projects.json | Array of 2 recently visited projects |
| get.json | GetProject | GET /projects/{id}.json | Single project |
| create-request.json | CreateProject (input) | POST /projects.json | Minimal create body |
| update-request.json | UpdateProject (input) | PUT /projects/{id}.json | Full update with schedule |
| error-limit.json | CreateProject (error) | 507 response | Project limit exceeded |

## Golden Test Cases

### ListProjects
- **list_active**: GET /projects.json -> list.json (status=active default)
- **list_archived**: GET /projects.json?status=archived -> (need fixture)
- **list_trashed**: GET /projects.json?status=trashed -> (need fixture)

### ListRecentProjects
- **list_recent**: GET /my/recent_projects.json -> recent.json (most recent visit first)

### GetProject
- **get_by_id**: GET /projects/2085958499.json -> get.json
- **get_not_found**: GET /projects/999.json -> 404 error

### CreateProject
- **create_minimal**: POST /projects.json + create-request.json -> get.json (201)
- **create_limit_error**: POST /projects.json -> error-limit.json (507)

### UpdateProject
- **update_full**: PUT /projects/1.json + update-request.json -> get.json (200)
- **update_name_only**: PUT /projects/1.json + {name} -> get.json (200)

### TrashProject
- **trash_success**: DELETE /projects/1.json -> 204 No Content
- **trash_not_found**: DELETE /projects/999.json -> 404 error

## Notes

- get.json includes a scheduled project with `start_date` and `end_date`
- list.json includes one scheduled project and one project with `client_company` and `clientside` fields (id: 2085958500)
- `bookmarked` and `starred` describe the current user's home page (BC3 #13042): `bookmarked` is true when the project is pinned there at all, `starred` only when the pin carries a star — so `starred` implies `bookmarked`, never the reverse. list.json carries one starred project and one bookmarked-but-unstarred project (id: 2085958500), so a decoder that conflated the two would fail on the second
- `star_url` is the bucket stars collection, `/buckets/{id}/stars.json`
- recent.json is the recently-visited list (BC3 #13043): the standard project projection plus `bookmarked` only — the wire omits `starred` here, which is fine because `starred` is optional. Entries are ordered most recent visit first, and one entry is unpinned (`bookmarked: false`) so the flag is exercised both ways
- DockItem.position can be null when enabled=false
- All timestamps are ISO8601 format
