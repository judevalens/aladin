package dawn.system.anchor.features.note

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The bridge is a seam between two languages, evaluated as source into a live page. A
 * mistake here is a JavaScript syntax error inside a web view — a blank editor with the
 * reason buried in a console nobody is attached to.
 */
class WebSurfaceHostTest {

    private fun page(id: String, title: String = "A note") =
        WebPane(id = id, kind = WebPane.Kind.Page, title = title)

    @Test
    fun `the command carries the whole desired state`() {
        val command = syncCommand(
            panes = listOf(
                page("p1", "Collars"),
                WebPane("s1", WebPane.Kind.Shard, "Payoff"),
                WebPane("b1", WebPane.Kind.Board, "Board"),
            ),
            activeId = "s1",
        )

        assertEquals(
                """window.__aladinHost.sync({panes:[""" +
                """{id:"p1",kind:"page",title:"Collars"},""" +
                """{id:"s1",kind:"shard",title:"Payoff"},""" +
                """{id:"b1",kind:"board",title:"Board"}""" +
                """],active:"s1",insets:{bottom:0}});""",
            command,
        )
    }

    /**
     * Idempotence is the whole design: the command is re-evaluated after a web content
     * process dies, so it has to describe a destination rather than a step.
     */
    @Test
    fun `the same state produces the same command`() {
        val panes = listOf(page("p1"), page("p2"))

        assertEquals(syncCommand(panes, "p1"), syncCommand(panes, "p1"))
    }

    @Test
    fun `nothing open is a valid state, not an omitted call`() {
        assertEquals(
            "window.__aladinHost.sync({panes:[],active:null,insets:{bottom:0}});",
            syncCommand(emptyList(), null),
        )
    }

    /**
     * An active id that is not mounted would leave the page showing nothing while the shell
     * believed a note was up — the two sides silently disagreeing, which is exactly what a
     * declarative bridge is supposed to make impossible.
     */
    @Test
    fun `an active id outside the pane set is dropped`() {
        val command = syncCommand(listOf(page("p1")), activeId = "p9")

        assertTrue(command.contains("active:null"), command)
    }

    /**
     * A title is user input and goes into a script literal. An unescaped quote is a syntax
     * error and a blank page; an unescaped `<` could close the surrounding element.
     */
    @Test
    fun `titles are escaped, not trusted`() {
        val command = syncCommand(listOf(page("p1", """he said "no" </script>""")), "p1")

        assertTrue(command.contains("""\"no\""""), command)
        // `<` becomes \u003c, so a title can never close the surrounding element.
        assertTrue(command.contains("\\u003c/script>"), command)
        assertTrue(!command.contains("</script>"), command)
    }

    @Test
    fun `the bootstrap installs a queue so early state is not lost`() {
        val bootstrap = embedBootstrap(
            token = "t",
            collabWsUrl = "ws://10.0.0.2:3501",
            userName = "Jude",
            apiBaseUrl = "http://10.0.0.2:8000",
        )

        // The stub must exist before the bundle's own scripts run, or a sync arriving
        // before React mounts would throw and the first note would never open.
        assertTrue(bootstrap.contains("window.__aladinHost=window.__aladinHost||"), bootstrap)
        assertTrue(bootstrap.contains("_queued"), bootstrap)
        // No pageId: the session is what the bootstrap carries, panes come over the bridge.
        assertTrue(!bootstrap.contains("pageId"), bootstrap)
    }

    /**
     * The API origin is read once, when the view is built — long before the first shard is
     * opened. Omitting it fails at the worst moment: notes work, and only a shard reveals
     * that the page has nothing to serve itself from.
     */
    @Test
    fun `the bootstrap carries the api origin a shard needs`() {
        val bootstrap = embedBootstrap(
            token = "t",
            collabWsUrl = "ws://10.0.0.2:3501",
            userName = "Jude",
            apiBaseUrl = "http://10.0.0.2:8000",
        )

        assertTrue(bootstrap.contains("""apiBaseUrl:"http://10.0.0.2:8000""""), bootstrap)
    }

    @Test
    fun `a token with a quote cannot break out of its literal`() {
        val bootstrap = embedBootstrap(
            token = """a"b\c""",
            collabWsUrl = "ws://x",
            userName = "u",
            apiBaseUrl = "http://x",
        )

        assertTrue(bootstrap.contains("""token:"a\"b\\c""""), bootstrap)
    }

    @Test
    fun `a user keeps the same presence colour between sessions`() {
        assertEquals(colorForUser("jude@example.com"), colorForUser("jude@example.com"))
        assertTrue(colorForUser("a").startsWith("hsl("))
    }

    // ── web → native ──

    @Test
    fun `openArtifact parses to its id`() {
        assertEquals(
            HostMessage.OpenArtifact("a-123"),
            parseHostMessage("""{"type":"openArtifact","id":"a-123"}"""),
        )
    }

    @Test
    fun `haptic parses to its weight and drops unknown weights`() {
        assertEquals(
            HostMessage.Haptic(dawn.system.anchor.services.platform.Haptic.Select),
            parseHostMessage("""{"type":"haptic","kind":"select"}"""),
        )
        assertEquals(
            HostMessage.Haptic(dawn.system.anchor.services.platform.Haptic.Light),
            parseHostMessage("""{"type":"haptic","kind":"light"}"""),
        )
        assertEquals(null, parseHostMessage("""{"type":"haptic","kind":"earthquake"}"""))
        assertEquals(null, parseHostMessage("""{"type":"haptic"}"""))
    }

    /** A board must not shrink above the keyboard — a resized canvas jumps its camera. */
    @Test
    fun `only text surfaces give way to the keyboard`() {
        assertTrue(givesWayToKeyboard(WebPane.Kind.Page))
        assertTrue(givesWayToKeyboard(WebPane.Kind.Shard))
        assertTrue(givesWayToKeyboard(null))
        assertTrue(!givesWayToKeyboard(WebPane.Kind.Board))
    }

    @Test
    fun `ready parses, with or without extra fields`() {
        assertEquals(HostMessage.Ready, parseHostMessage("""{"type":"ready"}"""))
        assertEquals(HostMessage.Ready, parseHostMessage("""{"type":"ready","v":2}"""))
    }

    /**
     * A newer bundle must never crash an older shell: unknown types, missing ids and
     * non-JSON bodies (the pre-JSON bundle posted an object, which lands as an
     * NSDictionary description) all read as "nothing to do".
     */
    @Test
    fun `unknown or malformed messages are ignored`() {
        assertEquals(null, parseHostMessage("""{"type":"askAbout","id":"x"}"""))
        assertEquals(null, parseHostMessage("""{"type":"openArtifact"}"""))
        assertEquals(null, parseHostMessage("""{"type":"openArtifact","id":""}"""))
        assertEquals(null, parseHostMessage("{ type = ready; }"))
        assertEquals(null, parseHostMessage("not json at all"))
    }
}
