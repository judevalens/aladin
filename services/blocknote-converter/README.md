## Endpoints

- `POST /md-to-blocks` `{markdown: string}` → `{blocks: Block[]}`
- `POST /blocks-to-md` `{blocks: Block[]}` → `{markdown: string}`
- `POST /blocks-to-md-batch` `{blocks: Block[][]}` → `{markdowns: string[]}`
- `GET  /healthz` → `ok`

## Local dev without Docker

```
cd services/blocknote-converter
npm install
node server.js   # listens on :3500 by default
```
