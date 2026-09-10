//! Documents: the generated wire methods plus the merge-safe `update` and
//! read-modify-write `edit` of SPEC §5 "Merge-Safe Write Surface (Documents)".
//!
//! `PUT /documents/{documentId}` is a full replace: BC3 builds a brand-new `Document`
//! from the permitted params and swaps it in, so a body that omits `content` erases it
//! and one that omits `title` leaves the document reading back as "Untitled" — neither
//! is a 422. The composites GET the current document, change what the caller asked for,
//! and PUT `{title, content}` back through [`DocumentsService::replace`], so hooks
//! observe `GetDocument` then `ReplaceDocument` under their own identities.
//!
//! Subscribers are untouched: a body naming neither `subscriptions` nor `notify` keeps a
//! drafted document's subscriber list, which is what makes the composite safe on a draft.
//!
//! Neither composite is atomic: a concurrent write between the GET and the PUT is
//! overwritten, last write wins, with a window of one round-trip. Use `replace` to
//! overwrite deliberately.

pub use crate::generated::services::documents::{DocumentsService, ListDocumentsParams};

use crate::error::Error;
use crate::generated::types::{Document, ReplaceDocumentRequestContent};

/// The fields a merge-safe [`DocumentsService::update`] may set. A `None` member is left
/// untouched, guaranteed; `Some(String::new())` clears.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct UpdateDocumentRequest {
    /// Plain-text title. Cleared, the document reads back as "Untitled".
    pub title: Option<String>,
    /// Rich text body (HTML).
    pub content: Option<String>,
}

/// A document's full writable state, handed to the [`DocumentsService::edit`] closure.
/// The whole value is PUT back, so clearing a field means setting it `""`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DocumentFields {
    /// Plain-text title. Set `""` to clear; the document then reads back as "Untitled".
    pub title: String,
    /// Rich text body (HTML). Set `""` to clear.
    pub content: String,
}

impl DocumentFields {
    /// `title` is required on the response and BC3 can never render it blank
    /// (`Document#title` falls back to "Untitled"), so an absent, null or blank one in a
    /// 2xx body is malformed rather than empty: resending it would blank a real title on
    /// a call that only touched `content`. `content` is optional on the response, so an
    /// absent one is genuinely empty.
    fn from_document(document: &Document) -> Result<DocumentFields, Error> {
        if document.title.trim().is_empty() {
            return Err(Error::malformed_response(
                "GetDocument returned a document with a blank \"title\", which the API never \
                 renders",
            )
            .with_hint(MALFORMED_HINT));
        }
        Ok(DocumentFields {
            title: document.title.clone(),
            content: document.content.clone().unwrap_or_default(),
        })
    }

    fn apply(&mut self, request: &UpdateDocumentRequest) {
        if let Some(title) = &request.title {
            self.title.clone_from(title);
        }
        if let Some(content) = &request.content {
            self.content.clone_from(content);
        }
    }

    /// Both fields always, empties included: `""` is how a clear is spelled on a
    /// full-replace endpoint, never `null` (SPEC §18) and never omission.
    fn into_body(self) -> ReplaceDocumentRequestContent {
        ReplaceDocumentRequestContent {
            title: Some(self.title),
            content: Some(self.content),
        }
    }
}

const MALFORMED_HINT: &str = "The merge-safe update and edit resend this record's fields \
                              verbatim, so a malformed response cannot be written back \
                              safely. Use replace to write the record deliberately.";

impl DocumentsService<'_> {
    /// Sets the given fields on a document and preserves the rest: GETs the current
    /// document, overlays the `Some` members of `request`, and PUTs `{title, content}`
    /// back through [`DocumentsService::replace`].
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn update(
        &self,
        document_id: i64,
        request: &UpdateDocumentRequest,
    ) -> Result<Document, Error> {
        let mut fields = DocumentFields::from_document(&self.get(document_id).await?)?;
        fields.apply(request);
        self.replace(document_id, &fields.into_body()).await
    }

    /// Applies a read-modify-write closure to a document: GETs the current document,
    /// hands the closure its writable state, and PUTs it back through
    /// [`DocumentsService::replace`]. An error from the closure aborts before anything is
    /// written.
    ///
    /// ```no_run
    /// # async fn example(account: basecamp_sdk::AccountClient) -> Result<(), basecamp_sdk::Error> {
    /// account
    ///     .documents()
    ///     .edit(456, |document| {
    ///         document.title = format!("🚨 {}", document.title);
    ///         document.content = String::new();
    ///         Ok(())
    ///     })
    ///     .await?;
    /// # Ok(())
    /// # }
    /// ```
    ///
    /// Not atomic — see the module docs for the GET→PUT race.
    pub async fn edit<F>(&self, document_id: i64, mutate: F) -> Result<Document, Error>
    where
        F: FnOnce(&mut DocumentFields) -> Result<(), Error>,
    {
        let mut fields = DocumentFields::from_document(&self.get(document_id).await?)?;
        mutate(&mut fields)?;
        self.replace(document_id, &fields.into_body()).await
    }
}
