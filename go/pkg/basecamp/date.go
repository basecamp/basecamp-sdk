package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/types"

// Date is an alias for types.Date, representing a calendar date without time.
// Re-exported here for convenience so users can use basecamp.Date.
type Date = types.Date

// ParseDate parses a string in YYYY-MM-DD format.
var ParseDate = types.ParseDate

// DateOf returns the Date portion of a time.Time.
var DateOf = types.DateOf

// Today returns today's date in the local timezone.
var Today = types.Today
