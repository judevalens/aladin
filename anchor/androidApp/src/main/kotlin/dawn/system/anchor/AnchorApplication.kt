package dawn.system.anchor

import android.app.Application
import dawn.system.anchor.app.initKoin

class AnchorApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        initKoin(this)
    }
}
