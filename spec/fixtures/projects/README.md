# Projects Fixtures

JSON fixtures extracted from bc3-api/sections/projects.md for golden tests.

## Fixture to Operation Mapping

| Fixture | Smithy Operation | HTTP | Description |
|---------|-----------------|------|-------------|
| list.json | ListProjects | GET /projects.json | Array of 2 projects |
| get.json | GetProject | GET /projects/{id}.json | Single project |
| create-request.json | CreateProject (input) | POST /projects.json | Minimal create body |
| update-request.json | UpdateProject (input) | PUT /projects/{id}.json | Full update with schedule |
| error-limit.json | CreateProject (error) | 507 response | Project limit exceeded |

## Golden Test Cases

### ListProjects
- **list_active**: GET /projects.json -> list.json (status=active default)
- **list_archived**: GET /projects.json?status=archived -> (need fixture)
- **list_trashed**: GET /projects.json?status=trashed -> (need fixture)

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

- list.json includes a project with `client_company` and `clientside` fields (id: 2085958500)
- get.json is a basic project without client fields
- DockItem.position can be null when enabled=false
- All timestamps are ISO8601 format
