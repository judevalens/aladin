/**
 * Config from the real environment, falling back to `research/.env`.
 *
 * Gradle's `run` task does not load .env files, so without this the smoke test would
 * see no key even though one is sitting right there.
 */

import java.io.File

object Env {
    private val fromFile: Map<String, String> by lazy {
        File(".env").takeIf { it.exists() }?.readLines().orEmpty()
            .map { it.trim() }
            .filter { it.isNotEmpty() && !it.startsWith("#") && "=" in it }
            .associate { line ->
                line.substringBefore("=").trim() to
                        line.substringAfter("=").trim().trim('"', '\'')
            }
    }

    operator fun get(key: String): String? = System.getenv(key) ?: fromFile[key]

    fun require(key: String): String =
        get(key) ?: error("$key is not set — put it in the environment or research/.env")
}
