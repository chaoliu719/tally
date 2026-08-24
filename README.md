# tally

A personal bookkeeping capability, packaged for an agent to carry: **tally = tally-mcp + plugin**.

- [`mcp/`](mcp/README.md) — the fact layer. A pure [MCP](https://modelcontextprotocol.io) server
  backed by its own SQLite schema: ledgers, sources, categories, transactions. No REST API, no web
  UI, no third-party bookkeeping library underneath.
- `plugin/` — the practice layer. The skills, commands, agents, and hooks that turn the raw MCP
  tools into a bookkeeping workflow inside a conversation. Skeleton only for now; content lands as
  it's built.

One-line boundary: tally tracks what your money became, not what kind of person you are. Your
money is a file you can take with you (a single SQLite database).

See [`openspec/`](openspec/) for the full product vision and the spec-driven change process used
to evolve both layers.
