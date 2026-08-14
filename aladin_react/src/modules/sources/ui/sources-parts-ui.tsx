import { Trash2 } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatHumanDate } from "@/shared/lib/utils";
import type { IntegrationToken } from "@/shared/api/models";

export function MetricCard({
  label,
  value,
  description,
}: {
  label: string;
  value: string;
  description: string;
}) {
  return (
    <div className="bg-card px-5 py-4">
      <Eyebrow>{label}</Eyebrow>
      <div className="mt-2 font-display text-title font-semibold leading-tight tracking-[-0.02em] text-ink tabular-nums">
        {value}
      </div>
      <div className="mt-1 text-small leading-[1.45] text-ink-3">
        {description}
      </div>
    </div>
  );
}

export function Pill({ children }: { children: ReactNode }) {
  return (
    <div className="inline-flex items-center rounded-full border border-line bg-field px-2 py-0.5 text-meta text-ink-2">
      {children}
    </div>
  );
}

export function IntegrationTokenRow({
  token,
  revoking,
  onRevoke,
}: {
  token: IntegrationToken;
  revoking: boolean;
  onRevoke: () => void;
}) {
  return (
    <div className="border-t border-line py-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1.5">
          <div className="flex items-center gap-2">
            <div className="font-semibold tracking-[-0.02em] text-ink">
              {token.name}
            </div>
            <Badge>{token.status}</Badge>
          </div>
          <div className="text-body leading-7 text-ink-2">
            {token.scopes.join(", ") || "No scopes"}
          </div>
          <div className="text-body leading-7 text-ink-3">
            Created {formatHumanDate(token.createdAt)} · Last used{" "}
            {formatHumanDate(token.lastUsedAt)} · Expires{" "}
            {formatHumanDate(token.expiresAt)}
          </div>
        </div>
        {token.status === "active" ? (
          <Button
            variant="secondary"
            size="sm"
            disabled={revoking}
            onClick={onRevoke}
          >
            Revoke
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function FactCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-t border-line p-4">
      <Eyebrow>{label}</Eyebrow>
      <div className="mt-2 text-body leading-7 text-ink-2">{value}</div>
    </div>
  );
}
