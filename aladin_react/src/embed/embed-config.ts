/**
 * What a native host injects before the embedded editor loads, and the one declaration of
 * the global that carries it.
 *
 * Two entry points read it — the bundled single-file host (`editor-entry.tsx`, what the iPad
 * actually ships) and the `/embed/page/:id` route it grew out of — and both used to declare
 * `window.__ALADIN_EMBED__` themselves. Two declarations of one global agree right up until
 * one of them gains a field, so the shape lives here instead.
 *
 * **Configuration comes from the host, not the URL.** The native side sets this in a
 * document-start user script; a bearer token in a query string would leak into history, logs
 * and any referrer.
 */
export interface EmbedConfig {
  token: string;
  /** Hocuspocus, e.g. `ws://192.168.1.109:3501` — a device cannot use localhost. */
  collabWsUrl: string;
  user: { name: string; color: string };
  /**
   * The Go API's origin, e.g. `http://192.168.1.109:8000`. Shards only — a note needs
   * nothing but the collab socket, which is why this stayed absent until shards landed.
   */
  apiBaseUrl?: string;
  /**
   * The board sync room server's base, e.g. `ws://192.168.1.109:3502`. Boards refuse to
   * mount without it (a local-only board on a companion device would silently fork).
   */
  boardSyncWsUrl?: string;
}

declare global {
  interface Window {
    __ALADIN_EMBED__?: EmbedConfig;
  }
}
