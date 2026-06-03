import { type Observable } from "rxjs";
import type { IntegrationRepo } from "@/repos/integrations/integration-repo";
import type { SourcesRepo } from "@/repos/sources/sources-repo";
import type {
  CreatedIntegrationToken,
  IntegrationToken,
  IntegrationTokenCreateRequest,
  ProviderConnectionProvider,
  ProviderConnectSession,
  Source,
  SourceCreateRequest,
} from "@/shared/api/models";
import { LazyResultStream } from "@/shared/flow/lazy-result-stream";
import { type Result } from "@/shared/flow/result";

export class SourcesCatalogService {
  private readonly sourcesStream = new LazyResultStream<Source[]>(() =>
    this.sourcesRepo.getSources(),
  );
  private readonly providersStream = new LazyResultStream<
    ProviderConnectionProvider[]
  >(() => this.integrationRepo.getProviders());
  private readonly tokensStream = new LazyResultStream<IntegrationToken[]>(() =>
    this.integrationRepo.getIntegrationTokens(),
  );

  constructor(
    private readonly sourcesRepo: SourcesRepo,
    private readonly integrationRepo: IntegrationRepo,
  ) {}

  sources(): Observable<Result<Source[]>> {
    return this.sourcesStream.observable();
  }

  providers(): Observable<Result<ProviderConnectionProvider[]>> {
    return this.providersStream.observable();
  }

  tokens(): Observable<Result<IntegrationToken[]>> {
    return this.tokensStream.observable();
  }

  refreshSources(): Promise<void> {
    return this.sourcesStream.refresh();
  }

  refreshProviders(): Promise<void> {
    return this.providersStream.refresh();
  }

  refreshTokens(): Promise<void> {
    return this.tokensStream.refresh();
  }

  async createSource(input: SourceCreateRequest) {
    const source = await this.sourcesRepo.createSource(input);
    await this.refreshSources();
    return source;
  }

  async deleteSource(sourceId: string) {
    await this.sourcesRepo.deleteSource(sourceId);
    await this.refreshSources();
  }

  startProviderConnect(provider: string): Promise<ProviderConnectSession> {
    return this.integrationRepo.startProviderConnect(provider);
  }

  async syncProviders() {
    await this.integrationRepo.syncProviders();
    await Promise.all([this.refreshProviders(), this.refreshSources()]);
  }

  async disconnectProvider(connectionId: string) {
    await this.integrationRepo.disconnectProvider(connectionId);
    await Promise.all([this.refreshProviders(), this.refreshSources()]);
  }

  async createIntegrationToken(
    request: IntegrationTokenCreateRequest,
  ): Promise<CreatedIntegrationToken> {
    const created = await this.integrationRepo.createIntegrationToken(request);
    // The token now exists server-side, and `created.token` is the ONE-TIME
    // plaintext reveal — that is the critical path. Refresh the lists
    // best-effort: a refresh failure (e.g. providers/sources endpoint erroring)
    // must NOT reject this call and swallow the created token before the UI can
    // show it. The lists re-fetch on next open anyway.
    try {
      await Promise.all([
        this.refreshTokens(),
        this.refreshProviders(),
        this.refreshSources(),
      ]);
    } catch {
      // ignore — the created token is already captured and returned below
    }
    return created;
  }

  async revokeIntegrationToken(tokenId: string) {
    await this.integrationRepo.revokeIntegrationToken(tokenId);
    await this.refreshTokens();
  }
}
