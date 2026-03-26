from typing import Generator
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, AIMessage


def _build_context(artifacts: list[dict]) -> str:
    if not artifacts:
        return "No artifacts in collection yet."

    lines = []
    for a in artifacts:
        enrichment = a.get("enrichment") or {}
        summary = enrichment.get("summary") or a.get("content", "")[:200]
        topics = ", ".join(enrichment.get("topics") or [])
        line = f"- [{a['type'].upper()}] {a['label']}"
        if topics:
            line += f" ({topics})"
        line += f": {summary}"
        lines.append(line)

    return "\n".join(lines)


def stream_chat(
    message: str,
    history: list[dict],
    artifacts: list[dict],
) -> Generator[str, None, None]:
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.7, streaming=True)

    context = _build_context(artifacts)

    system = f"""You are Aladin, a research intelligence assistant.
You help the user reason over their research collection — artifacts they have gathered including links, notes, documents, and recordings.

Here is their current collection:
{context}

Answer questions about their research, help connect ideas, surface patterns, and suggest what to explore next.
Be concise. Cite specific artifacts when relevant."""

    messages = [SystemMessage(content=system)]

    for msg in history[-10:]:  # last 10 turns for context
        if msg["role"] == "user":
            messages.append(HumanMessage(content=msg["content"]))
        elif msg["role"] == "assistant":
            messages.append(AIMessage(content=msg["content"]))

    messages.append(HumanMessage(content=message))

    for chunk in llm.stream(messages):
        token = chunk.content
        if token:
            yield token
