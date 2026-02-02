/**
 * Tests for the hooks module
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  chainHooks,
  consoleHooks,
  noopHooks,
  safeInvoke,
  type BasecampHooks,
  type OperationInfo,
  type RequestInfo,
  type RequestResult,
  type OperationResult,
} from "../src/hooks.js";

describe("chainHooks", () => {
  it("should return empty object for no hooks", () => {
    const result = chainHooks();
    expect(result).toEqual({});
  });

  it("should return the hook directly for single hook", () => {
    const hook: BasecampHooks = {
      onOperationStart: vi.fn(),
    };
    const result = chainHooks(hook);
    expect(result).toBe(hook);
  });

  it("should call all hooks in order for onOperationStart", () => {
    const calls: string[] = [];
    const hook1: BasecampHooks = {
      onOperationStart: () => calls.push("hook1"),
    };
    const hook2: BasecampHooks = {
      onOperationStart: () => calls.push("hook2"),
    };

    const chained = chainHooks(hook1, hook2);
    const info: OperationInfo = {
      service: "Todos",
      operation: "List",
      resourceType: "todo",
      isMutation: false,
    };

    chained.onOperationStart?.(info);

    expect(calls).toEqual(["hook1", "hook2"]);
  });

  it("should call all hooks for onOperationEnd", () => {
    const hook1 = { onOperationEnd: vi.fn() };
    const hook2 = { onOperationEnd: vi.fn() };

    const chained = chainHooks(hook1, hook2);
    const info: OperationInfo = {
      service: "Todos",
      operation: "Get",
      resourceType: "todo",
      isMutation: false,
    };
    const result: OperationResult = { durationMs: 100 };

    chained.onOperationEnd?.(info, result);

    expect(hook1.onOperationEnd).toHaveBeenCalledWith(info, result);
    expect(hook2.onOperationEnd).toHaveBeenCalledWith(info, result);
  });

  it("should call all hooks for onRequestStart", () => {
    const hook1 = { onRequestStart: vi.fn() };
    const hook2 = { onRequestStart: vi.fn() };

    const chained = chainHooks(hook1, hook2);
    const info: RequestInfo = {
      method: "GET",
      url: "https://api.example.com/todos",
      attempt: 1,
    };

    chained.onRequestStart?.(info);

    expect(hook1.onRequestStart).toHaveBeenCalledWith(info);
    expect(hook2.onRequestStart).toHaveBeenCalledWith(info);
  });

  it("should call all hooks for onRequestEnd", () => {
    const hook1 = { onRequestEnd: vi.fn() };
    const hook2 = { onRequestEnd: vi.fn() };

    const chained = chainHooks(hook1, hook2);
    const info: RequestInfo = {
      method: "GET",
      url: "https://api.example.com/todos",
      attempt: 1,
    };
    const result: RequestResult = {
      statusCode: 200,
      durationMs: 50,
      fromCache: false,
    };

    chained.onRequestEnd?.(info, result);

    expect(hook1.onRequestEnd).toHaveBeenCalledWith(info, result);
    expect(hook2.onRequestEnd).toHaveBeenCalledWith(info, result);
  });

  it("should call all hooks for onRetry", () => {
    const hook1 = { onRetry: vi.fn() };
    const hook2 = { onRetry: vi.fn() };

    const chained = chainHooks(hook1, hook2);
    const info: RequestInfo = {
      method: "GET",
      url: "https://api.example.com/todos",
      attempt: 2,
    };
    const error = new Error("Rate limited");

    chained.onRetry?.(info, 2, error, 1000);

    expect(hook1.onRetry).toHaveBeenCalledWith(info, 2, error, 1000);
    expect(hook2.onRetry).toHaveBeenCalledWith(info, 2, error, 1000);
  });

  it("should catch and log errors from hooks", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const hook1: BasecampHooks = {
      onOperationStart: () => {
        throw new Error("Hook error");
      },
    };
    const hook2 = { onOperationStart: vi.fn() };

    const chained = chainHooks(hook1, hook2);
    const info: OperationInfo = {
      service: "Todos",
      operation: "List",
      resourceType: "todo",
      isMutation: false,
    };

    // Should not throw
    chained.onOperationStart?.(info);

    // Hook2 should still be called
    expect(hook2.onOperationStart).toHaveBeenCalled();

    // Error should be logged
    expect(consoleSpy).toHaveBeenCalledWith(
      "Hook onOperationStart error:",
      expect.any(Error)
    );

    consoleSpy.mockRestore();
  });

  it("should filter out undefined hooks", () => {
    const hook = { onOperationStart: vi.fn() };
    const chained = chainHooks(undefined as any, hook, null as any);

    const info: OperationInfo = {
      service: "Todos",
      operation: "List",
      resourceType: "todo",
      isMutation: false,
    };

    chained.onOperationStart?.(info);
    expect(hook.onOperationStart).toHaveBeenCalled();
  });
});

describe("consoleHooks", () => {
  let mockLogger: { log: ReturnType<typeof vi.fn>; warn: ReturnType<typeof vi.fn>; error: ReturnType<typeof vi.fn> };

  beforeEach(() => {
    mockLogger = {
      log: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
    };
  });

  describe("onOperationStart", () => {
    it("should log operation start by default", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "List",
        resourceType: "todo",
        isMutation: false,
      };

      hooks.onOperationStart?.(info);

      expect(mockLogger.log).toHaveBeenCalledWith("[Basecamp] Todos.List");
    });

    it("should include mutation marker", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "Create",
        resourceType: "todo",
        isMutation: true,
      };

      hooks.onOperationStart?.(info);

      expect(mockLogger.log).toHaveBeenCalledWith(
        expect.stringContaining("[mutation]")
      );
    });

    it("should include resource ID", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
        resourceId: 456,
      };

      hooks.onOperationStart?.(info);

      expect(mockLogger.log).toHaveBeenCalledWith(
        "[Basecamp] Todos.Get #456"
      );
    });

    it("should not log when logOperations is false", () => {
      const hooks = consoleHooks({ logOperations: false, logger: mockLogger });

      expect(hooks.onOperationStart).toBeUndefined();
    });
  });

  describe("onOperationEnd", () => {
    it("should log operation completion", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
      };
      const result: OperationResult = { durationMs: 150 };

      hooks.onOperationEnd?.(info, result);

      expect(mockLogger.log).toHaveBeenCalledWith(
        "[Basecamp] Todos.Get completed (150ms)"
      );
    });

    it("should log errors", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
      };
      const result: OperationResult = {
        durationMs: 100,
        error: new Error("Not found"),
      };

      hooks.onOperationEnd?.(info, result);

      expect(mockLogger.error).toHaveBeenCalledWith(
        "[Basecamp] Todos.Get failed (100ms):",
        "Not found"
      );
    });

    it("should respect minDurationMs filter", () => {
      const hooks = consoleHooks({ minDurationMs: 100, logger: mockLogger });
      const info: OperationInfo = {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
      };

      hooks.onOperationEnd?.(info, { durationMs: 50 });
      expect(mockLogger.log).not.toHaveBeenCalled();

      hooks.onOperationEnd?.(info, { durationMs: 150 });
      expect(mockLogger.log).toHaveBeenCalled();
    });
  });

  describe("onRequestStart", () => {
    it("should not log by default", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      expect(hooks.onRequestStart).toBeUndefined();
    });

    it("should log when logRequests is true", () => {
      const hooks = consoleHooks({ logRequests: true, logger: mockLogger });
      const info: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos.json",
        attempt: 1,
      };

      hooks.onRequestStart?.(info);

      expect(mockLogger.log).toHaveBeenCalledWith(
        "[Basecamp] -> GET https://api.example.com/todos.json"
      );
    });

    it("should include retry attempt number", () => {
      const hooks = consoleHooks({ logRequests: true, logger: mockLogger });
      const info: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos.json",
        attempt: 2,
      };

      hooks.onRequestStart?.(info);

      expect(mockLogger.log).toHaveBeenCalledWith(
        expect.stringContaining("(attempt 2)")
      );
    });
  });

  describe("onRequestEnd", () => {
    it("should log when logRequests is true", () => {
      const hooks = consoleHooks({ logRequests: true, logger: mockLogger });
      const info: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos.json",
        attempt: 1,
      };
      const result: RequestResult = {
        statusCode: 200,
        durationMs: 75,
        fromCache: false,
      };

      hooks.onRequestEnd?.(info, result);

      expect(mockLogger.log).toHaveBeenCalledWith(
        "[Basecamp] <- GET https://api.example.com/todos.json 200 (75ms)"
      );
    });

    it("should indicate cached responses", () => {
      const hooks = consoleHooks({ logRequests: true, logger: mockLogger });
      const info: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos.json",
        attempt: 1,
      };
      const result: RequestResult = {
        statusCode: 200,
        durationMs: 5,
        fromCache: true,
      };

      hooks.onRequestEnd?.(info, result);

      expect(mockLogger.log).toHaveBeenCalledWith(
        expect.stringContaining("(cached)")
      );
    });
  });

  describe("onRetry", () => {
    it("should log retries by default", () => {
      const hooks = consoleHooks({ logger: mockLogger });
      const info: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos.json",
        attempt: 1,
      };
      const error = new Error("Rate limited");

      hooks.onRetry?.(info, 2, error, 2000);

      expect(mockLogger.warn).toHaveBeenCalledWith(
        "[Basecamp] Retrying GET https://api.example.com/todos.json (attempt 3, waiting 2000ms): Rate limited"
      );
    });

    it("should not log when logRetries is false", () => {
      const hooks = consoleHooks({ logRetries: false, logger: mockLogger });
      expect(hooks.onRetry).toBeUndefined();
    });
  });
});

describe("noopHooks", () => {
  it("should return an empty hooks object", () => {
    const hooks = noopHooks();
    expect(hooks).toEqual({});
  });
});

describe("safeInvoke", () => {
  it("should invoke hook when present", () => {
    const hook = { onOperationStart: vi.fn() };
    const info: OperationInfo = {
      service: "Todos",
      operation: "List",
      resourceType: "todo",
      isMutation: false,
    };

    safeInvoke(hook, "onOperationStart", info);

    expect(hook.onOperationStart).toHaveBeenCalledWith(info);
  });

  it("should do nothing when hooks is undefined", () => {
    // Should not throw
    safeInvoke(undefined, "onOperationStart", {} as OperationInfo);
  });

  it("should do nothing when specific hook is not defined", () => {
    const hooks: BasecampHooks = {};
    // Should not throw
    safeInvoke(hooks, "onOperationStart", {} as OperationInfo);
  });

  it("should catch and log errors", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const hooks: BasecampHooks = {
      onOperationStart: () => {
        throw new Error("Hook failed");
      },
    };

    // Should not throw
    safeInvoke(hooks, "onOperationStart", {} as OperationInfo);

    expect(consoleSpy).toHaveBeenCalledWith(
      "Hook onOperationStart error:",
      expect.any(Error)
    );

    consoleSpy.mockRestore();
  });
});
