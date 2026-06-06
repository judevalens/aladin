import { Check, Copy, Link2, RefreshCcw, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useIntegrationsDialogState,
  IntegrationsState,
} from "@/modules/sources/hooks/use-integrations-dialog-state";
import { cn } from "@/shared/lib/utils";
import {
  IntegrationTokenRow,
  Pill,
} from "@/modules/sources/ui/sources-parts-ui";
import type { IntegrationsDialogProps } from "@/modules/sources/ui/sources-ui-types";

export function IntegrationsDialog(props: IntegrationsDialogProps) {
  const { open, onOpenChange } = props;
  const state = useIntegrationsDialogState({ open });

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        state.onOpenChange(nextOpen);
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent className="h-[min(82vh,760px)] w-[min(94vw,1100px)] max-w-none">
        <DialogHeader>
          <DialogTitle>Integrations</DialogTitle>
          <DialogDescription>
            Manage connected accounts and local agent access.
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="flex min-h-0 flex-1 flex-col overflow-hidden pb-0">
          <Tabs
            defaultValue="connected"
            className="flex min-h-0 flex-1 flex-col gap-3"
          >
            <TabsList className="h-auto p-2">
              <TabsTrigger value="connected">Connected accounts</TabsTrigger>
              <TabsTrigger value="tokens">Agent access</TabsTrigger>
            </TabsList>
            <TabsContent
              value="connected"
              className="mt-0 min-h-0 flex-1 overflow-hidden"
            >
              <IntegrationTabs state={state} />
            </TabsContent>
            <TabsContent
              value="tokens"
              className="mt-0 min-h-0 flex-1 overflow-hidden"
            >
              <AgentAccessTabs state={state} />
            </TabsContent>
          </Tabs>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

function AgentAccessTabs({ state }: { state: IntegrationsState }) {
  const [copied, setCopied] = useState(false);
  const onCopy = async () => {
    if (!state.createdToken) return;
    try {
      await navigator.clipboard.writeText(state.createdToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard unavailable (e.g. no permission) — the token is still
      // selectable in the box below as a fallback.
    }
  };
  return (
    <div className="grid h-full min-h-0 gap-0 overflow-hidden rounded-lg border border-line bg-card lg:grid-cols-[1.1fr_0.9fr]">
      <div className="flex min-h-0 flex-col border-b border-line bg-card lg:border-b-0 lg:border-r">
        <div className="border-b border-line px-5 py-5">
          <div className="space-y-1">
            <div className="eyebrow">Agent tokens</div>
            <p className="text-sm leading-7 text-ink-3">
              Bearer tokens for local MCP clients and agents.
            </p>
          </div>
        </div>
        <ScrollArea className="h-0 min-h-0 flex-1">
          <div className="space-y-0 px-5 py-5">
            {state.tokens.length === 0 ? (
              <div className="rounded-md border border-dashed border-line bg-card p-4 text-sm leading-7 text-ink-3">
                No agent tokens yet.
              </div>
            ) : (
              state.tokens.map((token) => (
                <IntegrationTokenRow
                  key={token.id}
                  token={token}
                  revoking={state.revokeTokenPending}
                  onRevoke={() => state.onRevokeToken(token.id)}
                />
              ))
            )}
          </div>
        </ScrollArea>
      </div>
      <div className="flex min-h-0 flex-col bg-card p-5">
        <div className="space-y-2">
          <div className="eyebrow">MCP setup</div>
          <h3 className="text-[1.8rem] font-semibold leading-[1.05] tracking-[-0.04em] text-ink">
            Local agent access
          </h3>
          <p className="text-sm leading-7 text-ink-2">
            Create and manage bearer tokens for local MCP clients and agents.
          </p>
        </div>
        <div className="border-t border-line pt-4">
          <div className="eyebrow">Endpoint</div>
          <div className="mt-2 font-mono text-sm text-ink-2">
            http://localhost:8090/mcp
          </div>
        </div>
        <div className="border-t border-line pt-4">
          <div className="eyebrow">Recommended scopes</div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Pill>artifacts:read</Pill>
            <Pill>artifacts:write</Pill>
          </div>
        </div>
        <div className="space-y-2">
          <label className="eyebrow">Token name</label>
          <Input
            value={state.tokenName}
            onChange={(event) => state.onTokenNameChange(event.target.value)}
          />
        <Button
          onClick={state.onCreateToken}
          disabled={
            state.createTokenPending || state.tokenName.trim().length === 0
          }
        >
          <ShieldCheck className="h-4 w-4" />
          {state.createTokenPending ? "Creating…" : "Create MCP token"}
          </Button>
        </div>
        {state.createdToken ? (
          <div className="border-t border-line pt-4">
            <div className="flex items-center justify-between gap-2">
              <div className="eyebrow">One-time token reveal</div>
              <Button variant="outline" size="sm" onClick={onCopy}>
                {copied ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <div className="mt-2 text-sm leading-7 text-ink-2">
              Copy this now. It will not be shown again.
            </div>
            <pre className="mt-3 overflow-auto rounded-md border border-line bg-field p-3 font-mono text-xs text-ink-2">
              {state.createdToken}
            </pre>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function IntegrationTabs({ state }: { state: IntegrationsState }) {
  return (
    <div className="flex h-full min-h-0 flex-col rounded-lg border border-line bg-card">
      <div className="border-b border-line px-5 py-5">
        <div className="space-y-1">
          <div className="eyebrow">Available now</div>
          <p className="text-sm leading-7 text-ink-3">
            Configured providers you can connect or manage.
          </p>
        </div>
      </div>
      <div className="flex min-h-0 flex-1 w-full">
        <div className="min-h-0 flex-1 overflow-y-auto border-r border-line">
          <div className="grid gap-3 p-5 sm:grid-cols-2 xl:grid-cols-3">
            {state.providers.map((provider) => (
              <button
                key={provider.provider}
                className={cn(
                  "flex aspect-square min-h-[176px] min-w-0 flex-col overflow-hidden rounded-md border p-4 text-left transition-colors",
                  provider.provider === state.selectedProvider?.provider
                    ? "border-[rgb(var(--sel))] bg-[rgb(var(--sel))] text-ink"
                    : provider.available || provider.connected
                      ? "border-line bg-card text-ink hover:bg-raise"
                      : "border-line bg-card text-ink-4",
                )}
                onClick={() => state.onSelectProvider(provider.provider)}
                type="button"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1 space-y-1 overflow-hidden">
                    <h3 className="truncate text-base font-semibold tracking-[-0.02em]">
                      {provider.label}
                    </h3>
                    <p className="line-clamp-4 overflow-hidden text-sm leading-6 opacity-80">
                      {provider.description || "Provider connection."}
                    </p>
                  </div>
                  <Badge
                    variant={
                      provider.provider === state.selectedProvider?.provider
                        ? "inverted"
                        : "default"
                    }
                  >
                    {provider.connected
                      ? "Connected"
                      : provider.available
                        ? "Available"
                        : "Later"}
                  </Badge>
                </div>
                <div className="mt-auto truncate pt-4 text-xs opacity-70">
                  {provider.category || provider.backend}
                </div>
              </button>
            ))}
          </div>
        </div>
        <div className="flex w-[340px] shrink-0 flex-col bg-card p-5">
          {state.selectedProvider ? (
            <div className="flex h-full flex-col gap-5">
              <div className="space-y-2">
                <div className="eyebrow">Provider detail</div>
                <h3 className="text-[1.8rem] font-semibold leading-[1.05] tracking-[-0.04em] text-ink">
                  {state.selectedProvider.label}
                </h3>
                <p className="text-sm leading-7 text-ink-2">
                  {state.selectedProvider.connected
                    ? "This account is already connected."
                    : state.selectedProvider.available
                      ? "Start a new connection for this provider."
                      : "This provider is visible here, but not available yet."}
                </p>
              </div>

              {state.selectedProvider.capabilities.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {state.selectedProvider.capabilities.map((capability) => (
                    <Pill key={capability}>{capability}</Pill>
                  ))}
                </div>
              ) : null}

              {state.selectedProvider.grantedScopes.length > 0 ? (
                <div className="border-t border-line pt-4">
                  <div className="eyebrow">Granted scopes</div>
                  <p className="mt-2 text-sm leading-7 text-ink-2">
                    {state.selectedProvider.grantedScopes.join(", ")}
                  </p>
                </div>
              ) : null}

              <div className="mt-auto flex flex-wrap gap-2">
                {state.selectedProvider.connected &&
                state.selectedProvider.connectionId ? (
                  <Button
                    variant="secondary"
                    onClick={() =>
                      state.onDisconnectProvider(
                        state.selectedProvider!.connectionId!,
                      )
                    }
                    disabled={state.disconnectPending || state.syncPending}
                  >
                    Disconnect
                  </Button>
                ) : null}
                {state.selectedProvider.available &&
                !state.selectedProvider.connected ? (
                  <>
                    <Button
                      onClick={() =>
                        state.onConnectProvider(
                          state.selectedProvider!.provider,
                        )
                      }
                      disabled={state.connectPending || state.syncPending}
                    >
                      <Link2 className="h-4 w-4" />
                      Connect with Nango
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={state.onSyncProviders}
                      disabled={state.connectPending || state.syncPending}
                    >
                      <RefreshCcw className="h-4 w-4" />
                      Check connection
                    </Button>
                  </>
                ) : null}
              </div>
            </div>
          ) : (
            <p className="text-sm leading-7 text-ink-3">
              No provider catalog available.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
