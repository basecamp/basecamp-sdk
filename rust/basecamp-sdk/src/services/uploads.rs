//! Uploads: the generated wire methods plus the `download` composite of SPEC §18, which
//! joins [`UploadsService::get`] to the client-level two-hop download of SPEC §14.

pub use crate::generated::services::uploads::{ListUploadsParams, UploadsService};

use crate::download::DownloadResult;
use crate::error::Error;

impl UploadsService<'_> {
    /// Downloads an upload's file in one call: GETs the upload for its `download_url`,
    /// then fetches that through [`AccountClient::download_url`] — the authenticated
    /// first hop and the bare signed second hop — so hooks observe `GetUpload` and the
    /// download's own requests under their native identities.
    ///
    /// The result's filename prefers the upload metadata's `filename` and falls back to
    /// the last segment of the URL. An upload that carries no `download_url` is a
    /// statusless `api_error`: the record is a file and the API always renders its link.
    ///
    /// [`AccountClient::download_url`]: crate::AccountClient::download_url
    pub async fn download(&self, upload_id: i64) -> Result<DownloadResult, Error> {
        let upload = self.get(upload_id).await?;
        let url = upload
            .download_url
            .as_ref()
            .filter(|url| !url.as_str().is_empty())
            .ok_or_else(|| {
                Error::malformed_response(format!("upload {upload_id} has no download_url"))
            })?;
        let mut result = self.client().download_url(url.as_str()).await?;
        if let Some(filename) = upload.filename.filter(|filename| !filename.is_empty()) {
            result.filename = filename;
        }
        Ok(result)
    }
}
