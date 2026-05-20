package com.basecamp.sdk

import kotlin.time.Duration

// Disables HttpTimeout by default. Ktor 3.5.0 (KTOR-8271) made HttpTimeout
// honor the kotlinx-coroutines test scheduler's virtual clock; runTest then
// auto-advances past the timeout while MockEngine's IO-dispatched response is
// in flight, firing HttpRequestTimeoutException before the mock can respond.
// Override `timeout` inside the block to test timeout behavior explicitly.
internal fun testBasecampClient(block: BasecampClientBuilder.() -> Unit): BasecampClient =
    BasecampClient {
        timeout = Duration.INFINITE
        block()
    }
