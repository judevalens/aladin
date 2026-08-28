// Transport liveness is separate from model progress: quiet reasoning, tools,
// and approval holds may legitimately outlast the Go client's idle watchdog.
export function createEventStream(res, { heartbeatMs = 15_000 } = {}) {
  let closed = false;
  const writeEvent = (event) => {
    if (closed) return;
    if (res.writableEnded || res.destroyed) {
      dispose();
      return;
    }
    res.write(`${JSON.stringify(event)}\n`);
    if (event.type === "done") dispose();
  };
  const timer = setInterval(() => writeEvent({ type: "heartbeat" }), heartbeatMs);
  timer.unref();

  function dispose() {
    if (closed) return;
    closed = true;
    clearInterval(timer);
    res.off("close", dispose);
    res.off("finish", dispose);
  }
  res.once("close", dispose);
  res.once("finish", dispose);
  return { writeEvent, dispose };
}
