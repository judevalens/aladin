import { Trash2 } from "lucide-react";
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
    <div className="bg-white px-5 py-4">
      <div className="eyebrow">{label}</div>
      <div className="mt-2 text-[1.5rem] font-semibold leading-tight tracking-[-0.02em] text-[#0a0a0a] tabular-nums">
        {value}
      </div>
      <div className="mt-1 text-[12px] leading-[1.45] text-[#78716c]">
        {description}
      </div>
    </div>
  );
}

export function Pill({ children }: { children: ReactNode }) {
  return (
    <div className="inline-flex items-center rounded-full border border-[#e7e5e4] bg-[#fafaf9] px-2 py-0.5 text-[11px] text-[#57534e]">
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
    <div className="border-t border-[#e7e5e4] py-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1.5">
          <div className="flex items-center gap-2">
            <div className="font-semibold tracking-[-0.02em] text-[#0a0a0a]">
              {token.name}
            </div>
            <Badge>{token.status}</Badge>
          </div>
          <div className="text-sm leading-7 text-[#44403c]">
            {token.scopes.join(", ") || "No scopes"}
          </div>
          <div className="text-sm leading-7 text-[#78716c]">
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
    <div className="border-t border-[#e7e5e4] p-4">
      <div className="eyebrow">{label}</div>
      <div className="mt-2 text-sm leading-7 text-[#44403c]">{value}</div>
    </div>
  );
}
