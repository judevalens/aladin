/**
 * Reading bars out of DuckDB.
 *
 * The store is long — (ts, symbol, close) — because that is the shape real bars
 * arrive in. Strategies want a (time × symbol) matrix, so [loadBars] pivots once at
 * load and hands back primitive arrays. Nothing downstream touches a ResultSet.
 */

import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.io.db.PostgreSql
import org.jetbrains.kotlinx.dataframe.io.readResultSet
import org.jetbrains.kotlinx.multik.api.mk
import org.jetbrains.kotlinx.multik.api.ndarray
import org.jetbrains.kotlinx.multik.ndarray.data.D2Array
import java.sql.Connection
import java.sql.DriverManager

const val RESEARCH_DB = "data/research.duckdb"

fun openDb(path: String = RESEARCH_DB): Connection =
    DriverManager.getConnection("jdbc:duckdb:$path")

/**
 * Kotlin DataFrame rejects `jdbc:duckdb:` URLs — its supported-database list is
 * hardcoded. Going through the ResultSet with an explicit DbType works, because
 * DuckDB's types are Postgres-shaped.
 */
fun Connection.frame(sql: String): AnyFrame =
    createStatement().use { st -> st.executeQuery(sql).use { DataFrame.readResultSet(it, PostgreSql) } }

class BarMatrix(
    val dates: List<String>,
    val symbols: List<String>,
    /** row-major (time × symbol) — what multik wants */
    val rowMajor: DoubleArray,
) {
    val rows get() = dates.size
    val cols get() = symbols.size

    /** column-major — per-symbol contiguous, what the raw per-bar loop wants */
    fun colMajor() = DoubleArray(rows * cols) { i -> rowMajor[(i % rows) * cols + i / rows] }

    fun nd(): D2Array<Double> = mk.ndarray(rowMajor, rows, cols)
}

fun loadBars(table: String = "bars", db: String = RESEARCH_DB): BarMatrix =
    openDb(db).use { conn ->
        val symbols = mutableListOf<String>()
        conn.createStatement().use { st ->
            st.executeQuery("SELECT DISTINCT symbol FROM $table ORDER BY symbol").use { rs ->
                while (rs.next()) symbols += rs.getString(1)
            }
        }
        val dates = mutableListOf<String>()
        val values = mutableListOf<Double>()
        conn.createStatement().use { st ->
            st.executeQuery("SELECT ts, symbol, close FROM $table ORDER BY ts, symbol").use { rs ->
                var lastTs = ""
                while (rs.next()) {
                    val ts = rs.getTimestamp(1).toLocalDateTime().toLocalDate().toString()
                    if (ts != lastTs) { dates += ts; lastTs = ts }
                    values += rs.getDouble(3)
                }
            }
        }
        check(values.size == dates.size * symbols.size) {
            "ragged store: ${values.size} values for ${dates.size} bars x ${symbols.size} symbols"
        }
        BarMatrix(dates, symbols, values.toDoubleArray())
    }
