package com.basecamp.sdk

/** Creates a thread-safe mutable map for service caching. */
@PublishedApi
internal expect fun <V> createServiceCache(): MutableMap<String, V>
