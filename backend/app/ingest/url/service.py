from app.ingest.url.base import IngestStrategy, IngestedContent
from app.ingest.url.strategies.bluesky import BlueSkyStrategy


class UrlIngestService:
    """
    Routes a URL to the appropriate strategy and returns structured content.
    Register new strategies here as platforms are added.
    """

    def __init__(self) -> None:
        self._strategies: list[IngestStrategy] = [
            BlueSkyStrategy(),
            # RedditStrategy(),
            # ArxivStrategy(),
            # GenericPageStrategy(),  # fallback
        ]

    def ingest(self, url: str) -> IngestedContent:
        for strategy in self._strategies:
            if strategy.can_handle(url):
                return strategy.ingest(url)
        raise ValueError(f"No strategy available for URL: {url}")

    def supports(self, url: str) -> bool:
        return any(s.can_handle(url) for s in self._strategies)
