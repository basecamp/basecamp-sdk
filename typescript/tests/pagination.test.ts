/**
 * Tests for pagination types and utilities
 */
import { describe, it, expect } from "vitest";
import { ListResult, parseTotalCount } from "../src/pagination.js";

describe("ListResult", () => {
  it("should behave as an array (length, indexing, forEach, map, spread)", () => {
    const items = [{ id: 1 }, { id: 2 }, { id: 3 }];
    const result = new ListResult(items, { totalCount: 100 });

    // Length
    expect(result.length).toBe(3);

    // Indexing
    expect(result[0]).toEqual({ id: 1 });
    expect(result[2]).toEqual({ id: 3 });

    // forEach
    const collected: number[] = [];
    result.forEach((item) => collected.push(item.id));
    expect(collected).toEqual([1, 2, 3]);

    // map
    const ids = result.map((item) => item.id);
    expect(ids).toEqual([1, 2, 3]);

    // spread
    const spread = [...result];
    expect(spread).toEqual(items);
  });

  it("should report Array.isArray as true", () => {
    const result = new ListResult([], { totalCount: 0 });
    expect(Array.isArray(result)).toBe(true);
  });

  it("should expose meta.totalCount", () => {
    const result = new ListResult([1, 2, 3], { totalCount: 150 });
    expect(result.meta.totalCount).toBe(150);
  });

  it("should work with empty arrays", () => {
    const result = new ListResult([], { totalCount: 0 });
    expect(result.length).toBe(0);
    expect(result.meta.totalCount).toBe(0);
  });
});

describe("parseTotalCount", () => {
  it("should extract X-Total-Count header", () => {
    const response = new Response(null, {
      headers: { "X-Total-Count": "42" },
    });
    expect(parseTotalCount(response)).toBe(42);
  });

  it("should return 0 for missing header", () => {
    const response = new Response(null);
    expect(parseTotalCount(response)).toBe(0);
  });

  it("should return 0 for invalid header", () => {
    const response = new Response(null, {
      headers: { "X-Total-Count": "not-a-number" },
    });
    expect(parseTotalCount(response)).toBe(0);
  });
});
