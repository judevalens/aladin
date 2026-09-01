import { useEffect, useRef, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { ShardChannel, ShardPublication } from "@/app/state/shard-build-slice";
import type { ShardReleaseMetadata } from "../bridge/resource-host-hub";

// Seed from the protected release endpoint. Later publications carry their
// complete identity over the workspace event stream; a build is not a release.
export function useShardRelease(pageId: string, channel: ShardChannel, buildId = "", publication?: ShardPublication) {
  const { runtime } = useAppComposition();
  const initialPublication = useRef(publication);
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{ runtime: typeof runtime; key: string; value?: ShardReleaseMetadata; error?: boolean }>();
  // Legacy builds activate mutable dist directly and have no protected release
  // event. Preserve their build signal without treating resource staging as live.
  const buildKey = channel === "draft" || result?.value?.protocol === "bridge/1" ? buildId : "";
  const key = JSON.stringify([pageId, channel, buildKey, publication?.eventId, attempt]);
  useEffect(() => {
    let alive = true;
    // Cached events from before mount cannot replace the authoritative seed.
    if (publication && publication !== initialPublication.current && attempt === 0) {
      setResult({ runtime, key, value: publication });
      return;
    }
    void runtime.apis.shardResources.release(pageId, channel).then(value => {
      if (alive) setResult({ runtime, key, value });
    }).catch(() => {
      if (alive) setResult({ runtime, key, error: true });
    });
    return () => { alive = false; };
  }, [runtime, pageId, channel, key, publication, attempt]);
  const settled = result?.runtime === runtime && result?.key === key;
  const value = settled ? result.value : undefined;
  return {
    value,
    error: settled && result.error === true,
    available: value !== undefined && (value.protocol === "bridge/2" || value.available !== false),
    retry: () => setAttempt(value => value + 1),
  };
}
