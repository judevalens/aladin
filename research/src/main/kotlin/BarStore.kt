/**
 * Reading bars out of DuckDB.
 *
 * The store is long — (ts, symbol, close) — because that is the shape real bars
 * arrive in. Strategies want a (time × symbol) matrix, so [loadBars] pivots once at
 * load and hands back primitive arrays. Nothing downstream touches a ResultSet.
 */

import org.jetbrains.kotlinx.dataframe.AnyCol
import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.ColumnSelector
import org.jetbrains.kotlinx.dataframe.ColumnsSelector
import org.jetbrains.kotlinx.dataframe.api.select
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.io.TableColumnMetadata
import org.jetbrains.kotlinx.dataframe.io.TableMetadata
import org.jetbrains.kotlinx.dataframe.io.db.DbType
import org.jetbrains.kotlinx.dataframe.io.readSqlQuery
import org.jetbrains.kotlinx.dataframe.schema.ColumnSchema
import java.sql.ResultSet
import kotlin.reflect.KType
import org.jetbrains.kotlinx.multik.api.mk
import org.jetbrains.kotlinx.multik.api.ndarray
import org.jetbrains.kotlinx.multik.ndarray.data.D1Array
import org.jetbrains.kotlinx.multik.ndarray.data.D2Array
import java.sql.Connection
import java.sql.DriverManager

const val RESEARCH_DB = "data/research.duckdb"

fun openDb(path: String = RESEARCH_DB): Connection =
    DriverManager.getConnection("jdbc:duckdb:$path")

/**
 * Kotlin DataFrame ships DbTypes for H2/MariaDB/MySQL/MSSQL/SQLite/PostgreSQL only, so
 * auto-detection from a `jdbc:duckdb:` URL fails. `DbType` is an open extension point
 * though — this registers DuckDB properly rather than pretending to be Postgres.
 */
object DuckDb : DbType("duckdb") {
    override val driverClassName = "org.duckdb.DuckDBDriver"
    override fun convertSqlTypeToColumnSchemaValue(tableColumnMetadata: TableColumnMetadata): ColumnSchema? = null
    override fun convertSqlTypeToKType(tableColumnMetadata: TableColumnMetadata): KType? = null
    override fun isSystemTable(tableMetadata: TableMetadata): Boolean =
        tableMetadata.schemaName?.lowercase() in setOf("information_schema", "pg_catalog")
    override fun buildTableMetadata(tables: ResultSet): TableMetadata = TableMetadata(
        tables.getString("TABLE_NAME"), tables.getString("TABLE_SCHEM"), tables.getString("TABLE_CAT"),
    )
}

/** Run a query and get a DataFrame back. */
fun Connection.frame(sql: String, limit: Int = Int.MIN_VALUE): AnyFrame =
    DataFrame.readSqlQuery(this, sql, limit, dbType = DuckDb)

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

// ---------------------------------------------------------------------------
// DataFrame -> multik.
//
// No artifact ships this; the canonical implementation lives in the dataframe
// repo's `examples/projects/multik` as compatibilityLayer.kt. The selector API
// below follows it, but the fill is different on purpose: theirs goes through
// `toList()`, boxing every value into a java.lang.Double (~32 bytes each against
// 8, plus an object per value for the GC). At bar-store scale that is the
// difference between ~4 GB and ~16 GB, so these write straight into a flat
// primitive array instead.
// ---------------------------------------------------------------------------

private fun List<AnyCol>.fillD2(rows: Int): D2Array<Double> {
    require(isNotEmpty()) { "no columns selected" }
    val flat = DoubleArray(rows * size)
    forEachIndexed { c, col ->
        for (r in 0 until rows) {
            flat[r * size + c] = (col[r] as? Number)?.toDouble()
                ?: error("column '${col.name()}' holds a non-numeric value at row $r")
        }
    }
    return mk.ndarray(flat, rows, size)
}

/** Selected columns as a (rows x cols) matrix: `df.convertToMultik { colsOf<Double>() }`. */
fun <T> DataFrame<T>.convertToMultik(selector: ColumnsSelector<T, Number?>): D2Array<Double> =
    select(selector).columns().fillD2(rowsCount())

/** Every numeric column, in frame order — non-numeric columns (ts, symbol) are skipped. */
fun AnyFrame.convertToMultik(): D2Array<Double> =
    columns().filter { it.values().firstOrNull { v -> v != null } is Number }.fillD2(rowsCount())

/** One column as a 1-D array: `df.convertToMultikD1 { close }`. */
fun <T> DataFrame<T>.convertToMultikD1(selector: ColumnSelector<T, Number?>): D1Array<Double> {
    val col = select(selector).columns().single()
    return mk.ndarray(DoubleArray(rowsCount()) { (col[it] as Number).toDouble() })
}
