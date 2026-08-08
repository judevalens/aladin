package aladin.store

import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.io.TableColumnMetadata
import org.jetbrains.kotlinx.dataframe.io.TableMetadata
import org.jetbrains.kotlinx.dataframe.io.db.DbType
import org.jetbrains.kotlinx.dataframe.io.readResultSet
import org.jetbrains.kotlinx.dataframe.schema.ColumnSchema
import java.sql.Connection
import java.sql.DriverManager
import java.sql.ResultSet
import java.util.Properties
import kotlin.reflect.KType

const val RESEARCH_DB = "data/research.duckdb"

/**
 * The store on disk. Derived — delete it and it rebuilds by fetching.
 *
 * [readOnly] takes DuckDB's shared lock instead of the exclusive one, which is what lets
 * several processes open the same file at once — a sweep alongside a DataGrip session,
 * say. The rule is **one writer or many readers, never both**: a reader cannot attach
 * while a writer holds the file, and vice versa.
 *
 * Two consequences worth knowing before reaching for it. Every DDL statement throws
 * under a read-only handle, `IF NOT EXISTS` included, so the schema must already exist.
 * And a file that is not there cannot be opened read-only, since there is nothing to
 * attach to — read-only never creates.
 */
fun openDb(path: String = RESEARCH_DB, readOnly: Boolean = false): Connection =
    DriverManager.getConnection(
        "jdbc:duckdb:$path",
        Properties().apply { if (readOnly) setProperty("duckdb.read_only", "true") },
    )

/**
 * DuckDB's own view of the handle.
 *
 * Asked rather than tracked, so it is right even for a connection this code did not
 * open — and because the alternative is discovering it from the driver, which reports a
 * rejected write as "an unsuccessful or closed pending query result": true, and useless.
 */
internal fun Connection.isReadOnlyStore(): Boolean =
    queryOne("SELECT current_setting(\'access_mode\')") { it.getString(1) }
        .equals("READ_ONLY", ignoreCase = true)

/** Tables present in the main schema. */
internal fun Connection.tableNames(): Set<String> =
    createStatement().use { st ->
        st.executeQuery("SELECT table_name FROM information_schema.tables WHERE table_schema = \'main\'")
            .use { rs -> buildSet { while (rs.next()) add(rs.getString(1)) } }
    }

/**
 * A throwaway store, torn down with its last connection (~43ms to stand up).
 *
 * NAMED in-memory on purpose: a bare `jdbc:duckdb:` gives every connection its own
 * database, so helpers that open separately would each see an empty one. The flip side
 * is that the database dies with its last connection — seed and read through the same one.
 */
fun openScratch(name: String = "scratch"): Connection =
    DriverManager.getConnection("jdbc:duckdb::memory:$name")

/**
 * Kotlin DataFrame ships DbTypes for six databases and auto-detects from the JDBC URL,
 * so `jdbc:duckdb:` fails. `DbType` is an open extension point, so DuckDB is registered
 * properly here rather than masquerading as PostgreSQL.
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

/**
 * Run a query and get a DataFrame back.
 *
 * Uses `readResultSet` rather than `readSqlQuery` because the latter insists a query
 * starts with SELECT, which rejects the CTEs the grid query needs.
 */
fun Connection.frame(sql: String): AnyFrame =
    createStatement().use { st -> st.executeQuery(sql).use { DataFrame.readResultSet(it, DuckDb) } }

/** Single-value query, for counts and bounds. */
internal fun <T> Connection.queryOne(sql: String, read: (ResultSet) -> T): T =
    createStatement().use { st -> st.executeQuery(sql).use { it.next(); read(it) } }

/**
 * SQL identifiers cannot be bound as parameters, so anything interpolated into a
 * statement is checked against what actually exists first. Without this a
 * caller-supplied column name is an injection point.
 */
internal fun requireIdentifier(value: String, allowed: Set<String>, what: String): String {
    require(value in allowed) { "unknown $what '$value' — expected one of ${allowed.sorted()}" }
    return value
}

/** Quote a value for the few places binding is not available. */
internal fun sqlLiteral(value: String): String =
    "'" + value.replace("'", "''") + "'"
