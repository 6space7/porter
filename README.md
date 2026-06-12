# porter

porter is an open source, self-hosted deployment platform for running AI-built apps on a VPS without requiring the user to understand Docker, reverse proxies, or TLS.

The product is built phase by phase from the project brief:

1. Phase 0: repository and GitHub setup.
2. Phase 1: core API and deployment engine for one server.
3. Phase 2: embedded Svelte web UI.
4. Phase 3: deployment history and rollback.
5. Phase 4: service catalog and Nixpacks.
6. Phase 5: MCP server and AI-agent workflow.
7. Phase 6: multi-server deploys, GitHub auto-deploy, and release lifecycle.

## Principles

- The JSON API is the product.
- Everything must be machine-legible for AI agents.
- Secure defaults come first.
- Deployment detection happens on every deploy.
- A production install should be one binary and one command.

## Status

Phase 0 is in progress.
