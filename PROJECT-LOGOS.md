# Project logos

Project detail reads expose optional `logo: { url }` when an image is attached.
Project lists and home responses omit it. An absent logo is omitted, not `null`.
The URL is an authenticated API-host endpoint that redirects to storage: fetch
it with the SDK's existing authenticated URL-download helper, not an image tag
or an unauthenticated HTTP client. No separate logo download operation is needed.

| SDK | Upload / replace | Remove | Download |
|---|---|---|---|
| Go | `Projects().UpdateLogo` | `Projects().RemoveLogo` | `DownloadURL` |
| TypeScript | `projects.updateProjectLogo` | `projects.removeProjectLogo` | `downloadURL` |
| Ruby | `projects.update_project_logo` | `projects.remove_project_logo` | `download_url` |
| Swift | `projects.updateProjectLogo` | `projects.removeProjectLogo` | `downloadURL` |
| Kotlin | `projects.updateProjectLogo` | `projects.removeProjectLogo` | `downloadURL` |
| Python (sync and async) | `projects.update_project_logo` | `projects.remove_project_logo` | `download_url` |
| Rust | `projects().update_project_logo` | `projects().remove_project_logo` | `download_url` |

Uploads use multipart field `logo`, with a filename and content type, at
`PUT /projects/{projectId}/logo.json`. Maximum size is 5 MiB; existing project
image formats apply. The server returns 204 for upload and removal. Removal is
idempotent, including when no logo is attached. Invalid replacements return a
validation error without replacing the existing image. Project-edit permissions
apply to people and agents who can edit the project. Others receive 403; projects
outside the caller's visibility receive 404.

These operations do not change the project wallpaper, layout, name or description.
Wallpaper and logo are independent attachments. Uploading during project creation,
attachment-SGID inputs, and public image links are not part of this surface.

## Go example

```go
if err := account.Projects().UpdateLogo(ctx, projectID, image, "logo.png", "image/png"); err != nil {
    return err
}
project, err := account.Projects().Get(ctx, projectID)
if err != nil {
    return err
}
if project.Logo != nil {
    result, err := account.DownloadURL(ctx, project.Logo.URL)
    if err != nil {
        return err
    }
    defer result.Body.Close()
    // Consume result.Body.
}
return account.Projects().RemoveLogo(ctx, projectID)
```

Before upgrading, see the migration notes for
[Go generated-client interfaces](MIGRATING.md#go-generated-client-interfaces-gain-project-logo-methods)
and [Go/Kotlin Project models](MIGRATING.md#go-and-kotlin-project-gains-an-optional-logo).
Custom Go interface implementations must add the new methods or regenerate
mocks; unkeyed Go literals and positional Kotlin model calls also need attention.
