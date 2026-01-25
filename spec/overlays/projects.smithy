$version: "2"

namespace basecamp

use basecamp.traits#pagination
use basecamp.traits#retry
use basecamp.traits#idempotency

apply ListProjects @pagination({ style: "link" })
apply ListProjects @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })

apply GetProject @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })

apply CreateProject @idempotency({ supported: false })
apply CreateProject @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })

apply UpdateProject @idempotency({ supported: false })
apply UpdateProject @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })

apply TrashProject @idempotency({ supported: false })
apply TrashProject @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })
