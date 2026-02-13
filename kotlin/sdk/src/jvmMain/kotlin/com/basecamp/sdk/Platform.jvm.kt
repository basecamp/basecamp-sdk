package com.basecamp.sdk

import java.util.concurrent.ConcurrentHashMap

@PublishedApi
internal actual fun <V> createServiceCache(): MutableMap<String, V> = ConcurrentHashMap()
