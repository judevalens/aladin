from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime


@dataclass
class Post:
    """Universal post model — common across all social platforms."""
    platform: str          # "bluesky" | "reddit" | "twitter" | ...
    post_id: str
    url: str
    author_handle: str
    author_name: str
    content: str
    created_at: datetime
    like_count: int = 0
    repost_count: int = 0
    reply_count: int = 0
    embeds: list[dict] = field(default_factory=list)
    thread: list["Post"] = field(default_factory=list)  # parent posts, oldest first


@dataclass
class IngestedContent:
    """Result returned by any ingestion strategy."""
    type: str              # "post" | "article" | "paper" | "page"
    raw_url: str
    post: Post | None = None
    # Future slots: article, paper, page


class IngestStrategy(ABC):
    """Base class for all URL ingestion strategies."""

    @abstractmethod
    def can_handle(self, url: str) -> bool:
        """Return True if this strategy knows how to handle the given URL."""

    @abstractmethod
    def ingest(self, url: str) -> IngestedContent:
        """Fetch and parse the URL into structured content."""
