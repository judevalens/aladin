export function healthz(_req, res) {
  res.type("text/plain").send("ok");
}
