//! SPEC §18 `uploads.download`, from `conformance/tests/uploads_download.json`.

#![cfg(feature = "reqwest")]

mod composites_support;

use basecamp_sdk::ErrorCode;
use composites_support::run;

const FIXTURE: &str = "uploads_download";
const UPLOAD: i64 = 1_069_479_400;

#[tokio::test]
async fn uploads_download_delegates_through_download_url_primitive() {
    let result = run(
        FIXTURE,
        "UploadsDownload delegates through DownloadURL primitive",
        |account| async move { account.uploads().download(UPLOAD).await },
    )
    .await
    .expect("downloaded");
    assert_eq!(result.body, "pixels");
    assert_eq!(result.content_type, "image/png");
    assert_eq!(result.filename, "logo.png");
}

#[tokio::test]
async fn uploads_download_errors_when_upload_has_no_download_url() {
    let error = run(
        FIXTURE,
        "UploadsDownload errors when upload has no download_url",
        |account| async move { account.uploads().download(UPLOAD).await },
    )
    .await
    .expect_err("refused");
    assert_eq!(error.code(), ErrorCode::ApiError);
    assert_eq!(error.http_status(), None);
    assert!(!error.is_retryable());
}
