//! The SPEC §18 hand-written composites: merge-safe writes over the generated wire
//! operations. Nothing here touches the wire on its own.
//!
//! Each composite module re-exports the generated service it extends, so
//! `services::todos::TodosService` is one type carrying both the wire methods and the
//! composites built over them.

pub use crate::generated::services::*;

pub mod cards;
pub mod documents;
pub mod schedules;
pub mod todolists;
pub mod todos;
pub mod uploads;

use crate::types::Date;

/// How a write addresses a date member: `Option<DateChange>` is the tri-state — `None`
/// leaves the date alone, [`DateChange::Clear`] removes it, [`DateChange::On`] sets it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum DateChange {
    /// Remove the date.
    Clear,
    /// Set the date.
    On(Date),
}

impl From<Date> for DateChange {
    fn from(date: Date) -> DateChange {
        DateChange::On(date)
    }
}
