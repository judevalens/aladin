import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUpRight,
  Link2,
  RefreshCcw,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import type { ReactNode } from "react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/shared/api/client";
import type { IntegrationToken, ProviderConnectionProvider, Source } from "@/shared/api/models";
import { cn, formatHumanDate, formatRelativeTime, titleCase } from "@/shared/lib/utils";

const sourcesQueryKey = ["sources"];
const providerCatalogQueryKey = ["providers", "catalog"];
const integrationTokensQueryKey = ["integration-tokens"];

export function SourcesScreen() {
  const queryClient = useQueryClient();
  const sourcesQuery = useQuery({
    queryKey: sourcesQueryKey,
    queryFn: () => api.getSources(),
  });
  const providersQuery = useQuery({
    queryKey: providerCatalogQueryKey,
    queryFn: async () => {
      const response = await api.getProviderConnectionProviders();
      return response.providers;
    },
  });

  const [addOpen, setAddOpen] = useState(false);
  const [integrationsOpen, setIntegrationsOpen] = useState(false);
  const [selectedSource, setSelectedSource] = useState<Source | null>(null);

  const providers = providersQuery.data ?? [];
  const sources = sourcesQuery.data ?? [];
  const connectedCount = providers.filter((provider) => provider.connected).length;
  const activeCount = sources.filter((source) => isOperational(source.syncState)).length;
  const pendingCount = sources.filter((source) => !source.lastSyncedAt).length;
  const providerCount = new Set(sources.map((source) => source.type)).size;

  return (
    <div className="flex h-full flex-col bg-aladin-canvas">
      <div className="border-b border-aladin-divider px-6 py-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-2">
            <h1 className="text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">
              Live inputs
            </h1>
            <p className="max-w-3xl text-sm leading-6 text-aladin-ink-secondary">
              A workspace-level index of the feeds and searches currently bringing in new material.
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={() => setIntegrationsOpen(true)}>
              Integrations ({connectedCount})
            </Button>
            <Button onClick={() => setAddOpen(true)}>+ Add Stream</Button>
          </div>
        </div>
        <div className="mt-6 grid gap-4 md:grid-cols-4">
          <MetricCard label="Subscribed" value={String(sources.length)} description="sources currently feeding this workspace" />
          <MetricCard label="Healthy" value={String(activeCount)} description="sources reporting a live or active state" />
          <MetricCard label="Pending" value={String(pendingCount)} description="sources still waiting on a first refresh" />
          <MetricCard label="Providers" value={String(providerCount)} description="upstream systems represented here" />
        </div>
      </div>

      <ScrollArea className="min-h-0 flex-1 px-6 py-6">
        {sourcesQuery.isLoading ? (
          <p className="text-sm text-aladin-ink-secondary">Loading sources…</p>
        ) : sources.length === 0 ? (
          <div className="flex items-center justify-between rounded-sharp border border-aladin-border bg-aladin-panel p-5">
            <div className="space-y-1">
              <h2 className="text-xl font-semibold text-aladin-ink">No live streams yet</h2>
              <p className="text-sm leading-6 text-aladin-ink-muted">
                Add a Bluesky search stream to start feeding the global record pipeline.
              </p>
            </div>
            <Button onClick={() => setAddOpen(true)}>+ Add Stream</Button>
          </div>
        ) : (
          <div className="grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
            {sources.map((source) => (
              <button
                key={source.id}
                className="min-h-[188px] rounded-sharp border border-aladin-border bg-aladin-panel p-4 text-left transition-colors hover:bg-aladin-row-hover"
                onClick={() => setSelectedSource(source)}
                type="button"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1.5">
                    <div className="inline-flex items-center gap-2 rounded-full border border-aladin-divider bg-aladin-panel-muted px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-aladin-code-text">
                      {providerLabel(source)}
                    </div>
                    <h3 className="text-lg font-semibold text-aladin-ink">{source.name}</h3>
                    <p className="text-sm leading-6 text-aladin-ink-secondary">
                      {descriptionLine(source)}
                    </p>
                  </div>
                  <FreshnessBadge source={source} />
                </div>
                <div className="mt-4 flex flex-wrap gap-2">
                  <Pill>{healthLabel(source)}</Pill>
                  <Pill>{lastRefreshSummary(source)}</Pill>
                </div>
                <div className="mt-6 inline-flex items-center gap-2 text-sm font-medium text-aladin-code-text">
                  View details
                  <ArrowUpRight className="h-4 w-4" />
                </div>
              </button>
            ))}
          </div>
        )}
      </ScrollArea>

      <AddStreamDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        onCreated={async () => {
          await queryClient.invalidateQueries({ queryKey: sourcesQueryKey });
          setAddOpen(false);
        }}
      />

      <IntegrationsDialog
        open={integrationsOpen}
        onOpenChange={setIntegrationsOpen}
        providers={providers}
        onChanged={async () => {
          await Promise.all([
            queryClient.invalidateQueries({ queryKey: providerCatalogQueryKey }),
            queryClient.invalidateQueries({ queryKey: sourcesQueryKey }),
          ]);
        }}
      />

      <SourceDetailsDialog
        source={selectedSource}
        onOpenChange={(open) => {
          if (!open) setSelectedSource(null);
        }}
        onRemoved={async () => {
          await queryClient.invalidateQueries({ queryKey: sourcesQueryKey });
          setSelectedSource(null);
        }}
      />
    </div>
  );
}

