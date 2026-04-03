package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertTrue

class BasecampExceptionBinaryCompatibilityTest {

    @Test
    fun notFoundRetainsLegacyConstructors() {
        val constructors = BasecampException.NotFound::class.java.declaredConstructors

        val hasLegacyFourArgumentConstructor = constructors.any { constructor ->
            val parameterTypes = constructor.parameterTypes
            parameterTypes.size == 4
                && parameterTypes[0] == String::class.java
                && parameterTypes[1] == String::class.java
                && parameterTypes[2] == String::class.java
                && parameterTypes[3] == Throwable::class.java
        }
        val hasLegacyZeroArgumentConstructor = constructors.any { it.parameterTypes.isEmpty() }

        assertTrue(
            hasLegacyFourArgumentConstructor,
            "Expected legacy NotFound(String, String?, String?, Throwable?) constructor for binary compatibility",
        )
        assertTrue(
            hasLegacyZeroArgumentConstructor,
            "Expected legacy zero-argument NotFound() constructor for binary compatibility",
        )
    }
}
