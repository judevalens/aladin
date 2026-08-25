/**
 * Liveness plus a line of board-sync occupancy (rooms/sessions), so `curl /healthz`
 * answers "is it up" and "is anyone drawing" in one read. A stats failure must never
 * fail the probe — health is about THIS process accepting requests.
 */
export function healthz(boardSync) {
  return async (_req, res) => {
    let suffix = "";
    try {
      const { rooms, sessions } = await boardSync.stats();
      suffix = ` board-rooms=${rooms} board-sessions=${sessions}`;
    } catch {
      suffix = " board-rooms=?";
    }
    res.type("text/plain").send(`ok${suffix}\n`);
  };
}
