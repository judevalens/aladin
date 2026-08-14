import { Check, Copy, Link2, RefreshCcw, ShieldCheck } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
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
import { cn } from "@/lib/utils";
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
    <div className="grid h-full min-h-0 gap-0 overflow-hidden rounded-control border border-line bg-card lg:grid-cols-[1.1fr_0.9fr]">
      <div className="flex min-h-0 flex-col border-b border-line bg-card lg:border-b-0 lg:border-r">
        <div className="border-b border-line px-5 py-5">
          <div className="space-y-1">
            <Eyebrow>Agent tokens</Eyebrow>
            <p className="text-body leading-7 text-ink-3">
              Bearer tokens for local MCP clients and agents.
            </p>
          </div>
        </div>
        <ScrollArea className="h-0 min-h-0 flex-1">
          <div className="space-y-0 px-5 py-5">
            {state.tokens.length === 0 ? (
              <div className="rounded-control border border-dashed border-line bg-card p-4 text-body leading-7 text-ink-3">
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
          <Eyebrow>MCP setup</Eyebrow>
          <h3 className="text-display font-semibold leading-[1.05] tracking-[-0.04em] text-ink">
            Local agent access
          </h3>
          <p className="text-body leading-7 text-ink-2">
            Create and manage bearer tokens for local MCP clients and agents.
          </p>
        </div>
        <div className="border-t border-line pt-4">
          <Eyebrow>Endpoint</Eyebrow>
          <div className="mt-2 font-mono text-body text-ink-2">
            http://localhost:8090/mcp
          </div>
        </div>
        <div className="border-t border-line pt-4">
          <Eyebrow>Recommended scopes</Eyebrow>
          <div className="mt-3 flex flex-wrap gap-2">
            <Pill>artifacts:read</Pill>
            <Pill>artifacts:write</Pill>
          </div>
        </div>
        <div className="space-y-2">
          <Eyebrow as="label">Token name</Eyebrow>
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
          <Icon as={ShieldCheck} mark />
          {state.createTokenPending ? "Creating…" : "Create MCP token"}
          </Button>
        </div>
        {state.createdToken ? (
          <div className="border-t border-line pt-4">
            <div className="flex items-center justify-between gap-2">
              <Eyebrow>One-time token reveal</Eyebrow>
              <Button variant="outline" size="sm" onClick={onCopy}>
                {copied ? (
                  <Icon as={Check} mark />
                ) : (
                  <Icon as={Copy} mark />
                )}
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <div className="mt-2 text-body leading-7 text-ink-2">
              Copy this now. It will not be shown again.
            </div>
            <pre className="mt-3 overflow-auto rounded-control border border-line bg-field p-3 font-mono text-small text-ink-2">
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
    <div className="flex h-full min-h-0 flex-col rounded-control border border-line bg-card">
      <div className="border-b border-line px-5 py-5">
        <div className="space-y-1">
          <Eyebrow>Available now</Eyebrow>
          <p className="text-body leading-7 text-ink-3">
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
                  "flex aspect-square min-h-[176px] min-w-0 flex-col overflow-hidden rounded-control border p-4 text-left transition-colors",
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
                    <h3 className="truncate text-lead font-semibold tracking-[-0.02em]">
                      {provider.label}
                    </h3>
                    <p className="line-clamp-4 overflow-hidden text-body leading-6 opacity-80">
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
                <div className="mt-auto truncate pt-4 text-small opacity-70">
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
                <Eyebrow>Provider detail</Eyebrow>
                <h3 className="text-display font-semibold leading-[1.05] tracking-[-0.04em] text-ink">
                  {state.selectedProvider.label}
                </h3>
                <p className="text-body leading-7 text-ink-2">
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
                  <Eyebrow>Granted scopes</Eyebrow>
                  <p className="mt-2 text-body leading-7 text-ink-2">
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
                      <Icon as={Link2} mark />
                      Connect with Nango
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={state.onSyncProviders}
                      disabled={state.connectPending || state.syncPending}
                    >
                      <Icon as={RefreshCcw} mark />
                      Check connection
                    </Button>
                  </>
                ) : null}
              </div>
            </div>
          ) : (
            <p className="text-body leading-7 text-ink-3">
              No provider catalog available.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
