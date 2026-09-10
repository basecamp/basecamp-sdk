use std::fmt::Write;

use crate::emit::HEADER;
use crate::model::Model;

/// The service accessors on `AccountClient`, one per service, in the shape
/// `scripts/check-service-inventory-parity` reads Kotlin's and Swift's accessor files in.
pub fn render(model: &Model) -> String {
    let mut out = String::from(HEADER);
    out.push_str("//! The service accessors on [`AccountClient`], one per generated service.\n\n");
    out.push_str("use crate::client::AccountClient;\n");
    out.push_str("use crate::generated::services;\n\n");
    out.push_str("#[rustfmt::skip]\nimpl AccountClient {\n");
    for service in &model.services {
        writeln!(out, "    /// `{}` operations.\n    pub fn {}(&self) -> services::{}::{}<'_> {{\n        services::{}::{}::new(self)\n    }}\n",
            service.name, service.module, service.module, service.struct_name, service.module, service.struct_name
        )
        .unwrap();
    }
    out.push_str("}\n");
    out
}
