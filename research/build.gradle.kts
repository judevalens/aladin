plugins {
    kotlin("jvm") version "2.3.20"
    kotlin("plugin.serialization") version "2.3.20"
    application
    idea
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

// ---------------------------------------------------------------------------
// Library documentation
//
// multik and dataframe are the two libraries whose behaviour is least guessable —
// multik has silent-wrong-answer traps around views and aliasing — so reading the
// actual source beats inferring the contract from a signature.
// ---------------------------------------------------------------------------

/** Tells IntelliJ's Gradle import to attach sources and javadoc as it syncs. */
idea.module {
    isDownloadSources = true
    isDownloadJavadoc = true
}

/**
 * The doc artifacts for everything on the runtime classpath.
 *
 * Gradle exposes a Maven module's `-sources`/`-javadoc` jars as separate *variants* of
 * the same coordinates, which is what [ArtifactView.withVariantReselection] reaches:
 * same dependency graph, different [DocsType]. Resolving them through the classpath
 * means transitives are covered too — duckdb and slf4j come along without being named.
 *
 * Lenient because plenty of libraries publish neither, and a missing javadoc jar is not
 * a reason to fail a build.
 */
fun docsOfType(docsType: String): FileCollection =
    configurations.runtimeClasspath.get().incoming.artifactView {
        withVariantReselection()
        isLenient = true
        attributes {
            attribute(Usage.USAGE_ATTRIBUTE, objects.named(Usage.JAVA_RUNTIME))
            attribute(Category.CATEGORY_ATTRIBUTE, objects.named(Category.DOCUMENTATION))
            attribute(Bundling.BUNDLING_ATTRIBUTE, objects.named(Bundling.EXTERNAL))
            attribute(DocsType.DOCS_TYPE_ATTRIBUTE, objects.named(docsType))
        }
    }.files

/**
 * `./gradlew documentation` — pulls every available sources and javadoc jar into the
 * Gradle cache, where an IDE finds them, and collects them under build/documentation
 * for anything that cannot read the cache.
 */
tasks.register<Sync>("documentation") {
    group = "documentation"
    description = "Download sources + javadoc jars for every dependency."

    from(docsOfType(DocsType.SOURCES)) { into("sources") }
    from(docsOfType(DocsType.JAVADOC)) { into("javadoc") }
    into(layout.buildDirectory.dir("documentation"))

    doLast {
        val root = destinationDir
        for (kind in listOf("sources", "javadoc")) {
            val jars = File(root, kind).listFiles()?.sorted().orEmpty()
            logger.lifecycle("$kind (${jars.size}):")
            jars.forEach { logger.lifecycle("  ${it.name}  ${it.length() / 1024} KiB") }
        }
        logger.lifecycle("\n-> $root")
    }
}
