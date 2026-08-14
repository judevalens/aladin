package dawn.system.anchor.services.di

/**
 * Multiplatform stand-in for `@Parcelize`. Circuit's [com.slack.circuit.runtime.screen.Screen]
 * is `Parcelable` on Android (so screens survive process death in the saveable back stack)
 * but a plain marker elsewhere.
 *
 * This annotation is registered with the kotlin-parcelize compiler plugin via
 * `parcelize { additionalAnnotations.add(...) }` in `build.gradle.kts`, so on Android it
 * triggers Parcelable code generation exactly like `@Parcelize`. On iOS it is an inert
 * annotation. (A typealias to `kotlinx.parcelize.Parcelize` does not work — parcelize
 * matches annotations by fully-qualified name and does not expand typealiases.)
 */
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class CommonParcelize
