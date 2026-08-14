import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// `globals` is off in vitest.config.ts, so testing-library's own auto-cleanup — which hooks the
// GLOBAL afterEach — never registers. Single-render test files never noticed. Anything that
// portals into document.body does: the previous test's overlay is still mounted when the next
// one queries, and getByRole finds two of everything.
afterEach(cleanup);
