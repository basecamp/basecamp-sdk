import XCTest
@testable import Basecamp

final class ETagCacheTests: XCTestCase {

    // MARK: - Store and Retrieve

    func testStoreAndRetrieveETag() {
        let cache = ETagCache(maxEntries: 10)
        cache.store(url: "https://example.com/a", data: Data("body-a".utf8), etag: "\"etag-a\"")

        XCTAssertEqual(cache.etag(for: "https://example.com/a"), "\"etag-a\"")
        XCTAssertEqual(cache.data(for: "https://example.com/a"), Data("body-a".utf8))
    }

    func testStoreMultipleURLs() {
        let cache = ETagCache(maxEntries: 10)
        cache.store(url: "https://example.com/a", data: Data("a".utf8), etag: "\"a\"")
        cache.store(url: "https://example.com/b", data: Data("b".utf8), etag: "\"b\"")

        XCTAssertEqual(cache.etag(for: "https://example.com/a"), "\"a\"")
        XCTAssertEqual(cache.etag(for: "https://example.com/b"), "\"b\"")
    }

    // MARK: - Cache Miss

    func testMissOnUnknownURL() {
        let cache = ETagCache(maxEntries: 10)
        XCTAssertNil(cache.etag(for: "https://example.com/unknown"))
        XCTAssertNil(cache.data(for: "https://example.com/unknown"))
    }

    // MARK: - Eviction at Capacity (FIFO)

    func testEvictsOldestAtCapacity() {
        let cache = ETagCache(maxEntries: 2)
        cache.store(url: "https://example.com/1", data: Data("1".utf8), etag: "\"e1\"")
        cache.store(url: "https://example.com/2", data: Data("2".utf8), etag: "\"e2\"")

        // This should evict /1
        cache.store(url: "https://example.com/3", data: Data("3".utf8), etag: "\"e3\"")

        XCTAssertNil(cache.etag(for: "https://example.com/1"), "Oldest entry should be evicted")
        XCTAssertEqual(cache.etag(for: "https://example.com/2"), "\"e2\"")
        XCTAssertEqual(cache.etag(for: "https://example.com/3"), "\"e3\"")
    }

    func testEvictionIsFIFO() {
        let cache = ETagCache(maxEntries: 3)
        cache.store(url: "https://example.com/a", data: Data("a".utf8), etag: "\"a\"")
        cache.store(url: "https://example.com/b", data: Data("b".utf8), etag: "\"b\"")
        cache.store(url: "https://example.com/c", data: Data("c".utf8), etag: "\"c\"")

        // Evicts /a
        cache.store(url: "https://example.com/d", data: Data("d".utf8), etag: "\"d\"")
        XCTAssertNil(cache.etag(for: "https://example.com/a"))
        XCTAssertNotNil(cache.etag(for: "https://example.com/b"))

        // Evicts /b
        cache.store(url: "https://example.com/e", data: Data("e".utf8), etag: "\"e\"")
        XCTAssertNil(cache.etag(for: "https://example.com/b"))
        XCTAssertNotNil(cache.etag(for: "https://example.com/c"))
    }

    // MARK: - Update in Place Does Not Evict (Bug Fix)

    func testUpdateInPlaceAtCapacityDoesNotEvict() {
        let cache = ETagCache(maxEntries: 2)
        cache.store(url: "https://example.com/1", data: Data("v1".utf8), etag: "\"e1\"")
        cache.store(url: "https://example.com/2", data: Data("v2".utf8), etag: "\"e2\"")

        // Update /1 — should NOT evict /2
        cache.store(url: "https://example.com/1", data: Data("v1-updated".utf8), etag: "\"e1-new\"")

        XCTAssertEqual(cache.etag(for: "https://example.com/1"), "\"e1-new\"")
        XCTAssertEqual(cache.data(for: "https://example.com/1"), Data("v1-updated".utf8))
        XCTAssertEqual(cache.etag(for: "https://example.com/2"), "\"e2\"",
                       "Updating an existing entry at capacity should not evict other entries")
    }

    func testUpdateMovesToEndOfInsertionOrder() {
        let cache = ETagCache(maxEntries: 2)
        cache.store(url: "https://example.com/1", data: Data("v1".utf8), etag: "\"e1\"")
        cache.store(url: "https://example.com/2", data: Data("v2".utf8), etag: "\"e2\"")

        // Update /1 — moves it to end of insertion order
        cache.store(url: "https://example.com/1", data: Data("v1-new".utf8), etag: "\"e1-new\"")

        // New entry should evict /2 (now oldest), not /1
        cache.store(url: "https://example.com/3", data: Data("v3".utf8), etag: "\"e3\"")

        XCTAssertNotNil(cache.etag(for: "https://example.com/1"), "/1 was updated and should be newest")
        XCTAssertNil(cache.etag(for: "https://example.com/2"), "/2 should be evicted as oldest")
        XCTAssertNotNil(cache.etag(for: "https://example.com/3"))
    }

    // MARK: - removeAll

    func testRemoveAll() {
        let cache = ETagCache(maxEntries: 10)
        cache.store(url: "https://example.com/a", data: Data("a".utf8), etag: "\"a\"")
        cache.store(url: "https://example.com/b", data: Data("b".utf8), etag: "\"b\"")

        cache.removeAll()

        XCTAssertNil(cache.etag(for: "https://example.com/a"))
        XCTAssertNil(cache.etag(for: "https://example.com/b"))
    }

    func testStoreAfterRemoveAll() {
        let cache = ETagCache(maxEntries: 2)
        cache.store(url: "https://example.com/a", data: Data("a".utf8), etag: "\"a\"")
        cache.store(url: "https://example.com/b", data: Data("b".utf8), etag: "\"b\"")

        cache.removeAll()

        // Should be able to store fresh entries after clear
        cache.store(url: "https://example.com/c", data: Data("c".utf8), etag: "\"c\"")
        XCTAssertEqual(cache.etag(for: "https://example.com/c"), "\"c\"")
    }

    // MARK: - Concurrent Access

    func testConcurrentAccessViaTasks() async {
        let cache = ETagCache(maxEntries: 100)

        // Perform concurrent reads and writes
        await withTaskGroup(of: Void.self) { group in
            for i in 0..<50 {
                group.addTask {
                    cache.store(
                        url: "https://example.com/\(i)",
                        data: Data("data-\(i)".utf8),
                        etag: "\"etag-\(i)\""
                    )
                }
                group.addTask {
                    // Concurrent reads (may or may not find the entry yet)
                    _ = cache.etag(for: "https://example.com/\(i)")
                    _ = cache.data(for: "https://example.com/\(i)")
                }
            }
        }

        // After all tasks complete, verify no crashes and at least some entries exist
        var found = 0
        for i in 0..<50 {
            if cache.etag(for: "https://example.com/\(i)") != nil {
                found += 1
            }
        }
        XCTAssertEqual(found, 50, "All 50 entries should be present after concurrent writes")
    }

    func testConcurrentStoreAndRemoveAll() async {
        let cache = ETagCache(maxEntries: 100)

        // Race stores against removeAll — should not crash
        await withTaskGroup(of: Void.self) { group in
            for i in 0..<30 {
                group.addTask {
                    cache.store(
                        url: "https://example.com/\(i)",
                        data: Data("data".utf8),
                        etag: "\"e\(i)\""
                    )
                }
            }
            group.addTask {
                cache.removeAll()
            }
        }

        // Just verify it didn't crash — state after race is indeterminate
    }
}
