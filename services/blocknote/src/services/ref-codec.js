// Reference <-> markdown codec. The custom inline nodes entityMention (@entity) and
// artifactRef (#claim/#page/#shard) have `content:"none"`, so BlockNote's lossy markdown
// serializer drops them — an agent reading a page as markdown would never see a reference,
// and there'd be no way to author one back. We encode them as markdown LINKS with an
// `aladin.ref` host, which round-trips losslessly, needs no character-escaping (a literal
// `#`/`@` in prose is never matched — only the full link form is), and degrades to a normal
// clickable link anywhere else:
//
//   @entity  ->  [@Label](https://aladin.ref/entity/<id>)
//   #claim   ->  [#Label](https://aladin.ref/claim/<id>)
//   #page    ->  [#Label](https://aladin.ref/page/<id>)
//   #shard   ->  [#Label](https://aladin.ref/shard/<id>)
//
// NB: an `aladin:` URI scheme would be cleaner, but the markdown sanitizer strips non-web
// schemes (the href serializes to empty and the link is dropped on parse), so the reference
// URL rides on https with the `aladin.ref` host instead — same semantics, sanitizer-safe.
//
// Used by the converter: blocksToMd encodes refs -> links before serializing; mdToBlocks
// decodes these links -> refs after parsing.

const REF_HOST = "https://aladin.ref";
const REF_HREF = /^https:\/\/aladin\.ref\/(entity|claim|page|shard)\/(.+)$/;

function linkText(content) {
  if (!Array.isArray(content)) return "";
  return content
    .map((n) => (n && typeof n === "object" && typeof n.text === "string" ? n.text : ""))
    .join("");
}

// refToLink turns an entityMention/artifactRef inline node into a standard link node; other
// inline content passes through untouched.
function refToLink(ic) {
  if (!ic || typeof ic !== "object") return ic;
  if (ic.type === "entityMention") {
    const { entityId = "", label = "" } = ic.props ?? {};
    return {
      type: "link",
      href: `${REF_HOST}/entity/${entityId}`,
      content: [{ type: "text", text: `@${label}`, styles: {} }],
    };
  }
  if (ic.type === "artifactRef") {
    const { kind = "", targetId = "", label = "" } = ic.props ?? {};
    return {
      type: "link",
      href: `${REF_HOST}/${kind}/${targetId}`,
      content: [{ type: "text", text: `#${label}`, styles: {} }],
    };
  }
  return ic;
}

// linkToRef turns an aladin-scheme link node back into the matching reference node; other
// links (and inline content) pass through untouched.
function linkToRef(ic) {
  if (!ic || typeof ic !== "object" || ic.type !== "link") return ic;
  const m = typeof ic.href === "string" ? ic.href.match(REF_HREF) : null;
  if (!m) return ic;
  const [, kind, id] = m;
  const label = linkText(ic.content).replace(/^[@#]/, "");
  if (kind === "entity") {
    return { type: "entityMention", props: { entityId: id, label, kind: "" } };
  }
  return { type: "artifactRef", props: { kind, targetId: id, label, polarity: "" } };
}

// mapInline applies fn to every inline content node across the block tree (content arrays +
// nested children), returning new blocks (the input is left unmutated).
function mapInline(blocks, fn) {
  if (!Array.isArray(blocks)) return blocks;
  return blocks.map((b) => {
    if (!b || typeof b !== "object") return b;
    const next = { ...b };
    if (Array.isArray(b.content)) next.content = b.content.map(fn);
    if (Array.isArray(b.children)) next.children = mapInline(b.children, fn);
    return next;
  });
}

export const encodeRefsToLinks = (blocks) => mapInline(blocks, refToLink);
export const decodeLinksToRefs = (blocks) => mapInline(blocks, linkToRef);
