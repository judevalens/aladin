plugins {
    kotlin("jvm") version "2.3.20"
    application
}

repositories { mavenCentral() }

dependencies {
    implementation(kotlin("reflect"))
    implementation("org.jetbrains.kotlinx:dataframe-core:0.15.0")
    implementation("org.jetbrains.kotlinx:multik-default:0.3.1")
    implementation("org.duckdb:duckdb_jdbc:1.5.5.1")
    implementation("org.jetbrains.kotlinx:dataframe-jdbc:0.15.0")
}

kotlin { jvmToolchain(21) }

application {
    mainClass.set(System.getProperty("mc") ?: "MaCrossKt")
}
