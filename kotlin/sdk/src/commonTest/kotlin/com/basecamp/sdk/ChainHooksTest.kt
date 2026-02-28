package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.time.Duration.Companion.milliseconds

class ChainHooksTest {

    private val info = OperationInfo(
        service = "Projects",
        operation = "ListProjects",
        resourceType = "project",
        isMutation = false,
    )

    private val requestInfo = RequestInfo(method = "GET", url = "https://example.com", attempt = 1)

    @Test
    fun startEventsFireInForwardOrder() {
        val order = mutableListOf<String>()

        val hook1 = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("hook1")
            }
        }
        val hook2 = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("hook2")
            }
        }
        val hook3 = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("hook3")
            }
        }

        val chain = chainHooks(hook1, hook2, hook3)
        chain.onOperationStart(info)

        assertEquals(listOf("hook1", "hook2", "hook3"), order)
    }

    @Test
    fun endEventsFireInReverseOrder() {
        val order = mutableListOf<String>()
        val result = OperationResult(duration = 100.milliseconds)

        val hook1 = object : BasecampHooks {
            override fun onOperationEnd(info: OperationInfo, result: OperationResult) {
                order.add("hook1")
            }
        }
        val hook2 = object : BasecampHooks {
            override fun onOperationEnd(info: OperationInfo, result: OperationResult) {
                order.add("hook2")
            }
        }
        val hook3 = object : BasecampHooks {
            override fun onOperationEnd(info: OperationInfo, result: OperationResult) {
                order.add("hook3")
            }
        }

        val chain = chainHooks(hook1, hook2, hook3)
        chain.onOperationEnd(info, result)

        assertEquals(listOf("hook3", "hook2", "hook1"), order)
    }

    @Test
    fun requestStartForwardRequestEndReverse() {
        val startOrder = mutableListOf<String>()
        val endOrder = mutableListOf<String>()
        val requestResult = RequestResult(statusCode = 200, duration = 50.milliseconds)

        val hook1 = object : BasecampHooks {
            override fun onRequestStart(info: RequestInfo) {
                startOrder.add("hook1")
            }
            override fun onRequestEnd(info: RequestInfo, result: RequestResult) {
                endOrder.add("hook1")
            }
        }
        val hook2 = object : BasecampHooks {
            override fun onRequestStart(info: RequestInfo) {
                startOrder.add("hook2")
            }
            override fun onRequestEnd(info: RequestInfo, result: RequestResult) {
                endOrder.add("hook2")
            }
        }

        val chain = chainHooks(hook1, hook2)
        chain.onRequestStart(requestInfo)
        chain.onRequestEnd(requestInfo, requestResult)

        assertEquals(listOf("hook1", "hook2"), startOrder)
        assertEquals(listOf("hook2", "hook1"), endOrder)
    }

    @Test
    fun emptyChainReturnsNoopHooks() {
        val chain = chainHooks()
        assertEquals(NoopHooks, chain)
        // Should not throw
        chain.onOperationStart(info)
        chain.onOperationEnd(info, OperationResult(duration = 0.milliseconds))
    }

    @Test
    fun singleHookReturnedDirectly() {
        val hook = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {}
        }
        val chain = chainHooks(hook)
        assertEquals(hook, chain, "Single hook should be returned directly, not wrapped")
    }

    @Test
    fun noopHooksAreFiltered() {
        val order = mutableListOf<String>()
        val hook1 = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("hook1")
            }
        }

        val chain = chainHooks(NoopHooks, hook1, NoopHooks)
        // With only one active hook, chainHooks returns it directly
        assertEquals(hook1, chain)
    }

    @Test
    fun retryHookFiresInForwardOrder() {
        val order = mutableListOf<String>()
        val error = RuntimeException("test")

        val hook1 = object : BasecampHooks {
            override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                order.add("hook1")
            }
        }
        val hook2 = object : BasecampHooks {
            override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                order.add("hook2")
            }
        }

        val chain = chainHooks(hook1, hook2)
        chain.onRetry(requestInfo, 2, error, 1000L)

        assertEquals(listOf("hook1", "hook2"), order)
    }

    @Test
    fun hookExceptionsDoNotPropagate() {
        val order = mutableListOf<String>()

        val throwingHook = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("throwing")
                throw RuntimeException("hook failure")
            }
        }
        val goodHook = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                order.add("good")
            }
        }

        val chain = chainHooks(throwingHook, goodHook)
        // Should not throw even though first hook throws
        chain.onOperationStart(info)

        assertEquals(listOf("throwing", "good"), order)
    }

    @Test
    fun hookReceivesCorrectOperationInfo() {
        var capturedInfo: OperationInfo? = null

        val hook = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                capturedInfo = info
            }
        }

        val testInfo = OperationInfo(
            service = "Todos",
            operation = "CompleteTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = 42,
            resourceId = 100,
        )

        val chain = chainHooks(hook)
        chain.onOperationStart(testInfo)

        val captured = capturedInfo!!
        assertEquals("Todos", captured.service)
        assertEquals("CompleteTodo", captured.operation)
        assertEquals(true, captured.isMutation)
        assertEquals(42L, captured.projectId)
        assertEquals(100L, captured.resourceId)
    }
}
