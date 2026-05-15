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
  DialogBody,
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
    <div className="flex h-full flex-col bg-white">
      <div className="border-b border-gray-300 px-6 py-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-2">
            <h1 className="text-2xl font-semibold text-black">
              Sources
            </h1>
            <p className="max-w-3xl text-sm leading-6 text-gray-700">
              Streams and connected services that feed new material into this workspace.
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
          <p className="text-sm text-gray-700">Loading sources…</p>
        ) : sources.length === 0 ? (
          <div className="flex items-center justify-between border border-gray-300 bg-white p-5">
            <div className="space-y-1">
              <h2 className="text-xl font-semibold text-black">No live streams yet</h2>
              <p className="text-sm leading-6 text-gray-500">
                Add a source to start bringing new material into the workspace.
              </p>
            </div>
            <Button onClick={() => setAddOpen(true)}>+ Add Stream</Button>
          </div>
        ) : (
          <div className="grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
            {sources.map((source) => (
              <button
                key={source.id}
                className="min-h-[188px] border border-gray-300 bg-white p-4 text-left transition-colors hover:bg-gray-100"
                onClick={() => setSelectedSource(source)}
                type="button"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1.5">
                    <div className="inline-flex items-center gap-2 border border-gray-300 bg-gray-50 px-2.5 py-1 text-xs text-gray-500">
                      {providerLabel(source)}
                    </div>
                    <h3 className="text-lg font-semibold text-black">{source.name}</h3>
                    <p className="text-sm leading-6 text-gray-700">
                      {descriptionLine(source)}
                    </p>
                  </div>
                  <FreshnessBadge source={source} />
                </div>
                <div className="mt-4 flex flex-wrap gap-2">
                  <Pill>{healthLabel(source)}</Pill>
                  <Pill>{lastRefreshSummary(source)}</Pill>
                </div>
                <div className="mt-6 inline-flex items-center gap-2 text-sm font-medium text-gray-600">
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
    <div className="space-y-1 border border-gray-300 bg-white px-4 py-4">
      <div className="text-xs text-gray-500">
        {label}
      </div>
      <div className="text-2xl font-semibold text-black">{value}</div>
      <div className="text-sm leading-6 text-gray-500">{description}</div>
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
    <div className="border border-gray-300 bg-white px-2.5 py-1 text-xs text-gray-700">
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
          <DialogTitle>Add source</DialogTitle>
          <DialogDescription>Create a new search stream for this workspace.</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4 pb-4">
          <div className="space-y-2">
            <label className="text-xs text-gray-500">
              Provider
            </label>
            <div className="border border-gray-300 bg-white px-3 py-2 text-sm text-black">
              Bluesky search
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-xs text-gray-500">
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
              <label className="text-xs text-gray-500">
                Display title
              </label>
              <Input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Optional custom title"
              />
            </div>
            <div className="space-y-2">
              <label className="text-xs text-gray-500">
                Limit
              </label>
              <Input value={limit} onChange={(event) => setLimit(event.target.value)} />
            </div>
          </div>
          {errorMessage ? (
            <div className="border border-gray-300 bg-white px-3 py-2 text-sm text-black">
              {errorMessage}
            </div>
          ) : null}
        </DialogBody>
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
            Manage connected accounts and local agent access.
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="pb-0">
          <Tabs defaultValue="connected" className="flex min-h-0 flex-1 flex-col pb-6">
          <TabsList>
            <TabsTrigger value="connected">Connected Accounts</TabsTrigger>
            <TabsTrigger value="tokens">Agent Access</TabsTrigger>
          </TabsList>
          <TabsContent value="connected" className="mt-4">
            <div className="grid min-h-[520px] gap-0 border border-gray-300 bg-white lg:grid-cols-[1.2fr_0.8fr]">
              <div className="border-b border-gray-300 p-5 lg:border-b-0 lg:border-r">
                <div className="mb-4 space-y-1">
                  <div className="text-xs text-gray-500">
                    Available now
                  </div>
                  <p className="text-sm text-gray-500">
                    Configured providers you can connect or manage.
                  </p>
                </div>
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  {providers.map((provider) => (
                    <button
                      key={provider.provider}
                      className={cn(
                        "min-h-[150px] border p-4 text-left transition-colors",
                        provider.provider === selectedProvider?.provider
                          ? "border-black bg-black text-white"
                          : provider.available || provider.connected
                            ? "border-gray-300 bg-white text-black hover:bg-gray-100"
                            : "border-gray-300 bg-gray-50 text-gray-500",
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
                      <div className="mt-5 text-xs text-gray-500">
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
                      <h3 className="text-2xl font-semibold text-black">
                        {selectedProvider.label}
                      </h3>
                      <p className="text-sm leading-6 text-gray-700">
                        {selectedProvider.connected
                          ? "This account is already connected."
                          : selectedProvider.available
                            ? "Start a new connection for this provider."
                            : "This provider is visible here, but not available yet."}
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
                        <div className="text-xs text-gray-500">
                          Granted scopes
                        </div>
                        <p className="text-sm text-gray-700">
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
                  <p className="text-sm text-gray-500">No provider catalog available.</p>
                )}
              </div>
            </div>
          </TabsContent>
          <TabsContent value="tokens" className="mt-4">
            <div className="grid min-h-[520px] gap-0 border border-gray-300 bg-white lg:grid-cols-[1.1fr_0.9fr]">
              <div className="border-b border-gray-300 p-5 lg:border-b-0 lg:border-r">
                <div className="mb-4 space-y-1">
                  <div className="text-xs text-gray-500">
                    Agent tokens
                  </div>
                  <p className="text-sm text-gray-500">
                    Bearer tokens for local MCP clients and agents.
                  </p>
                </div>
                <ScrollArea className="h-[420px] pr-4">
                  <div className="space-y-0">
                    {(tokensQuery.data ?? []).length === 0 ? (
                      <div className="border border-dashed border-gray-300 bg-white p-4 text-sm text-gray-500">
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
                  <div className="text-xs text-gray-500">
                    MCP setup
                  </div>
                  <p className="text-sm text-gray-500">
                    Create a token for the local MCP server.
                  </p>
                </div>
                <div className="border border-gray-300 bg-white p-4">
                  <div className="text-xs text-gray-500">
                    Endpoint
                  </div>
                  <div className="mt-2 font-mono text-sm text-gray-600">http://localhost:8090/mcp</div>
                </div>
                <div className="border border-gray-300 bg-white p-4">
                  <div className="text-xs text-gray-500">
                    Recommended scopes
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Pill>artifacts:read</Pill>
                    <Pill>artifacts:write</Pill>
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-xs text-gray-500">
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
                  <div className="space-y-3 border border-gray-300 bg-white p-4">
                    <div className="text-xs text-gray-500">
                      One-time token reveal
                    </div>
                    <div className="text-sm text-gray-700">
                      Copy this now. It will not be shown again.
                    </div>
                    <pre className="overflow-auto border border-gray-300 bg-white p-3 text-xs text-gray-600">
                      {createdToken}
                    </pre>
                  </div>
                ) : null}
              </div>
            </div>
          </TabsContent>
          </Tabs>
        </DialogBody>
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
    <div className="border-b border-gray-300 py-4 last:border-b-0">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="font-semibold text-black">{token.name}</div>
            <Badge variant={token.status === "active" ? "default" : "muted"}>
              {token.status}
            </Badge>
          </div>
          <div className="text-sm text-gray-700">{token.scopes.join(", ") || "No scopes"}</div>
          <div className="text-sm text-gray-500">
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
            <DialogBody className="space-y-4 pb-4">
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
              <div className="border border-gray-300 bg-white p-4">
                <div className="text-xs text-gray-500">
                  Config snapshot
                </div>
                <Textarea
                  className="mt-3 min-h-[160px] font-mono text-xs"
                  readOnly
                  value={JSON.stringify(source.config, null, 2)}
                />
              </div>
            </DialogBody>
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
    <div className="border border-gray-300 bg-white p-4">
      <div className="text-xs text-gray-500">{label}</div>
      <div className="mt-2 text-sm text-gray-700">{value}</div>
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
