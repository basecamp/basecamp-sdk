$version: "2"

namespace basecamp.traits

use smithy.api#documentation
use smithy.api#trait

@trait(selector: "operation")
@documentation("Pagination semantics for BasecampJson protocol")
structure pagination {
  @documentation("Pagination style: link | cursor | none")
  style: String
}

@trait(selector: "operation")
@documentation("Retry semantics for BasecampJson protocol")
structure retry {
  @documentation("max retries, base delay, and backoff formula")
  max: Integer
  base_delay_seconds: Integer
  backoff: String
}

@trait(selector: "operation")
@documentation("Idempotency semantics for BasecampJson protocol")
structure idempotency {
  @documentation("Whether idempotency keys are supported")
  supported: Boolean
}
