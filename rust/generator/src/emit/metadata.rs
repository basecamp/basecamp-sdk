use std::fmt::Write;

use crate::emit::HEADER;
use crate::model::{Model, Operation};
use crate::naming::constant_name;

/// One labelled struct literal per operation, on one line each (`#[rustfmt::skip]`), so the
/// repository's regex-based parity readers — `check-retry-metadata-parity.py`,
/// `check-idempotency-parity` — can read the tuple the way they read Swift's `Metadata.swift`.
pub(crate) fn render(model: &Model) -> String {
    let mut out = String::from(HEADER);
    out.push_str("//! Per-operation behaviour from `behavior-model.json`: idempotency, read-onlyness and the\n//! retry tuple SPEC §7's three gates consume.\n\n");
    out.push_str("use crate::route::{Backoff, OperationMetadata, RetryConfig};\n\n");
    let mut operations: Vec<&Operation> = model.operations().collect();
    operations.sort_by(|a, b| a.id.cmp(&b.id));
    for operation in &operations {
        writeln!(out, "/// `{}`.", operation.id).unwrap();
        out.push_str("#[rustfmt::skip]\n");
        writeln!(
            out,
            "pub static {}: OperationMetadata = OperationMetadata {{ operation: \"{}\", idempotent: {}, readonly: {}, retry: RetryConfig {{ max_attempts: {}, base_delay_ms: {}, backoff: Backoff::{}, retry_on: &{:?} }} }};",
            constant_name(&operation.id),
            operation.id,
            operation.idempotent,
            operation.readonly,
            operation.retry.max,
            operation.retry.base_delay_ms,
            backoff_variant(&operation.retry.backoff),
            operation.retry.retry_on
        )
        .unwrap();
    }
    out.push_str("\n/// Every operation's metadata, in `operationId` order.\n");
    out.push_str("pub static OPERATIONS: &[&OperationMetadata] = &[\n");
    for operation in &operations {
        writeln!(out, "    &{},", constant_name(&operation.id)).unwrap();
    }
    out.push_str("];\n");
    out
}

pub(crate) fn backoff_variant(backoff: &str) -> &'static str {
    match backoff {
        "exponential" => "Exponential",
        "linear" => "Linear",
        "constant" => "Constant",
        other => panic!("unsupported backoff {other}"),
    }
}
