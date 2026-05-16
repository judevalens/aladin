import {
  ArrowUpRight,
  Link2,
  RefreshCcw,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import type { ReactNode } from "react";
import { PlaceholderPane } from "@/components/ui/aladin";
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
import type { IntegrationToken, ProviderConnectionProvider, Source } from "@/shared/api/models";
import { cn, formatHumanDate } from "@/shared/lib/utils";

export function SourcesRouteView({
  children,
}: {
  children: ReactNode;
}) {
  return (
    <>
      <section className="flex w-[352px] flex-col overflow-hidden border-r border-[#e4e4e4] bg-white">
        <PlaceholderPane
          title="Sources"
          body="Connections, streams, and agent access for this workspace."
          className="h-full"
        />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-white">{children}</section>
    </>
  );
}

export function SourcesScreenView({
  loading,
  metrics,
  sources,
  providers,
  tokens,
  selectedSource,
  selectedProvider,
  connectedCount,
  addOpen,
  integrationsOpen,
  streamQuery,
  streamTitle,
  streamLimit,
  streamErrorMessage,
  tokenName,
  createdToken,
  connectPending,
  syncPending,
  disconnectPending,
  createSourcePending,
  createTokenPending,
  revokeTokenPending,
  removeSourcePending,
  onOpenAddStream,
  onOpenIntegrations,
  onAddOpenChange,
  onIntegrationsOpenChange,
  onSelectSource,
  onSelectProvider,
  onStreamQueryChange,
  onStreamTitleChange,
  onStreamLimitChange,
  onTokenNameChange,
  onCreateSource,
  onConnectProvider,
  onSyncProviders,
  onDisconnectProvider,
  onCreateToken,
  onRevokeToken,
  onRemoveSelectedSource,
  providerLabel,
  descriptionLine,
  healthLabel,
  lastRefreshSummary,
  formatSourceFacts,
}: {
  loading: boolean;
  metrics: Array<{ label: string; value: string; description: string }>;
  sources: Source[];
  providers: ProviderConnectionProvider[];
  tokens: IntegrationToken[];
  selectedSource: Source | null;
  selectedProvider: ProviderConnectionProvider | undefined;
  connectedCount: number;
  addOpen: boolean;
  integrationsOpen: boolean;
  streamQuery: string;
  streamTitle: string;
  streamLimit: string;
  streamErrorMessage: string | null;
  tokenName: string;
  createdToken: string | null;
  connectPending: boolean;
  syncPending: boolean;
  disconnectPending: boolean;
  createSourcePending: boolean;
  createTokenPending: boolean;
  revokeTokenPending: boolean;
  removeSourcePending: boolean;
  onOpenAddStream: () => void;
  onOpenIntegrations: () => void;
  onAddOpenChange: (open: boolean) => void;
  onIntegrationsOpenChange: (open: boolean) => void;
  onSelectSource: (sourceId: string | null) => void;
  onSelectProvider: (providerId: string | null) => void;
  onStreamQueryChange: (value: string) => void;
  onStreamTitleChange: (value: string) => void;
  onStreamLimitChange: (value: string) => void;
  onTokenNameChange: (value: string) => void;
  onCreateSource: () => void;
  onConnectProvider: (providerId: string) => void;
  onSyncProviders: () => void;
  onDisconnectProvider: (connectionId: string) => void;
  onCreateToken: () => void;
  onRevokeToken: (tokenId: string) => void;
  onRemoveSelectedSource: () => void;
  providerLabel: (source: Source) => string;
  descriptionLine: (source: Source) => string;
  healthLabel: (source: Source) => string;
  lastRefreshSummary: (source: Source) => string;
  formatSourceFacts: (source: Source) => Array<{ label: string; value: string }>;
}) {
  return (
    <div className="flex h-full flex-col bg-white">
      <div className="border-b border-[#e4e4e4] bg-white px-6 py-6">
        <div className="flex flex-wrap items-start justify-between gap-5">
          <div className="space-y-3">
            <div className="eyebrow">Workspace ingestion</div>
            <h1 className="max-w-3xl text-[2.3rem] font-semibold leading-[1.04] tracking-[-0.05em] text-[#111111]">
              Keep fresh material flowing into the workspace.
            </h1>
            <p className="section-copy max-w-3xl">
              Manage live search streams, connected providers, and local agent access from one calm control surface.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={onOpenIntegrations}>
              Integrations ({connectedCount})
            </Button>
            <Button onClick={onOpenAddStream}>Add stream</Button>
          </div>
        </div>
        <div className="mt-6 grid gap-0 border-t border-[#e4e4e4] md:grid-cols-2 xl:grid-cols-4">
          {metrics.map((metric) => (
            <MetricCard
              key={metric.label}
              label={metric.label}
              value={metric.value}
              description={metric.description}
            />
          ))}
        </div>
      </div>

      <ScrollArea className="min-h-0 flex-1">
        {loading ? (
          <p className="px-6 py-6 text-sm leading-7 text-[#3f3f46]">Loading sources…</p>
        ) : sources.length === 0 ? (
          <div className="flex flex-col items-start justify-between gap-5 border-t border-[#e4e4e4] p-6 md:flex-row md:items-center">
            <div className="space-y-2">
              <div className="eyebrow">No live streams yet</div>
              <h2 className="text-[1.65rem] font-semibold tracking-[-0.04em] text-[#111111]">
                Bring the first stream into this workspace.
              </h2>
              <p className="max-w-2xl text-sm leading-7 text-[#52525b]">
                Add a source to start pulling new material into the workspace and keep retrieval grounded in fresh context.
              </p>
            </div>
            <Button onClick={onOpenAddStream}>Add stream</Button>
          </div>
        ) : (
          <div className="grid gap-0 xl:grid-cols-2 2xl:grid-cols-3">
            {sources.map((source) => (
              <button
                key={source.id}
                className="min-h-[220px] border-t border-[#e4e4e4] p-5 text-left transition-colors hover:bg-[#f3f3f3]"
                onClick={() => onSelectSource(source.id)}
                type="button"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-3">
                    <div className="inline-flex items-center gap-2 rounded-full border border-[#e4e4e4] bg-white px-3 py-1 text-xs text-[#6b7280]">
                      {providerLabel(source)}
                    </div>
                    <div className="space-y-2">
                      <h3 className="text-[1.25rem] font-semibold tracking-[-0.03em] text-[#111111]">{source.name}</h3>
                      <p className="text-sm leading-7 text-[#3f3f46]">{descriptionLine(source)}</p>
                    </div>
                  </div>
                  <Badge>{healthLabel(source)}</Badge>
                </div>
                <div className="mt-5 flex flex-wrap gap-2">
                  <Pill>{healthLabel(source)}</Pill>
                  <Pill>{lastRefreshSummary(source)}</Pill>
                </div>
                <div className="mt-8 inline-flex items-center gap-2 text-sm font-medium text-[#52525b]">
                  View details
                  <ArrowUpRight className="h-4 w-4" />
                </div>
              </button>
            ))}
          </div>
        )}
      </ScrollArea>

      <Dialog open={addOpen} onOpenChange={onAddOpenChange}>
        <DialogContent className="w-[min(92vw,640px)]">
          <DialogHeader>
            <DialogTitle>Add source</DialogTitle>
            <DialogDescription>Create a new search stream for this workspace.</DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-5 pb-4">
            <div className="space-y-2">
              <label className="eyebrow">Provider</label>
              <div className="rounded-[12px] border border-[#e7e5e0] bg-white px-4 py-3 text-sm text-[#111111]">
                Bluesky search
              </div>
            </div>
            <div className="space-y-2">
              <label className="eyebrow">Query</label>
              <Input
                value={streamQuery}
                onChange={(event) => onStreamQueryChange(event.target.value)}
                placeholder="e.g. blocknote OR yjs"
              />
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="eyebrow">Display title</label>
                <Input
                  value={streamTitle}
                  onChange={(event) => onStreamTitleChange(event.target.value)}
                  placeholder="Optional custom title"
                />
              </div>
              <div className="space-y-2">
                <label className="eyebrow">Limit</label>
                <Input value={streamLimit} onChange={(event) => onStreamLimitChange(event.target.value)} />
              </div>
            </div>
            {streamErrorMessage ? (
              <div className="rounded-[12px] border border-[#e7e5e0] bg-white px-4 py-3 text-sm leading-6 text-[#111111]">
                {streamErrorMessage}
              </div>
            ) : null}
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => onAddOpenChange(false)}>
              Cancel
            </Button>
            <Button
              disabled={createSourcePending || streamQuery.trim().length === 0}
              onClick={onCreateSource}
            >
              {createSourcePending ? "Creating…" : "Create stream"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={integrationsOpen} onOpenChange={onIntegrationsOpenChange}>
        <DialogContent className="w-[min(94vw,1100px)] max-w-none">
          <DialogHeader>
            <DialogTitle>Integrations</DialogTitle>
            <DialogDescription>Manage connected accounts and local agent access.</DialogDescription>
          </DialogHeader>
          <DialogBody className="pb-0">
            <Tabs defaultValue="connected" className="flex min-h-0 flex-1 flex-col pb-6">
              <TabsList>
                <TabsTrigger value="connected">Connected accounts</TabsTrigger>
                <TabsTrigger value="tokens">Agent access</TabsTrigger>
              </TabsList>
              <TabsContent value="connected" className="mt-4">
                <div className="grid min-h-[520px] gap-0 overflow-hidden rounded-[16px] border border-[#e4e4e4] bg-white lg:grid-cols-[1.2fr_0.8fr]">
                  <div className="border-b border-[#e4e4e4] bg-white p-5 lg:border-b-0 lg:border-r">
                    <div className="mb-4 space-y-1">
                      <div className="eyebrow">Available now</div>
                      <p className="text-sm leading-7 text-[#6b7280]">
                        Configured providers you can connect or manage.
                      </p>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {providers.map((provider) => (
                        <button
                          key={provider.provider}
                          className={cn(
                            "rounded-[14px] border p-4 text-left transition-colors",
                            provider.provider === selectedProvider?.provider
                                ? "border-[#ededed] bg-[#ededed] text-[#111111]"
                                : provider.available || provider.connected
                                ? "border-[#e4e4e4] bg-white text-[#111111] hover:bg-[#f3f3f3]"
                                : "border-[#e4e4e4] bg-white text-[#8b8b92]",
                          )}
                          onClick={() => onSelectProvider(provider.provider)}
                          type="button"
                        >
                          <div className="flex items-start justify-between gap-3">
                            <div className="space-y-1">
                              <h3 className="text-base font-semibold tracking-[-0.02em]">{provider.label}</h3>
                              <p className="text-sm leading-6 opacity-80">
                                {provider.description || "Provider connection."}
                              </p>
                            </div>
                            <Badge variant={provider.provider === selectedProvider?.provider ? "inverted" : "default"}>
                              {provider.connected ? "Connected" : provider.available ? "Available" : "Later"}
                            </Badge>
                          </div>
                          <div className="mt-5 text-xs opacity-70">{provider.category || provider.backend}</div>
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="bg-white p-5">
                    {selectedProvider ? (
                      <div className="flex h-full flex-col gap-5">
                        <div className="space-y-2">
                          <div className="eyebrow">Provider detail</div>
                          <h3 className="text-[1.8rem] font-semibold leading-[1.05] tracking-[-0.04em] text-[#111111]">
                            {selectedProvider.label}
                          </h3>
                          <p className="text-sm leading-7 text-[#3f3f46]">
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
                        <div className="border-t border-[#e4e4e4] pt-4">
                            <div className="eyebrow">Granted scopes</div>
                            <p className="mt-2 text-sm leading-7 text-[#3f3f46]">
                              {selectedProvider.grantedScopes.join(", ")}
                            </p>
                          </div>
                        ) : null}

                        <div className="mt-auto flex flex-wrap gap-2">
                          {selectedProvider.connected && selectedProvider.connectionId ? (
                            <Button
                              variant="secondary"
                              onClick={() => onDisconnectProvider(selectedProvider.connectionId!)}
                              disabled={disconnectPending || syncPending}
                            >
                              Disconnect
                            </Button>
                          ) : null}
                          {selectedProvider.available && !selectedProvider.connected ? (
                            <>
                              <Button
                                onClick={() => onConnectProvider(selectedProvider.provider)}
                                disabled={connectPending || syncPending}
                              >
                                <Link2 className="h-4 w-4" />
                                Connect with Nango
                              </Button>
                              <Button
                                variant="secondary"
                                onClick={onSyncProviders}
                                disabled={connectPending || syncPending}
                              >
                                <RefreshCcw className="h-4 w-4" />
                                Check connection
                              </Button>
                            </>
                          ) : null}
                        </div>
                      </div>
                    ) : (
                      <p className="text-sm leading-7 text-[#6b7280]">No provider catalog available.</p>
                    )}
                  </div>
                </div>
              </TabsContent>
              <TabsContent value="tokens" className="mt-4">
                <div className="grid min-h-[520px] gap-0 overflow-hidden rounded-[16px] border border-[#e4e4e4] bg-white lg:grid-cols-[1.1fr_0.9fr]">
                  <div className="border-b border-[#e4e4e4] bg-white p-5 lg:border-b-0 lg:border-r">
                    <div className="mb-4 space-y-1">
                      <div className="eyebrow">Agent tokens</div>
                      <p className="text-sm leading-7 text-[#6b7280]">
                        Bearer tokens for local MCP clients and agents.
                      </p>
                    </div>
                    <ScrollArea className="h-[420px] pr-4">
                      <div className="space-y-3">
                        {tokens.length === 0 ? (
                          <div className="rounded-[14px] border border-dashed border-[#e4e4e4] bg-white p-4 text-sm leading-7 text-[#6b7280]">
                            No agent tokens yet.
                          </div>
                        ) : (
                          tokens.map((token) => (
                            <IntegrationTokenRow
                              key={token.id}
                              token={token}
                              revoking={revokeTokenPending}
                              onRevoke={() => onRevokeToken(token.id)}
                            />
                          ))
                        )}
                      </div>
                    </ScrollArea>
                  </div>
                  <div className="space-y-4 bg-white p-5">
                    <div className="space-y-1">
                      <div className="eyebrow">MCP setup</div>
                      <p className="text-sm leading-7 text-[#6b7280]">
                        Create a token for the local MCP server.
                      </p>
                    </div>
                    <div className="border-t border-[#e4e4e4] pt-4">
                      <div className="eyebrow">Endpoint</div>
                      <div className="mt-2 font-mono text-sm text-[#3f3f46]">http://localhost:8090/mcp</div>
                    </div>
                    <div className="border-t border-[#e4e4e4] pt-4">
                      <div className="eyebrow">Recommended scopes</div>
                      <div className="mt-3 flex flex-wrap gap-2">
                        <Pill>artifacts:read</Pill>
                        <Pill>artifacts:write</Pill>
                      </div>
                    </div>
                    <div className="space-y-2">
                      <label className="eyebrow">Token name</label>
                      <Input value={tokenName} onChange={(event) => onTokenNameChange(event.target.value)} />
                    </div>
                    <Button
                      onClick={onCreateToken}
                      disabled={createTokenPending || tokenName.trim().length === 0}
                    >
                      <ShieldCheck className="h-4 w-4" />
                      {createTokenPending ? "Creating…" : "Create MCP token"}
                    </Button>
                    {createdToken ? (
                      <div className="border-t border-[#e4e4e4] pt-4">
                        <div className="eyebrow">One-time token reveal</div>
                        <div className="mt-2 text-sm leading-7 text-[#3f3f46]">
                          Copy this now. It will not be shown again.
                        </div>
                        <pre className="mt-3 overflow-auto rounded-[12px] border border-[#e7e5e0] bg-white p-3 text-xs text-[#52525b]">
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

      <Dialog open={Boolean(selectedSource)} onOpenChange={(open) => !open && onSelectSource(null)}>
        <DialogContent className="w-[min(92vw,760px)]">
          {selectedSource ? (
            <>
              <DialogHeader>
                <DialogTitle>{selectedSource.name}</DialogTitle>
                <DialogDescription>{descriptionLine(selectedSource)}</DialogDescription>
              </DialogHeader>
              <DialogBody className="space-y-5 pb-4">
                <div className="flex flex-wrap gap-2">
                  <Badge>{healthLabel(selectedSource)}</Badge>
                  <Pill>{healthLabel(selectedSource)}</Pill>
                  <Pill>{lastRefreshSummary(selectedSource)}</Pill>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  {formatSourceFacts(selectedSource).map((fact) => (
                    <FactCard key={fact.label} label={fact.label} value={fact.value} />
                  ))}
                </div>
                <div className="border-t border-[#e4e4e4] pt-4">
                  <div className="eyebrow">Config snapshot</div>
                  <Textarea
                    className="mt-3 min-h-[180px] font-mono text-xs"
                    readOnly
                    value={JSON.stringify(selectedSource.config, null, 2)}
                  />
                </div>
              </DialogBody>
              <DialogFooter>
                <Button variant="destructive" onClick={onRemoveSelectedSource} disabled={removeSourcePending}>
                  <Trash2 className="h-4 w-4" />
                  Remove source
                </Button>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function MetricCard({ label, value, description }: { label: string; value: string; description: string }) {
  return (
    <div className="metric-card border-t-0 border-l-0 px-5 py-5 md:[&:nth-child(2n)]:border-r-0 xl:[&:nth-child(4n)]:border-r-0">
      <div className="eyebrow">{label}</div>
      <div className="mt-3 text-[2rem] font-semibold leading-none tracking-[-0.04em] text-[#111111]">{value}</div>
      <div className="mt-3 text-sm leading-7 text-[#6b7280]">{description}</div>
    </div>
  );
}

function Pill({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-full border border-[#e4e4e4] bg-white px-3 py-1 text-xs text-[#52525b]">{children}</div>
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
    <div className="border-t border-[#e4e4e4] py-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1.5">
          <div className="flex items-center gap-2">
            <div className="font-semibold tracking-[-0.02em] text-[#111111]">{token.name}</div>
            <Badge>{token.status}</Badge>
          </div>
          <div className="text-sm leading-7 text-[#3f3f46]">{token.scopes.join(", ") || "No scopes"}</div>
          <div className="text-sm leading-7 text-[#6b7280]">
            Created {formatHumanDate(token.createdAt)} · Last used {formatHumanDate(token.lastUsedAt)} · Expires{" "}
            {formatHumanDate(token.expiresAt)}
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

function FactCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-t border-[#e4e4e4] p-4">
      <div className="eyebrow">{label}</div>
      <div className="mt-2 text-sm leading-7 text-[#3f3f46]">{value}</div>
    </div>
  );
}
