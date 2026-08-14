package dawn.system.anchor.services.database

import app.cash.sqldelight.db.SqlDriver
import app.cash.sqldelight.driver.native.NativeSqliteDriver
import dawn.system.anchor.db.AnchorDatabase

actual class DatabaseDriverFactory {
    actual fun create(): SqlDriver =
        NativeSqliteDriver(AnchorDatabase.Schema, ANCHOR_DB_NAME)
}
