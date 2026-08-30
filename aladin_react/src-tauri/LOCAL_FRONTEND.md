# Bundled Local Frontend

Desktop bundles start an in-process HTTP server for the embedded Vite build. They
do not run Vite, serve a working directory, or change `NODE_ENV`. `tauri dev`
continues to use its configured development server; mobile builds retain the
custom protocol.

- `com.aladin.app` serves on `http://127.0.0.1:4174`.
- `com.aladin.react` bundles serve on `http://127.0.0.1:4175`.
- Other app identities get a deterministic port in 45000-54999.
- The server reserves the port before opening the window. An occupied port fails
  startup; it never loads content from another process or silently changes origin.
- API, collaboration, and board-sync URLs remain the existing build-time settings.
  `make prod-app` still builds and installs against 8080, 3511, and 3512.

The bootstrap window reads `aladin.*` localStorage entries at the previous custom
protocol origin and passes them through the native bridge to the main window's
initialization script. No credentials enter HTTP responses, URLs, logs, or new
files. The destination imports only missing values and records completion, so a
later logout cannot restore the old session. Original storage is left untouched;
IndexedDB caches are not migrated. Server-backed boards and notes resynchronize.

Native commands are declared in `permissions/desktop.json`. The HTTP capability
is granted only to the `main` window at the exact app origin, never arbitrary
localhost ports or shard origins. HTTP requests reject unexpected Host/Origin
headers and cross-site fetches, and responses disallow framing. The local server
is a desktop testing deployment choice, not a change to dependency license terms.

## Verification

With the installed app open, run from `aladin_react`:

```sh
ALADIN_BUNDLED_FRONTEND_URL=http://127.0.0.1:4174 npm run e2e -- local-frontend.spec.ts
```

This uses the running bundle's server, not Vite. It checks HTTP protections and
that the actual board component remains rendered beyond five seconds. Native
permissions and login continuity also need checking in the desktop app; an
ordinary browser intentionally has no Tauri bridge.
