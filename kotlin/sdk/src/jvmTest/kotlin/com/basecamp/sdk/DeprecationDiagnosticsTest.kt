package com.basecamp.sdk

import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import java.io.File
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * Diagnostic fixture for the deprecation-propagation work (#406).
 *
 * Kotlin is the one compiler-warning language in the matrix, so its behavior is
 * asserted at the level of actual compiler diagnostics — not "it compiles". Each
 * snippet is compiled at test time by the embedded Kotlin compiler against this
 * test's own classpath (which includes the built SDK), with all warnings
 * promoted to errors, so a deprecation warning becomes a hard compile failure we
 * can assert on. The snippets are compiled by the embedded compiler, never by
 * the ordinary SDK test compilation, so they don't perturb the normal build.
 *
 * The outer gate is the two @Test methods together (jvmTest is red unless BOTH
 * hold): reading a deprecated property must fail with the deprecation reason,
 * and the named-constructor-argument call site must stay clean — recording that
 * `@Deprecated` cannot target a `VALUE_PARAMETER`, so that site is unsupported.
 */
class DeprecationDiagnosticsTest {

    private data class CompileResult(val exitCode: ExitCode, val messages: List<String>)

    private fun compile(source: String): CompileResult {
        val tmpDir = File.createTempFile("dep-fixture", "").apply {
            delete()
            mkdirs()
        }
        try {
            val srcFile = File(tmpDir, "Fixture.kt").apply { writeText(source) }
            val outDir = File(tmpDir, "out").apply { mkdirs() }
            val messages = mutableListOf<String>()
            val collector = object : MessageCollector {
                private var errors = false
                override fun clear() {
                    errors = false
                }
                override fun hasErrors() = errors
                override fun report(
                    severity: CompilerMessageSeverity,
                    message: String,
                    location: CompilerMessageSourceLocation?,
                ) {
                    if (severity.isError) errors = true
                    messages += "$severity: $message"
                }
            }
            val args = K2JVMCompilerArguments().apply {
                freeArgs = listOf(srcFile.absolutePath)
                classpath = System.getProperty("java.class.path")
                destination = outDir.absolutePath
                // Deprecation is a warning by default; promote so it becomes an
                // asserting failure the fixture can observe.
                allWarningsAsErrors = true
                // stdlib is already on the test classpath.
                noStdlib = true
                noReflect = true
            }
            val exitCode = K2JVMCompiler().exec(collector, Services.EMPTY, args)
            return CompileResult(exitCode, messages)
        } finally {
            tmpDir.deleteRecursively()
        }
    }

    @Test
    fun readingDeprecatedPropertyFailsWithReason() {
        val result = compile(
            """
            import com.basecamp.sdk.generated.services.SearchOptions
            fun probe(o: SearchOptions): String? = o.type
            """.trimIndent(),
        )
        assertTrue(
            result.exitCode == ExitCode.COMPILATION_ERROR,
            "reading a deprecated property should fail under deprecation-as-error; " +
                "got ${result.exitCode}\n${result.messages.joinToString("\n")}",
        )
        assertTrue(
            result.messages.any { it.contains("prefer type_names[].") },
            "the diagnostic should carry the resolved deprecation reason; " +
                "got:\n${result.messages.joinToString("\n")}",
        )
    }

    @Test
    fun namedConstructorArgumentIsUnflagged() {
        val result = compile(
            """
            import com.basecamp.sdk.generated.services.SearchOptions
            fun probe(): SearchOptions = SearchOptions(type = "x")
            """.trimIndent(),
        )
        assertTrue(
            result.exitCode == ExitCode.OK,
            "constructing via the named argument should compile clean — @Deprecated " +
                "binds to the property, not the VALUE_PARAMETER, so this call site is " +
                "unsupported/unflagged; got ${result.exitCode}\n${result.messages.joinToString("\n")}",
        )
    }
}
