import { BehaviorSubject, type Observable } from "rxjs";
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
import {
  createErrorLoadable,
  createIdleLoadable,
  createLoadingLoadable,
  createReadyLoadable,
  type Loadable,
} from "@/shared/flow/loadable";

export class SourcesCatalogService {
  private readonly sourcesSubject = new BehaviorSubject<Loadable<Source[]>>(createIdleLoadable([]));
  private readonly providersSubject = new BehaviorSubject<Loadable<ProviderConnectionProvider[]>>(
    createIdleLoadable([]),
  );
  private readonly tokensSubject = new BehaviorSubject<Loadable<IntegrationToken[]>>(
    createIdleLoadable([]),
  );
  private sourcesLoadPromise: Promise<void> | null = null;
  private providersLoadPromise: Promise<void> | null = null;
  private tokensLoadPromise: Promise<void> | null = null;

  constructor(
    private readonly sourcesRepo: SourcesRepo,
    private readonly integrationRepo: IntegrationRepo,
  ) {}

  observeSources(): Observable<Loadable<Source[]>> {
    return this.sourcesSubject.asObservable();
  }

  getSourcesSnapshot(): Loadable<Source[]> {
    return this.sourcesSubject.getValue();
  }

  observeProviders(): Observable<Loadable<ProviderConnectionProvider[]>> {
    return this.providersSubject.asObservable();
  }

  getProvidersSnapshot(): Loadable<ProviderConnectionProvider[]> {
    return this.providersSubject.getValue();
  }

  observeTokens(): Observable<Loadable<IntegrationToken[]>> {
    return this.tokensSubject.asObservable();
  }

  getTokensSnapshot(): Loadable<IntegrationToken[]> {
    return this.tokensSubject.getValue();
  }

  ensureSourcesLoaded() {
    const snapshot = this.getSourcesSnapshot();
    if (snapshot.status === "ready" || snapshot.status === "loading") {
      return this.sourcesLoadPromise ?? Promise.resolve();
    }
    return this.refreshSources();
  }

  ensureProvidersLoaded() {
    const snapshot = this.getProvidersSnapshot();
    if (snapshot.status === "ready" || snapshot.status === "loading") {
      return this.providersLoadPromise ?? Promise.resolve();
    }
    return this.refreshProviders();
  }

  ensureTokensLoaded() {
    const snapshot = this.getTokensSnapshot();
    if (snapshot.status === "ready" || snapshot.status === "loading") {
      return this.tokensLoadPromise ?? Promise.resolve();
    }
    return this.refreshTokens();
  }

  async refreshSources() {
    if (!this.sourcesLoadPromise) {
      const previous = this.getSourcesSnapshot().data;
      this.sourcesSubject.next(createLoadingLoadable(previous));
      this.sourcesLoadPromise = this.sourcesRepo
        .getSources()
        .then((sources) => {
          this.sourcesSubject.next(createReadyLoadable(sources));
        })
        .catch((error) => {
          this.sourcesSubject.next(
            createErrorLoadable(
              previous,
              error instanceof Error ? error.message : "Failed to load sources.",
            ),
          );
          throw error;
        })
        .finally(() => {
          this.sourcesLoadPromise = null;
        });
    }
    return this.sourcesLoadPromise;
  }

  async refreshProviders() {
    if (!this.providersLoadPromise) {
      const previous = this.getProvidersSnapshot().data;
      this.providersSubject.next(createLoadingLoadable(previous));
      this.providersLoadPromise = this.integrationRepo
        .getProviders()
        .then((providers) => {
          this.providersSubject.next(createReadyLoadable(providers));
        })
        .catch((error) => {
          this.providersSubject.next(
            createErrorLoadable(
              previous,
              error instanceof Error ? error.message : "Failed to load providers.",
            ),
          );
          throw error;
        })
        .finally(() => {
          this.providersLoadPromise = null;
        });
    }
    return this.providersLoadPromise;
  }

  async refreshTokens() {
    if (!this.tokensLoadPromise) {
      const previous = this.getTokensSnapshot().data;
      this.tokensSubject.next(createLoadingLoadable(previous));
      this.tokensLoadPromise = this.integrationRepo
        .getIntegrationTokens()
        .then((tokens) => {
          this.tokensSubject.next(createReadyLoadable(tokens));
        })
        .catch((error) => {
          this.tokensSubject.next(
            createErrorLoadable(
              previous,
              error instanceof Error ? error.message : "Failed to load integration tokens.",
            ),
          );
          throw error;
        })
        .finally(() => {
          this.tokensLoadPromise = null;
        });
    }
    return this.tokensLoadPromise;
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

  async startProviderConnect(provider: string): Promise<ProviderConnectSession> {
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

  async createIntegrationToken(request: IntegrationTokenCreateRequest): Promise<CreatedIntegrationToken> {
    const created = await this.integrationRepo.createIntegrationToken(request);
    await Promise.all([this.refreshTokens(), this.refreshProviders(), this.refreshSources()]);
    return created;
  }

  async revokeIntegrationToken(tokenId: string) {
    await this.integrationRepo.revokeIntegrationToken(tokenId);
    await this.refreshTokens();
  }
}
