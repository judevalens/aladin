from pydantic import BaseModel
from langchain_openai import ChatOpenAI
from langchain_core.prompts import ChatPromptTemplate


class ArtifactEnrichment(BaseModel):
    summary: str
    entities: list[str]
    key_claims: list[str]
    topics: list[str]


_prompt = ChatPromptTemplate.from_messages([
    ("system", """You are a research assistant analyzing content from a knowledge graph.
Extract structured information from the artifact below.

- summary: 1-2 sentence summary of what this artifact is about
- entities: people, organizations, tools, or concepts explicitly mentioned
- key_claims: the main points, arguments, or facts stated
- topics: 2-5 high-level topic tags (e.g. "machine learning", "agent orchestration")

Be concise and factual. Only include what is present in the content."""),
    ("human", "Artifact type: {artifact_type}\n\nContent:\n{content}"),
])


def enrich(content: str, artifact_type: str) -> ArtifactEnrichment:
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
    chain = _prompt | llm.with_structured_output(ArtifactEnrichment)
    return chain.invoke({"content": content, "artifact_type": artifact_type})
