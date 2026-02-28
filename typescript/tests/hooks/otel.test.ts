/**
 * Tests for the OpenTelemetry hooks
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { otelHooks, type OtelHooksOptions } from "../../src/hooks/otel.js";
import type { OperationInfo, OperationResult, RequestInfo, RequestResult } from "../../src/hooks.js";

// Mock OpenTelemetry types for testing
interface MockSpan {
  setAttribute: ReturnType<typeof vi.fn>;
  setStatus: ReturnType<typeof vi.fn>;
  recordException: ReturnType<typeof vi.fn>;
  end: ReturnType<typeof vi.fn>;
}

interface MockTracer {
  startSpan: ReturnType<typeof vi.fn>;
}

interface MockHistogram {
  record: ReturnType<typeof vi.fn>;
}

interface MockCounter {
  add: ReturnType<typeof vi.fn>;
}

interface MockMeter {
  createHistogram: ReturnType<typeof vi.fn>;
  createCounter: ReturnType<typeof vi.fn>;
}

function createMockSpan(): MockSpan {
  return {
    setAttribute: vi.fn().mockReturnThis(),
    setStatus: vi.fn().mockReturnThis(),
    recordException: vi.fn().mockReturnThis(),
    end: vi.fn(),
  };
}

function createMockTracer(): MockTracer {
  return {
    startSpan: vi.fn().mockReturnValue(createMockSpan()),
  };
}

function createMockMeter(): MockMeter {
  const mockHistogram: MockHistogram = { record: vi.fn() };
  const mockCounter: MockCounter = { add: vi.fn() };

  return {
    createHistogram: vi.fn().mockReturnValue(mockHistogram),
    createCounter: vi.fn().mockReturnValue(mockCounter),
  };
}

describe("otelHooks", () => {
  describe("with no options", () => {
    it("should create hooks without tracer or meter", () => {
      const hooks = otelHooks();
      expect(hooks).toBeDefined();
      expect(hooks.onOperationStart).toBeDefined();
      expect(hooks.onOperationEnd).toBeDefined();
      expect(hooks.onRequestStart).toBeDefined();
      expect(hooks.onRequestEnd).toBeDefined();
      expect(hooks.onRetry).toBeDefined();
    });

    it("should not throw when called without tracer/meter", () => {
      const hooks = otelHooks();
      const operationInfo: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };
      const operationResult: OperationResult = { durationMs: 100 };

      expect(() => hooks.onOperationStart?.(operationInfo)).not.toThrow();
      expect(() => hooks.onOperationEnd?.(operationInfo, operationResult)).not.toThrow();
    });
  });

  describe("with tracer", () => {
    let tracer: MockTracer;
    let hooks: ReturnType<typeof otelHooks>;

    beforeEach(() => {
      tracer = createMockTracer();
      hooks = otelHooks({ tracer: tracer as unknown as OtelHooksOptions["tracer"] });
    });

    it("should start a span on operation start", () => {
      const operationInfo: OperationInfo = {
        service: "Todos",
        operation: "List",
        resourceType: "todo",
        isMutation: false,
        };

      hooks.onOperationStart?.(operationInfo);

      expect(tracer.startSpan).toHaveBeenCalledWith(
        "basecamp.Todos.List",
        expect.objectContaining({
          attributes: expect.objectContaining({
            "basecamp.service": "Todos",
            "basecamp.operation": "List",
            "basecamp.resource_type": "todo",
            "basecamp.is_mutation": false,
          }),
        })
      );
    });

    it("should end span with OK status on successful operation", () => {
      const span = createMockSpan();
      tracer.startSpan.mockReturnValue(span);

      const operationInfo: OperationInfo = {
        service: "Todos",
        operation: "Create",
        resourceType: "todo",
        isMutation: true,
      };
      const operationResult: OperationResult = { durationMs: 50 };

      hooks.onOperationStart?.(operationInfo);
      hooks.onOperationEnd?.(operationInfo, operationResult);

      expect(span.setStatus).toHaveBeenCalledWith({ code: 1 }); // OK
      expect(span.setAttribute).toHaveBeenCalledWith("duration_ms", 50);
      expect(span.end).toHaveBeenCalled();
    });

    it("should end span with ERROR status on failed operation", () => {
      const span = createMockSpan();
      tracer.startSpan.mockReturnValue(span);

      const operationInfo: OperationInfo = {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
      };
      const error = new Error("Not found");
      const operationResult: OperationResult = { durationMs: 25, error };

      hooks.onOperationStart?.(operationInfo);
      hooks.onOperationEnd?.(operationInfo, operationResult);

      expect(span.setStatus).toHaveBeenCalledWith({
        code: 2, // ERROR
        message: "Not found",
      });
      expect(span.recordException).toHaveBeenCalledWith(error);
      expect(span.setAttribute).toHaveBeenCalledWith("error", true);
      expect(span.setAttribute).toHaveBeenCalledWith("error.message", "Not found");
      expect(span.end).toHaveBeenCalled();
    });

    it("should correctly correlate spans for concurrent same-type operations", () => {
      const span1 = createMockSpan();
      const span2 = createMockSpan();
      tracer.startSpan.mockReturnValueOnce(span1).mockReturnValueOnce(span2);

      // Same OperationInfo shape, but different object references (as happens with concurrent calls)
      const info1: OperationInfo = {
        service: "Todos",
        operation: "List",
        resourceType: "todo",
        isMutation: false,
      };
      const info2: OperationInfo = {
        service: "Todos",
        operation: "List",
        resourceType: "todo",
        isMutation: false,
      };

      // Start both
      hooks.onOperationStart?.(info1);
      hooks.onOperationStart?.(info2);

      // End the second one first
      hooks.onOperationEnd?.(info2, { durationMs: 50 });

      // span2 should be ended (not span1)
      expect(span2.end).toHaveBeenCalled();
      expect(span1.end).not.toHaveBeenCalled();

      // Now end the first
      hooks.onOperationEnd?.(info1, { durationMs: 100 });
      expect(span1.end).toHaveBeenCalled();
    });

    it("should not create request spans by default", () => {
      const requestInfo: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/test",
        attempt: 1,
      };

      hooks.onRequestStart?.(requestInfo);

      // Should not have been called for request span
      expect(tracer.startSpan).not.toHaveBeenCalledWith(
        expect.stringContaining("basecamp.http"),
        expect.anything()
      );
    });
  });

  describe("with recordRequestSpans enabled", () => {
    let tracer: MockTracer;
    let hooks: ReturnType<typeof otelHooks>;

    beforeEach(() => {
      tracer = createMockTracer();
      hooks = otelHooks({
        tracer: tracer as unknown as OtelHooksOptions["tracer"],
        recordRequestSpans: true,
      });
    });

    it("should create request spans when enabled", () => {
      const requestInfo: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/test",
        attempt: 1,
      };

      hooks.onRequestStart?.(requestInfo);

      expect(tracer.startSpan).toHaveBeenCalledWith(
        "basecamp.http.GET",
        expect.objectContaining({
          attributes: {
            "http.method": "GET",
            "http.url": "https://api.example.com/test",
            "http.attempt": 1,
          },
        })
      );
    });

    it("should correctly correlate spans for concurrent same-type requests", () => {
      const span1 = createMockSpan();
      const span2 = createMockSpan();
      tracer.startSpan.mockReturnValueOnce(span1).mockReturnValueOnce(span2);

      // Two concurrent requests with the same method/url/attempt (different object refs)
      const startInfo1: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos",
        attempt: 1,
      };
      const startInfo2: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos",
        attempt: 1,
      };

      // Start both
      hooks.onRequestStart?.(startInfo1);
      hooks.onRequestStart?.(startInfo2);

      // End the second one first (LIFO: pops span2)
      const endInfo2: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos",
        attempt: 1,
      };
      hooks.onRequestEnd?.(endInfo2, { statusCode: 200, durationMs: 50, fromCache: false });

      expect(span2.end).toHaveBeenCalled();
      expect(span1.end).not.toHaveBeenCalled();

      // End the first one (LIFO: pops span1)
      const endInfo1: RequestInfo = {
        method: "GET",
        url: "https://api.example.com/todos",
        attempt: 1,
      };
      hooks.onRequestEnd?.(endInfo1, { statusCode: 200, durationMs: 100, fromCache: false });

      expect(span1.end).toHaveBeenCalled();
    });

    it("should end request span with status code", () => {
      const span = createMockSpan();
      tracer.startSpan.mockReturnValue(span);

      const requestInfo: RequestInfo = {
        method: "POST",
        url: "https://api.example.com/test",
        attempt: 1,
      };
      const requestResult: RequestResult = {
        statusCode: 201,
        durationMs: 75,
        fromCache: false,
      };

      hooks.onRequestStart?.(requestInfo);
      hooks.onRequestEnd?.(requestInfo, requestResult);

      expect(span.setAttribute).toHaveBeenCalledWith("http.status_code", 201);
      expect(span.setAttribute).toHaveBeenCalledWith("http.from_cache", false);
      expect(span.setStatus).toHaveBeenCalledWith({ code: 1 }); // OK
      expect(span.end).toHaveBeenCalled();
    });
  });

  describe("with meter", () => {
    let meter: MockMeter;
    let hooks: ReturnType<typeof otelHooks>;

    beforeEach(() => {
      meter = createMockMeter();
      hooks = otelHooks({ meter: meter as unknown as OtelHooksOptions["meter"] });
    });

    it("should create histograms and counters on initialization", () => {
      expect(meter.createHistogram).toHaveBeenCalledWith(
        "basecamp.operation.duration",
        expect.objectContaining({ unit: "ms" })
      );
      expect(meter.createHistogram).toHaveBeenCalledWith(
        "basecamp.request.duration",
        expect.objectContaining({ unit: "ms" })
      );
      expect(meter.createCounter).toHaveBeenCalledWith(
        "basecamp.operations.total",
        expect.anything()
      );
      expect(meter.createCounter).toHaveBeenCalledWith(
        "basecamp.errors.total",
        expect.anything()
      );
      expect(meter.createCounter).toHaveBeenCalledWith(
        "basecamp.retries.total",
        expect.anything()
      );
    });

    it("should record operation count on start", () => {
      // The operation counter is created at hooks initialization
      // We need to verify the counter was called
      const mockCounter: MockCounter = { add: vi.fn() };
      const freshMeter = createMockMeter();
      freshMeter.createCounter.mockReturnValue(mockCounter);

      // Create hooks with the meter that returns our mock counter
      const freshHooks = otelHooks({ meter: freshMeter as unknown as OtelHooksOptions["meter"] });

      const operationInfo: OperationInfo = {
        service: "Projects",
        operation: "List",
        resourceType: "project",
        isMutation: false,
      };

      freshHooks.onOperationStart?.(operationInfo);

      expect(mockCounter.add).toHaveBeenCalledWith(1, {
        service: "Projects",
        operation: "List",
        is_mutation: false,
      });
    });

    it("should record operation duration on end", () => {
      const mockHistogram: MockHistogram = { record: vi.fn() };
      meter.createHistogram.mockReturnValue(mockHistogram);

      hooks = otelHooks({ meter: meter as unknown as OtelHooksOptions["meter"] });

      const operationInfo: OperationInfo = {
        service: "Projects",
        operation: "Get",
        resourceType: "project",
        isMutation: false,
      };
      const operationResult: OperationResult = { durationMs: 150 };

      hooks.onOperationEnd?.(operationInfo, operationResult);

      expect(mockHistogram.record).toHaveBeenCalledWith(150, {
        service: "Projects",
        operation: "Get",
        is_mutation: false,
        success: true,
      });
    });

    it("should record error count on failed operation", () => {
      const mockCounter: MockCounter = { add: vi.fn() };
      meter.createCounter.mockReturnValue(mockCounter);

      hooks = otelHooks({ meter: meter as unknown as OtelHooksOptions["meter"] });

      const operationInfo: OperationInfo = {
        service: "Projects",
        operation: "Create",
        resourceType: "project",
        isMutation: true,
      };
      const error = new Error("Validation failed");
      const operationResult: OperationResult = { durationMs: 30, error };

      hooks.onOperationEnd?.(operationInfo, operationResult);

      expect(mockCounter.add).toHaveBeenCalledWith(1, {
        service: "Projects",
        operation: "Create",
        error_type: "Error",
      });
    });

    it("should record retry count", () => {
      const mockCounter: MockCounter = { add: vi.fn() };
      meter.createCounter.mockReturnValue(mockCounter);

      hooks = otelHooks({ meter: meter as unknown as OtelHooksOptions["meter"] });

      const requestInfo: RequestInfo = {
        method: "POST",
        url: "https://api.example.com/test",
        attempt: 2,
      };
      const error = new Error("Rate limited");

      hooks.onRetry?.(requestInfo, 2, error, 1000);

      expect(mockCounter.add).toHaveBeenCalledWith(1, {
        method: "POST",
        attempt: 2,
        error_type: "Error",
      });
    });
  });

  describe("custom prefixes", () => {
    it("should use custom span prefix", () => {
      const tracer = createMockTracer();
      const hooks = otelHooks({
        tracer: tracer as unknown as OtelHooksOptions["tracer"],
        spanPrefix: "custom",
      });

      const operationInfo: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      hooks.onOperationStart?.(operationInfo);

      expect(tracer.startSpan).toHaveBeenCalledWith(
        "custom.Test.Get",
        expect.objectContaining({
          attributes: expect.objectContaining({
            "custom.service": "Test",
          }),
        })
      );
    });

    it("should use custom metric prefix", () => {
      const meter = createMockMeter();
      otelHooks({
        meter: meter as unknown as OtelHooksOptions["meter"],
        metricPrefix: "myapp",
      });

      expect(meter.createHistogram).toHaveBeenCalledWith(
        "myapp.operation.duration",
        expect.anything()
      );
      expect(meter.createCounter).toHaveBeenCalledWith(
        "myapp.operations.total",
        expect.anything()
      );
    });
  });
});
