plugins {
    kotlin("jvm") version "2.3.20"
    kotlin("plugin.serialization") version "2.3.20"
    application
}

repositories { mavenCentral() }

dependencies {
    testImplementation(kotlin("test"))
    implementation(kotlin("reflect"))
    implementation("org.jetbrains.kotlinx:dataframe-core:0.15.0")
    implementation("org.jetbrains.kotlinx:multik-default:0.3.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")
    // something on the classpath logs through slf4j; without a binding every run
    // opens with "No SLF4J providers were found"
    runtimeOnly("org.slf4j:slf4j-simple:2.0.18")
    implementation("org.duckdb:duckdb_jdbc:1.5.5.1")
    implementation("org.jetbrains.kotlinx:dataframe-jdbc:0.15.0")
}

kotlin { jvmToolchain(21) }

tasks.test { useJUnitPlatform() }

// so the interactive cost approval can actually read a reply under `gradlew run`
tasks.named<JavaExec>("run") { standardInput = System.`in` }

application {
    mainClass.set(System.getProperty("mc") ?: "aladin.MainKt")
}
