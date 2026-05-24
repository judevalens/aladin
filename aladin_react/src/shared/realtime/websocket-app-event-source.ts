import type { AppEventEnvelope } from "@/shared/realtime/app-event";

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30_000;

export interface SubscriptionRequest {
  stream: string;
  resourceKind: string;
  resourceId: string;
}

export interface WebSocketAppEventSourceOptions {
  url: string;
  token: () => string | null;
  subscriptions: SubscriptionRequest[];
  onEvent: (event: AppEventEnvelope) => void;
  onConnectionChange?: (state: "connecting" | "open" | "closed") => void;
}

export interface WebSocketAppEventSource {
  start(): void;
  stop(): void;
}

export function createWebSocketAppEventSource(
  options: WebSocketAppEventSourceOptions,
): WebSocketAppEventSource {
  let socket: WebSocket | null = null;
  let stopped = false;
  let reconnectAttempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  function emitConnectionState(state: "connecting" | "open" | "closed") {
    options.onConnectionChange?.(state);
  }

  function buildUrl() {
    const token = options.token();
    if (!token) {
      return options.url;
    }
    const sep = options.url.includes("?") ? "&" : "?";
    return `${options.url}${sep}access_token=${encodeURIComponent(token)}`;
  }

  function scheduleReconnect() {
    if (stopped) return;
    const delay = Math.min(
      RECONNECT_MAX_MS,
      RECONNECT_BASE_MS * Math.pow(2, reconnectAttempt),
    );
    reconnectAttempt += 1;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  }

  function connect() {
    if (stopped) return;
    emitConnectionState("connecting");
    try {
      socket = new WebSocket(buildUrl());
    } catch {
      scheduleReconnect();
      return;
    }

    socket.onopen = () => {
      reconnectAttempt = 0;
      emitConnectionState("open");
      socket?.send(
        JSON.stringify({
          type: "subscribe",
          subscriptions: options.subscriptions,
        }),
      );
    };

    socket.onmessage = (message) => {
      let parsed: { type?: string; event?: AppEventEnvelope } | null = null;
      try {
        parsed = JSON.parse(message.data as string);
      } catch {
        return;
      }
      if (parsed?.type === "event" && parsed.event) {
        options.onEvent(parsed.event);
      }
    };

    socket.onclose = () => {
      emitConnectionState("closed");
      socket = null;
      scheduleReconnect();
    };

    socket.onerror = () => {
      try {
        socket?.close();
      } catch {
        // ignore
      }
    };
  }

  return {
    start() {
      stopped = false;
      connect();
    },
    stop() {
      stopped = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      try {
        socket?.close();
      } catch {
        // ignore
      }
      socket = null;
      emitConnectionState("closed");
    },
  };
}
