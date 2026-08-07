import org.jetbrains.kotlinx.dataframe.api.*
import org.jetbrains.kotlinx.dataframe.io.readArrowFeather
import org.jetbrains.kotlinx.dataframe.DataFrame
import java.io.File

fun main() {
    val f = File("data/bars.arrow").canonicalFile
    println("reading $f")
    val df = DataFrame.readArrowFeather(f)
    println("rows=${df.rowsCount()}  cols=${df.columnsCount()}")
    println("schema:")
    println(df.schema())
    println(df.tail(3))
}
