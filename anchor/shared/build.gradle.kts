import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
	  alias(libs.plugins.kotlinMultiplatform)
	  alias(libs.plugins.androidMultiplatformLibrary)
	  alias(libs.plugins.composeMultiplatform)
	  alias(libs.plugins.composeCompiler)
	  alias(libs.plugins.kotlinSerialization)
	  // Parcelize ships with the Kotlin Gradle plugin (already on the classpath), so it is
	  // applied by id without a version to avoid a resolution conflict.
	  id("org.jetbrains.kotlin.plugin.parcelize")
	  alias(libs.plugins.sqldelight)
}

kotlin {
	  compilerOptions {
			// expect/actual classes (DatabaseDriverFactory) are Beta but stable in practice.
			freeCompilerArgs.add("-Xexpect-actual-classes")
	  }

	  listOf(
			iosArm64(),
			iosSimulatorArm64()
	  ).forEach { iosTarget ->
			iosTarget.binaries.framework {
				  baseName = "Shared"
				  isStatic = true
			}
	  }

	  androidLibrary {
			namespace = "dawn.system.anchor.shared"
			compileSdk = libs.versions.android.compileSdk.get().toInt()
			minSdk = libs.versions.android.minSdk.get().toInt()

			compilerOptions {
				  jvmTarget = JvmTarget.JVM_11
			}
			androidResources {
				  enable = true
			}
			withHostTest {
				  isIncludeAndroidResources = true
			}
	  }

	  sourceSets {
			androidMain.dependencies {
				  implementation(libs.compose.uiToolingPreview)
				  implementation(libs.ktor.client.okhttp)
				  implementation(libs.sqldelight.driver.android)
			}
			commonMain.dependencies {
				  implementation(libs.compose.runtime)
				  implementation(libs.compose.foundation)
				  implementation(libs.compose.material3)
				  implementation(libs.compose.ui)
				  implementation(libs.compose.components.resources)
				  implementation(libs.compose.uiToolingPreview)
				  implementation(libs.androidx.lifecycle.viewmodelCompose)
				  implementation(libs.androidx.lifecycle.runtimeCompose)

				  // Circuit — navigation + UDF
				  implementation(libs.circuit.foundation)
				  implementation(libs.circuit.runtime)

				  // Ktor — HTTP client
				  implementation(libs.ktor.client.core)
				  implementation(libs.ktor.client.contentNegotiation)
				  implementation(libs.ktor.serialization.json)
				  implementation(libs.ktor.client.logging)
				  implementation(libs.ktor.client.auth)

				  // SQLDelight — local SQLite
				  implementation(libs.sqldelight.runtime)
				  implementation(libs.sqldelight.coroutines)
				  implementation(libs.sqldelight.primitiveAdapters)

				  // kotlinx
				  implementation(libs.kotlinx.coroutines.core)
				  implementation(libs.kotlinx.serialization.json)

				  // Koin — dependency injection
				  implementation(libs.koin.core)
			}
			iosMain.dependencies {
				  implementation(libs.ktor.client.darwin)
				  implementation(libs.sqldelight.driver.native)
			}
			commonTest.dependencies {
				  implementation(libs.kotlin.test)
				  implementation(libs.kotlinx.coroutines.test)
			}
	  }
}

// Register @CommonParcelize as a parcelize trigger so common-code Circuit Screens get
// Android Parcelable codegen without importing the Android-only @Parcelize. (Aliasing
// @Parcelize via typealias is unsupported on Kotlin 2.0+, hence the additionalAnnotation
// route.) Scoped to JVM/Android compilations — the parcelize plugin is not loaded for the
// iOS native compile, so passing the option there would fail.
tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile>().configureEach {
	  compilerOptions.freeCompilerArgs.addAll(
			"-P",
			"plugin:org.jetbrains.kotlin.parcelize:additionalAnnotation=dawn.system.anchor.services.di.CommonParcelize",
	  )
}

sqldelight {
	  databases {
			create("AnchorDatabase") {
				  packageName.set("dawn.system.anchor.db")
				  // 3.24+ for real UPSERT (ON CONFLICT DO UPDATE): the sync guard needs to
				  // merge a row without clobbering columns the frame did not carry.
				  dialect(libs.sqldelight.dialect.sqlite)
			}
	  }
}

dependencies {
	  androidRuntimeClasspath(libs.compose.uiTooling)
}