package dawn.system.anchor.services.database

import android.content.Context
import app.cash.sqldelight.db.SqlDriver
import app.cash.sqldelight.driver.android.AndroidSqliteDriver
import dawn.system.anchor.db.AnchorDatabase

actual class DatabaseDriverFactory(private val context: Context) {
    actual fun create(): SqlDriver =
        AndroidSqliteDriver(AnchorDatabase.Schema, context, ANCHOR_DB_NAME)
}
