plugins {
    kotlin("jvm") version "2.3.20"
    application
}

repositories { mavenCentral() }

dependencies {
    implementation("org.apache.arrow:arrow-vector:18.1.0")
    implementation("org.apache.arrow:arrow-memory-netty:18.1.0")
    implementation("org.apache.arrow:arrow-compression:18.1.0")   // pandas writes LZ4-framed feather
    implementation(kotlin("reflect"))
    implementation("org.jetbrains.kotlinx:dataframe-arrow:0.15.0")
    implementation("org.jetbrains.kotlinx:multik-default:0.3.1")
}

kotlin { jvmToolchain(21) }

application {
    mainClass.set(System.getProperty("mc") ?: "MaCrossKt")
    // Arrow's off-heap allocator needs this on JDK 17+
    applicationDefaultJvmArgs = listOf("--add-opens=java.base/java.nio=ALL-UNNAMED")
}
