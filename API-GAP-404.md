# API gap: uploading a new version of an existing file

Addresses basecamp/basecamp-cli#404.

## Question

basecamp-cli#404 asks the SDK to support uploading a **new version** of an
existing uploaded file. The issue hypothesizes that `PUT /uploads/{id}.json`
with a fresh `attachable_sgid` replaces the file and creates a version event,
the way the read-only [Get upload versions][versions] endpoint
(`GET /uploads/{id}/versions.json`, "each version event represents a file
replacement") implies file replacement is a real server-side concept.

The plumbing would be small: add `AttachableSGID` to `UpdateUploadRequest`
(`go/pkg/basecamp/vaults.go`) and map it into the request body in
`UploadsService.Update`. Before shipping that, the server behavior had to be
verified, because a mapping that the server silently drops is worse than no
method at all — it would look like it worked.

## Finding: the API ignores `attachable_sgid` on update

**Verified against the BC3 source pinned in `spec/api-provenance.json`
(`basecamp/bc3` @ `ba105ba7d7e48bd97afdc98305e9fb8a63a88beb`).** This is a
static source read of the pinned revision, not a live-account request — see
*Verification status* below.

`PUT /uploads/{id}.json` is served by `UploadsController#update`. The update
path never reads `attachable_sgid`:

```ruby
# app/controllers/uploads_controller.rb @ ba105ba7
class UploadsController < ApplicationController
  wrap_parameters :upload, include: %i[ base_name description ]

  before_action :set_new_upload, only: :create      # <- attachable_sgid consumed here, create only

  def update
    @recording.update! recordable: @upload.changing(upload_params), status: status_param
    # ...
  end

  private
    def upload_params
      params[:upload]&.permit(:base_name, :description) || {}   # <- only these two
    end

    def uploadable_params
      params.permit(:attachable_sgid, :file)                    # <- used only by set_new_upload (create)
    end
end
```

Three independent reasons the `attachable_sgid` sent to `update` is a no-op:

1. **`wrap_parameters :upload, include: %i[base_name description]`** — only
   `base_name` and `description` are wrapped into the `:upload` params the
   update reads. `attachable_sgid` is not among them.
2. **`upload_params` permits only `:base_name, :description`.** The update
   calls `@upload.changing(upload_params)`; `attachable_sgid` is never passed
   in.
3. **`uploadable_params` (which does permit `:attachable_sgid`) is consumed
   solely by `set_new_upload`**, a `before_action` scoped `only: :create`. The
   update action never calls it.

Even the version-event machinery cannot fire on an API update. `Upload#changing`
re-attaches the *existing* blob:

```ruby
# app/models/upload.rb @ ba105ba7
def changing(attributes)
  super.tap { |copy| copy.attach(blob) }    # re-attaches the SAME blob
end

def track_blob_change(recording)
  if previous_recordable = recording.last_version_event&.recordable
    if previous_recordable.blob != blob            # never true on a metadata-only update
      recording.track_event :blob_changed, recordable_previous: previous_recordable
    end
  end
end
```

Because the blob is unchanged, `previous_recordable.blob != blob` is false and
no `blob_changed` version event is created.

The route table confirms there is no API write path for versions at all:

```ruby
# config/routes.rb @ ba105ba7  (API namespace)
resources :uploads, only: %i[ show edit update destroy ] do
  resources :versions, only: %i[ index ], controller: "uploads/versions"   # read-only
end
```

The documentation matches the code. `doc/api/sections/uploads.md` documents
**Update an upload** as changing `description` and `base_name` only, and
**Get upload versions** as read-only. File replacement in the product happens
through a web-only path, not the JSON API.

## Conclusion

`PUT /uploads/{id}.json` accepts a new `attachable_sgid` on the wire (Rails
strong-params silently discard the unpermitted key) but **does not act on it**:
the file is not replaced and no version is created. Only `description` and
`base_name` are mutable through the JSON API. The basecamp-cli#404 hypothesis
is **not supported** by the current server.

Per the SDK's hard rule against shipping a wire method the server silently
drops, `AttachableSGID` was **not** added to `UpdateUploadRequest`. Doing so
would present a working "upload new version" affordance that never replaces
anything.

## Verification status

- **Source-verified** against `basecamp/bc3` @ `ba105ba7` (the revision
  `spec/api-provenance.json` pins). Controller, model, routes, and published
  API docs all agree.
- **Not** verified against a live account. A live confirmation is not required
  to reject the change — the source is unambiguous that the update path drops
  the key — but if the API team later adds a version-write contract, absorption
  should be confirmed end-to-end before the CLI exposes a user-facing command.

## Recommendation

1. Do not add `attachable_sgid` to the upload-update SDK surface while the
   server ignores it.
2. Raise the capability with the API team via the tracked registry entry
   [`spec/api-gaps/upload-new-version.md`](spec/api-gaps/upload-new-version.md).
   The read side (versions list) is already modeled and absorbed
   (`UploadsService.ListVersions`); only the **write** side (replace file /
   create version) is missing a JSON contract.
3. If and when BC3 ships a version-write contract, absorb it through the
   Smithy spec + regeneration and confirm against a live account before the
   CLI exposes "upload new version."

[versions]: https://github.com/basecamp/bc3-api/blob/master/sections/uploads.md
