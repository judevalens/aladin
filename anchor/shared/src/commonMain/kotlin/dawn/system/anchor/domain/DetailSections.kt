package dawn.system.anchor.domain

/**
 * Composes a node's detail from the shared section vocabulary.
 *
 * This is the handoff's central rule made executable: **purpose selects sections, not
 * layout**. A learning folder would add objective and topics; a research folder adds its
 * hypothesis and slots; a plain folder is contents alone. Same renderer, fewer sections.
 *
 * Pure — takes what the store knows, returns what to draw — so the rule can be tested
 * without a device, and so no section can quietly invent data the workspace does not have.
 * Where the backend has nothing to say (a hypothesis, a learning objective), the section
 * is *omitted* rather than filled with a placeholder.
 */
fun sectionsFor(
    node: WorkspaceNode,
    children: List<WorkspaceNode>,
): List<DetailSection> = buildList {
    when (node.kind) {
        NodeKind.Research -> {
            add(researchStats(children))
            addContents(children)
            add(DetailSection.Note("Slots (overview · manifest · runs · code) arrive with the research surface."))
        }

        NodeKind.Folder -> {
            if (children.isNotEmpty()) add(folderStats(children))
            addContents(children)
            if (children.isEmpty()) add(DetailSection.Note("This folder is empty."))
        }

        NodeKind.Artifact -> {
            add(artifactStats(node))
            node.summary?.takeIf { it.isNotBlank() }?.let {
                add(DetailSection.Prose(label = "summary", statement = it))
            }
        }
    }
}

private fun MutableList<DetailSection>.addContents(children: List<WorkspaceNode>) {
    if (children.isEmpty()) return
    add(
        DetailSection.Contents(
            label = "what's in here",
            items = children.map { child ->
                ContentRow(
                    id = child.id,
                    title = child.title.ifBlank { "Untitled" },
                    meta = child.artifactType,
                    kind = child.kind,
                    artifactType = child.artifactType,
                )
            },
        ),
    )
}

private fun folderStats(children: List<WorkspaceNode>) = DetailSection.Stats(
    listOf(
        StatCard("items", children.size.toString()),
        StatCard("folders", children.count { it.isContainer }.toString()),
    ),
)

private fun researchStats(children: List<WorkspaceNode>) = DetailSection.Stats(
    listOf(
        StatCard("items", children.size.toString(), Tone.Amber),
        StatCard("folders", children.count { it.isContainer }.toString()),
    ),
)

private fun artifactStats(node: WorkspaceNode) = DetailSection.Stats(
    listOf(StatCard("kind", node.artifactType ?: "artifact")),
)
