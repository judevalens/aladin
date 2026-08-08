package aladin

import java.io.File

/**
 * Configuration from the environment, falling back to `research/.env`.
 *
 * Gradle's `run` task does not load .env files, so without this a key sitting right
 * there would be invisible.
 */
object Env {
    private val fromFile: Map<String, String> by lazy {
        File(".env").takeIf(File::exists)?.readLines().orEmpty()
            .map(String::trim)
            .filter { it.isNotEmpty() && !it.startsWith("#") && "=" in it }
            .associate { line ->
                line.substringBefore("=").trim() to line.substringAfter("=").trim().trim('"', '\'')
            }
    }

    operator fun get(key: String): String? = System.getenv(key) ?: fromFile[key]

    fun require(key: String): String =
        get(key) ?: error("$key is not set — put it in the environment or research/.env")

    fun double(key: String, default: Double): Double =
        get(key)?.toDoubleOrNull() ?: default
}
