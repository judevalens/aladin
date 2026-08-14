package dawn.system.anchor.services.database

import app.cash.sqldelight.db.SqlDriver
import dawn.system.anchor.db.AnchorDatabase

/**
 * Creates the platform SQLite driver. Android needs a [android.content.Context], so the
 * constructors differ across actuals (see the platform implementations).
 */
expect class DatabaseDriverFactory {
    fun create(): SqlDriver
}

const val ANCHOR_DB_NAME = "anchor.db"

/** Opens the typed SQLDelight database over the platform driver. */
fun createDatabase(driverFactory: DatabaseDriverFactory): AnchorDatabase =
    AnchorDatabase(driverFactory.create())
