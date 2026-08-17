package com.basecamp.sdk

import kotlinx.serialization.SerializationException
import java.lang.reflect.Constructor
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The malformed-body marker on [BasecampException.Api] must not be settable
 * from Java **by accident** (#751 review).
 *
 * `decodeFailure` answers "did this `api_error` come out of the response
 * decoder?", and the §18 composites re-hint off it. An auth strategy that fills
 * it too puts back the #730 bug: an authentication failure relabelled as a
 * malformed Basecamp response for a request that was never sent.
 *
 * **The claim is bounded, deliberately.** What is prevented here is a Java
 * caller picking the convenient overload without meaning anything by it. Java
 * source can still call the mangled factory (`malformedBody$…`) on purpose —
 * `$` is a legal identifier character — and that is left open: the same
 * `AuthStrategy` can already throw, through the public constructor, an `Api`
 * carrying the composite's own message and hint verbatim, so the deliberate
 * path has an equivalent one line away and closing it would buy nothing. These
 * tests are named for what they hold: no Java-SELECTABLE constructor takes a
 * decoder exception, and the factory is reachable only under a MANGLED name.
 *
 * `internal` alone does not carry that guarantee across the JVM boundary, and
 * the distinction is exactly the one this test pins:
 *
 * - Internal **constructors** are emitted `public`, because `<init>` cannot be
 *   name-mangled. An `internal constructor(message: String, decodeFailure:
 *   SerializationException)` — the shape this SDK shipped first — is a public
 *   two-arg constructor to Java. Worse, it is the *shortest* one on offer:
 *   Kotlin default arguments do not exist for Java callers, so without
 *   `@JvmOverloads` Java sees only the six-arg `Api(String, Integer, String,
 *   boolean, String, Throwable)` beside it. A Java `AuthStrategy` classifying
 *   its own token-response decode failure would naturally write
 *   `new Api(message, decodeError)` and set the marker by accident.
 * - Internal **functions** ARE name-mangled (`malformedBody$…`), so a factory
 *   is not selectable by accident from Java.
 *
 * This is the accident/regression class, not deliberate evasion: nobody has to
 * be working around anything for a Java-authored `AuthStrategy` on the JVM
 * target to pick the convenient overload. Reflection is the closest the
 * toolchain gets to a Java caller here — `commonTest` cannot host Java source,
 * and this SDK has no Java sample module — so what is asserted is the emitted
 * constructor surface javac would choose from, not a compiled Java call site.
 * The compiler's own view of that surface is the same list.
 */
class ApiConstructorSurfaceTest {

    /**
     * Constructors javac can actually select: public and not synthetic.
     *
     * Synthetic members are excluded because javac refuses to resolve them from
     * source. Kotlin emits two here — the default-arguments bridge and the
     * accessor for the private marker constructor — and both also carry a
     * trailing `DefaultConstructorMarker` no Java call site would write.
     */
    private val javaSelectableConstructors: List<Constructor<*>>
        get() = BasecampException.Api::class.java.constructors.filterNot { it.isSynthetic }

    @Test
    fun noJavaSelectableConstructorTakesADecoderException() {
        val offenders = javaSelectableConstructors.filter { constructor ->
            constructor.parameterTypes.any { SerializationException::class.java.isAssignableFrom(it) }
        }

        assertTrue(
            offenders.isEmpty(),
            "the malformed-body marker must be unreachable from Java, but these constructors " +
                "accept a SerializationException directly: " +
                offenders.joinToString { it.parameterTypes.joinToString(prefix = "(", postfix = ")") { p -> p.simpleName } },
        )
    }

    /**
     * The control for the assertion above: it must be passing because the
     * marker constructor is gone, not because reflection found nothing to
     * inspect. The public constructor's shape is also the thing #751 promised
     * to keep byte-identical, so a change to it should be deliberate.
     *
     * `cause: Throwable` is intentionally still here — a Java caller may pass a
     * `SerializationException` AS the cause, and that is fine. It classifies
     * their own failure without claiming this SDK's decoder produced it, which
     * is the whole distinction.
     */
    @Test
    fun thePublicConstructorKeepsItsShape() {
        val signatures = javaSelectableConstructors
            .map { it.parameterTypes.map { p -> p.simpleName } }

        assertEquals(
            listOf(listOf("String", "Integer", "String", "boolean", "String", "Throwable")),
            signatures,
            "exactly one Java-selectable constructor is expected, unchanged since #751",
        )
    }

    /**
     * The producer stays name-mangled. Dropping `internal` from the factory —
     * or making it `@JvmStatic`, or moving it back to a constructor — puts a
     * plainly-named `malformedBody`, or none at all, on the Java surface, which
     * is the same accident one step over.
     *
     * Everything here is looked up by reflection rather than by referring to
     * `Api.Companion` in source, deliberately: this test has to COMPILE against
     * the shape it rejects. Naming the companion would turn its verdict on the
     * un-fixed code into "unresolved reference", and a guard that can only fail
     * by failing to build proves nothing about what the build emitted.
     */
    @Test
    fun theMarkerFactoryIsOnlyReachableUnderAMangledName() {
        val surface = BasecampException.Api::class.java.declaredClasses.toList() +
            BasecampException.Api::class.java
        val names = surface
            .flatMap { it.methods.asSequence() }
            .map { it.name }
            .filter { it.startsWith("malformedBody") }

        assertTrue(
            names.none { it == "malformedBody" },
            "the marker factory must not be callable under its plain name from Java, got: $names",
        )
        assertTrue(
            names.any { it.startsWith("malformedBody$") },
            "expected the mangled internal factory to exist on Api or its companion, got: $names",
        )
    }
}