function MetricCard({ label, value, description }: { label: string; value: string; description: string }) {
  return (
    <div className="space-y-1 rounded-sharp border border-aladin-divider bg-aladin-panel px-4 py-4">
      <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-aladin-code-text">
        {label}
      </div>
      <div className="text-2xl font-semibold text-aladin-ink">{value}</div>
      <div className="text-sm leading-6 text-aladin-ink-muted">{description}</div>
    </div>
  );
}

function FreshnessBadge({ source }: { source: Source }) {
  const healthy = isOperational(source.syncState);
  return (
    <Badge variant={healthy ? "default" : "muted"}>
      {healthy ? "Healthy" : titleCase(source.syncState)}
    </Badge>
  );
}

function Pill({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-full border border-aladin-divider bg-aladin-panel-muted px-2.5 py-1 text-xs text-aladin-ink-secondary">
      {children}
    </div>
  );
}

function AddStreamDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => Promise<void>;
}) {
  const [query, setQuery] = useState("");
  const [title, setTitle] = useState("");
  const [limit, setLimit] = useState("25");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: async () => {
      setErrorMessage(null);
      return api.createSource({
        kind: "bluesky.search",
        name: title.trim() || undefined,
        query: query.trim(),
        limit: Number.parseInt(limit, 10) || 25,
      });
    },
    onSuccess: async () => {
      setQuery("");
      setTitle("");
      setLimit("25");
      await onCreated();
    },
    onError: (error) => {
      setErrorMessage(error instanceof Error ? error.message : "Failed to create stream.");
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,640px)]">
        <DialogHeader>
          <DialogTitle>Provider stream subscription</DialogTitle>
          <DialogDescription>
            The current backend supports a Bluesky search stream here. Keep the spike aligned with the existing source contract.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 px-6">
          <div className="space-y-2">
            <label className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
              Provider
            </label>
            <div className="rounded-control border border-aladin-border bg-aladin-panel-muted px-3 py-2 text-sm text-aladin-ink">
              Bluesky search
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
              Query
            </label>
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="e.g. blocknote OR yjs"
            />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                Display title
              </label>
              <Input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Optional custom title"
              />
            </div>
            <div className="space-y-2">
              <label className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                Limit
              </label>
              <Input value={limit} onChange={(event) => setLimit(event.target.value)} />
            </div>
          </div>
          {errorMessage ? (
            <div className="rounded-control border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700">
              {errorMessage}
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={createMutation.isPending || query.trim().length === 0}
            onClick={() => createMutation.mutate()}
          >
            {createMutation.isPending ? "Creating…" : "Create stream"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function IntegrationsDialog({
  open,
  onOpenChange,
  providers,
  onChanged,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers: ProviderConnectionProvider[];
  onChanged: () => Promise<void>;
}) {
  const queryClient = useQueryClient();
  const tokensQuery = useQuery({
    queryKey: integrationTokensQueryKey,
    enabled: open,
    queryFn: async () => {
      const response = await api.getIntegrationTokens();
      return response.tokens;
    },
  });
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null);
  const [tokenName, setTokenName] = useState("Claude Code");
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  const selectedProvider =
    providers.find((provider) => provider.provider === selectedProviderId) ??
    providers.find((provider) => provider.connected) ??
    providers.find((provider) => provider.provider === "google") ??
    providers[0];

  const connectMutation = useMutation({
    mutationFn: async (provider: ProviderConnectionProvider) => api.startProviderConnect(provider.provider),
    onSuccess: (session) => {
      window.open(session.connectLink, "_blank", "noopener,noreferrer");
    },
  });

  const syncMutation = useMutation({
    mutationFn: () => api.syncProviderConnections(),
    onSuccess: onChanged,
  });

  const disconnectMutation = useMutation({
    mutationFn: (connectionId: string) => api.disconnectProviderConnection(connectionId),
    onSuccess: onChanged,
  });

  const tokenCreateMutation = useMutation({
    mutationFn: async () =>
      api.createIntegrationToken({
        name: tokenName.trim(),
        scopes: ["artifacts:read", "artifacts:write"],
      }),
    onSuccess: async (response) => {
      setCreatedToken(response.token);
      await onChanged();
      await queryClient.invalidateQueries({ queryKey: integrationTokensQueryKey });
    },
  });

  const tokenRevokeMutation = useMutation({
    mutationFn: (tokenId: string) => api.revokeIntegrationToken(tokenId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: integrationTokensQueryKey });
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(94vw,1100px)] max-w-none">
        <DialogHeader>
          <DialogTitle>Integrations</DialogTitle>
          <DialogDescription>
            Manage third-party account connections and create MCP bearer tokens for local agents.
          </DialogDescription>
        </DialogHeader>
        <Tabs defaultValue="connected" className="px-6 pb-6">
          <TabsList>
            <TabsTrigger value="connected">Connected Accounts</TabsTrigger>
            <TabsTrigger value="tokens">Agent Access</TabsTrigger>
          </TabsList>
          <TabsContent value="connected" className="mt-4">
            <div className="grid min-h-[520px] gap-0 rounded-sharp border border-aladin-divider bg-aladin-panel lg:grid-cols-[1.2fr_0.8fr]">
              <div className="border-b border-aladin-divider p-5 lg:border-b-0 lg:border-r">
                <div className="mb-4 space-y-1">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-aladin-code-text">
                    Available now
                  </div>
                  <p className="text-sm text-aladin-ink-muted">
                    Configured providers you can connect or manage.
                  </p>
                </div>
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  {providers.map((provider) => (
                    <button
                      key={provider.provider}
                      className={cn(
                        "min-h-[150px] rounded-sharp border p-4 text-left transition-colors",
                        provider.provider === selectedProvider?.provider
                          ? "border-aladin-ink bg-aladin-ink-surface text-aladin-onInkSurface"
                          : provider.available || provider.connected
                            ? "border-aladin-border bg-aladin-panel text-aladin-ink hover:bg-aladin-row-hover"
                            : "border-aladin-divider bg-aladin-panel-muted text-aladin-ink-muted",
                      )}
                      onClick={() => setSelectedProviderId(provider.provider)}
                      type="button"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="space-y-1">
                          <h3 className="text-base font-semibold">{provider.label}</h3>
                          <p className="text-sm leading-6 opacity-80">
                            {provider.description || "Provider connection."}
                          </p>
                        </div>
                        <Badge variant={provider.connected ? "inverted" : "default"}>
                          {provider.connected ? "Connected" : provider.available ? "Available" : "Later"}
                        </Badge>
                      </div>
                      <div className="mt-5 text-[11px] font-semibold uppercase tracking-[0.22em] opacity-70">
                        {provider.category || provider.backend}
                      </div>
                    </button>
                  ))}
                </div>
              </div>

              <div className="p-5">
                {selectedProvider ? (
                  <div className="flex h-full flex-col gap-5">
                    <div className="space-y-2">
                      <h3 className="text-2xl font-semibold tracking-[-0.03em] text-aladin-ink">
                        {selectedProvider.label}
                      </h3>
                      <p className="text-sm leading-6 text-aladin-ink-secondary">
                        {selectedProvider.connected
                          ? "This account is already connected. Use disconnect to remove the local connection reference."
                          : selectedProvider.available
                            ? "Aladin creates a Nango Connect session. Nango owns provider credentials and refresh."
                            : "This provider is visible in the catalog, but no local provider config is enabled yet."}
                      </p>
                    </div>

                    {selectedProvider.capabilities.length > 0 ? (
                      <div className="flex flex-wrap gap-2">
                        {selectedProvider.capabilities.map((capability) => (
                          <Pill key={capability}>{capability}</Pill>
                        ))}
                      </div>
                    ) : null}

                    {selectedProvider.grantedScopes.length > 0 ? (
                      <div className="space-y-2">
                        <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                          Granted scopes
                        </div>
                        <p className="text-sm text-aladin-ink-secondary">
                          {selectedProvider.grantedScopes.join(", ")}
                        </p>
                      </div>
                    ) : null}

                    <div className="mt-auto flex flex-wrap gap-2">
                      {selectedProvider.connected && selectedProvider.connectionId ? (
                        <Button
                          variant="secondary"
                          onClick={() => disconnectMutation.mutate(selectedProvider.connectionId!)}
                          disabled={disconnectMutation.isPending || syncMutation.isPending}
                        >
                          Disconnect
                        </Button>
                      ) : null}
                      {selectedProvider.available && !selectedProvider.connected ? (
                        <>
                          <Button
                            onClick={() => connectMutation.mutate(selectedProvider)}
                            disabled={connectMutation.isPending || syncMutation.isPending}
                          >
                            <Link2 className="h-4 w-4" />
                            Connect with Nango
                          </Button>
                          <Button
                            variant="secondary"
                            onClick={() => syncMutation.mutate()}
                            disabled={connectMutation.isPending || syncMutation.isPending}
                          >
                            <RefreshCcw className="h-4 w-4" />
                            Check connection
                          </Button>
                        </>
                      ) : null}
                    </div>
                  </div>
                ) : (
                  <p className="text-sm text-aladin-ink-muted">No provider catalog available.</p>
                )}
              </div>
            </div>
          </TabsContent>
          <TabsContent value="tokens" className="mt-4">
            <div className="grid min-h-[520px] gap-0 rounded-sharp border border-aladin-divider bg-aladin-panel lg:grid-cols-[1.1fr_0.9fr]">
              <div className="border-b border-aladin-divider p-5 lg:border-b-0 lg:border-r">
                <div className="mb-4 space-y-1">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-aladin-code-text">
                    Agent tokens
                  </div>
                  <p className="text-sm text-aladin-ink-muted">
                    DB-backed bearer tokens for MCP clients and local agents.
                  </p>
                </div>
                <ScrollArea className="h-[420px] pr-4">
                  <div className="space-y-0">
                    {(tokensQuery.data ?? []).length === 0 ? (
                      <div className="rounded-sharp border border-dashed border-aladin-border bg-aladin-panel-muted p-4 text-sm text-aladin-ink-muted">
                        No agent tokens yet.
                      </div>
                    ) : (
                      tokensQuery.data?.map((token) => (
                        <IntegrationTokenRow
                          key={token.id}
                          token={token}
                          revoking={tokenRevokeMutation.isPending}
                          onRevoke={() => tokenRevokeMutation.mutate(token.id)}
                        />
                      ))
                    )}
                  </div>
                </ScrollArea>
              </div>
              <div className="space-y-4 p-5">
                <div className="space-y-1">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-aladin-code-text">
                    MCP setup
                  </div>
                  <p className="text-sm text-aladin-ink-muted">
                    Create a bearer token for the local MCP server.
                  </p>
                </div>
                <div className="rounded-sharp border border-aladin-divider bg-aladin-panel-muted p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                    Endpoint
                  </div>
                  <div className="mt-2 font-mono text-sm text-aladin-code-text">http://localhost:8090/mcp</div>
                </div>
                <div className="rounded-sharp border border-aladin-divider bg-aladin-panel-muted p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                    Recommended scopes
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Pill>artifacts:read</Pill>
                    <Pill>artifacts:write</Pill>
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                    Token name
                  </label>
                  <Input value={tokenName} onChange={(event) => setTokenName(event.target.value)} />
                </div>
                <Button
                  onClick={() => tokenCreateMutation.mutate()}
                  disabled={tokenCreateMutation.isPending || tokenName.trim().length === 0}
                >
                  <ShieldCheck className="h-4 w-4" />
                  {tokenCreateMutation.isPending ? "Creating…" : "Create MCP Token"}
                </Button>
                {createdToken ? (
                  <div className="space-y-3 rounded-sharp border border-aladin-ink bg-aladin-command-surface p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-aladin-code-text">
                      One-time token reveal
                    </div>
                    <div className="text-sm text-aladin-ink-secondary">
                      Copy this now. It will not be shown again.
                    </div>
                    <pre className="overflow-auto rounded-control border border-aladin-divider bg-aladin-panel p-3 text-xs text-aladin-code-text">
                      {createdToken}
                    </pre>
                  </div>
                ) : null}
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function IntegrationTokenRow({
  token,
  revoking,
  onRevoke,
}: {
  token: IntegrationToken;
  revoking: boolean;
  onRevoke: () => void;
}) {
  return (
    <div className="border-b border-aladin-divider py-4 last:border-b-0">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="font-semibold text-aladin-ink">{token.name}</div>
            <Badge variant={token.status === "active" ? "default" : "muted"}>
              {token.status}
            </Badge>
          </div>
          <div className="text-sm text-aladin-ink-secondary">{token.scopes.join(", ") || "No scopes"}</div>
          <div className="text-sm text-aladin-ink-muted">
            Created {formatHumanDate(token.createdAt)} · Last used {formatHumanDate(token.lastUsedAt)} · Expires {formatHumanDate(token.expiresAt)}
          </div>
        </div>
        {token.status === "active" ? (
          <Button variant="secondary" size="sm" disabled={revoking} onClick={onRevoke}>
            Revoke
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function SourceDetailsDialog({
  source,
  onOpenChange,
  onRemoved,
}: {
  source: Source | null;
  onOpenChange: (open: boolean) => void;
  onRemoved: () => Promise<void>;
}) {
  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!source) return;
      return api.deleteSource(source.id);
    },
    onSuccess: onRemoved,
  });

  return (
    <Dialog open={Boolean(source)} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,760px)]">
        {source ? (
          <>
            <DialogHeader>
              <DialogTitle>{source.name}</DialogTitle>
              <DialogDescription>{descriptionLine(source)}</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 px-6">
              <div className="flex flex-wrap gap-2">
                <FreshnessBadge source={source} />
                <Pill>{healthLabel(source)}</Pill>
                <Pill>{lastRefreshSummary(source)}</Pill>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <FactCard label="Provider" value={providerLabel(source)} />
                <FactCard label="Created" value={formatHumanDate(source.createdAt)} />
                <FactCard label="Sync mode" value={titleCase(source.syncMode)} />
                <FactCard label="Last synced" value={formatRelativeTime(source.lastSyncedAt)} />
              </div>
              <div className="rounded-sharp border border-aladin-divider bg-aladin-panel-muted p-4">
                <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                  Config snapshot
                </div>
                <Textarea
                  className="mt-3 min-h-[160px] font-mono text-xs"
                  readOnly
                  value={JSON.stringify(source.config, null, 2)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                variant="destructive"
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
              >
                <Trash2 className="h-4 w-4" />
                Remove source
              </Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function FactCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-sharp border border-aladin-divider bg-aladin-panel p-4">
      <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">{label}</div>
      <div className="mt-2 text-sm text-aladin-ink-secondary">{value}</div>
    </div>
  );
}

function providerLabel(source: Source) {
  const type = source.type.toLowerCase();
  if (type.includes("bluesky")) return "Bluesky";
  if (type.includes("reddit")) return "Reddit";
  if (type.includes("github")) return "GitHub";
  return titleCase(source.type);
}

function descriptionLine(source: Source) {
  if (source.type.toLowerCase().includes("bluesky")) {
    const query = typeof source.config.query === "string" ? source.config.query : null;
    return query ? `Tracks Bluesky results for “${query}”.` : "Tracks a Bluesky search stream.";
  }
  return `${providerLabel(source)} stream`;
}

function healthLabel(source: Source) {
  return isOperational(source.syncState) ? "Healthy" : titleCase(source.syncState);
}

function lastRefreshSummary(source: Source) {
  return source.lastSyncedAt ? formatRelativeTime(source.lastSyncedAt) : "Awaiting first refresh";
}

function isOperational(syncState: string) {
  const state = syncState.toLowerCase();
  return state === "active" || state === "live" || state === "operational" || state === "healthy";
}
