# porter frontend

Svelte 5 + Vite dashboard for Porter Phase 2.

```bash
npm install
npm run check
npm run build
```

The production build writes `frontend/dist`. That directory is committed because
the Go binary embeds it with `go:embed`, so `go build ./cmd/server` produces a
single binary with the UI included.
