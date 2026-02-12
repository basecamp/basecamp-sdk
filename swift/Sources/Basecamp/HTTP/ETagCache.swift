import Foundation

/// In-memory cache for ETag-based HTTP caching.
///
/// Thread-safe via `NSLock`. Bounded to `maxEntries` to prevent unbounded growth.
/// When the cache is full, the oldest entry (by insertion order) is evicted (FIFO).
package final class ETagCache: Sendable {
    package static let defaultMaxEntries = 1000

    private struct Entry: Sendable {
        let etag: String
        let data: Data
    }

    private let lock = NSLock()
    private let maxEntries: Int
    // Nonisolated(unsafe) because access is serialized by NSLock
    nonisolated(unsafe) private var entries: [String: Entry] = [:]
    nonisolated(unsafe) private var insertionOrder: [String] = []

    package init(maxEntries: Int = defaultMaxEntries) {
        self.maxEntries = maxEntries
    }

    /// Returns the cached ETag for a URL, or nil if not cached.
    package func etag(for url: String) -> String? {
        lock.withLock {
            entries[url]?.etag
        }
    }

    /// Returns the cached response data for a URL, or nil if not cached.
    package func data(for url: String) -> Data? {
        lock.withLock {
            entries[url]?.data
        }
    }

    /// Stores a response with its ETag for a URL.
    package func store(url: String, data: Data, etag: String) {
        lock.withLock {
            // Remove existing entry from insertion order if updating
            if entries[url] != nil {
                insertionOrder.removeAll { $0 == url }
            }

            // Evict oldest if at capacity
            if entries.count >= maxEntries, let oldest = insertionOrder.first {
                entries.removeValue(forKey: oldest)
                insertionOrder.removeFirst()
            }

            entries[url] = Entry(etag: etag, data: data)
            insertionOrder.append(url)
        }
    }

    /// Removes all cached entries.
    package func removeAll() {
        lock.withLock {
            entries.removeAll()
            insertionOrder.removeAll()
        }
    }
}
