/** Web Crypto works in opaque preview frames where randomUUID may be absent. */
export function resourceRequestId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
}
