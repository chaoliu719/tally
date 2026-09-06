globalThis.Tally = (function () {
"use strict";

// withTimeout rejects if `promise` has not settled within `ms`.
function withTimeout(promise, ms) {
  return Promise.race([
    promise,
    new Promise((_, reject) => setTimeout(() => reject(new Error("timeout")), ms)),
  ]);
}

// parseSearchResult pulls { transactions, next_cursor } out of a
// search_transactions tool result -- structuredContent when present,
// otherwise the JSON in the first text block.
function parseSearchResult(res) {
  if (res && res.structuredContent && Array.isArray(res.structuredContent.transactions)) {
    return res.structuredContent;
  }
  const text = res && res.content && res.content[0] && res.content[0].text;
  if (text) {
    try { return JSON.parse(text); } catch { /* fall through */ }
  }
  return { transactions: [], next_cursor: "" };
}

// The launcher tool result can reach a widget a few different ways depending
// on the host: as structuredContent, as a text block that is
// "<summary>\n\n<json>", or as bare JSON. Try all of them.
function extractFirstPagePayload(result) {
  const sc = result && result.structuredContent;
  if (sc && (Array.isArray(sc.transactions) || typeof sc.total === "number")) return sc;
  const content = result && result.content;
  const text = content && content[0] && content[0].text;
  if (typeof text === "string") {
    const i = text.indexOf("\n\n");
    const tries = i >= 0 ? [text.slice(i + 2), text] : [text];
    const m = text.match(/\{[\s\S]*\}/);
    if (m) tries.push(m[0]);
    for (const c of tries) {
      try {
        const o = JSON.parse(c);
        if (o && (Array.isArray(o.transactions) || typeof o.total === "number")) return o;
      } catch { /* next candidate */ }
    }
  }
  return { transactions: [] };
}

// mountWidget owns everything host-shaped: the App object, the bridge
// callbacks, theme, the fullscreen button, and supersession. The widget
// supplies onToolInput / onToolResult and keeps all of its own domain state.
//
// It is synchronous and installs the bridge callbacks immediately, so the
// returned api (including api.app) is available before api.connect() is
// called -- some hosts deliver the first tool result synchronously during
// connect(), and the widget's onToolResult must be able to reach api.app by
// then. The widget calls `await api.connect()` once its own wiring is done.
//
// Returns { app, connect, isSuperseded, extractFirstPagePayload,
// parseSearchResult, withTimeout }.
function mountWidget(opts) {
  const { App } = globalThis.ExtApps;

  const mainEl = document.getElementById("main");
  const bootEl = document.getElementById("boot");
  const expandBtn = document.getElementById("expand");
  function boot(msg) { if (bootEl) bootEl.textContent = msg; }
  function fatal(msg) { if (mainEl) mainEl.innerHTML = '<div class="empty">' + msg + "</div>"; }

  const app = new App(
    { name: opts.name, version: opts.version },
    {},
    { autoResize: true },
  );

  // --- supersession: a newer instance of this widget makes older ones stop
  // fetching, so a stale panel does not keep hammering the server. ---
  let superseded = false;
  const seq = Date.now() + Math.random();
  if (opts.channel) {
    try {
      const bc = new BroadcastChannel(opts.channel);
      bc.onmessage = (e) => { if (e.data && e.data.seq > seq) superseded = true; };
      bc.postMessage({ seq });
    } catch { /* BroadcastChannel unavailable — non-fatal */ }
  }

  // --- theme ---
  // The host fires onhostcontextchanged for display-mode changes too (entering
  // / leaving fullscreen), and that context can arrive without a `theme`
  // field. Treat only "light" / "dark" as authoritative and ignore anything
  // else, so a partial context never silently drops us back to light.
  function applyTheme(theme) {
    if (theme !== "light" && theme !== "dark") return;
    document.documentElement.classList.toggle("dark", theme === "dark");
  }

  // The "全屏" button only makes sense while inline: show it when the host
  // offers a fullscreen mode and we are not already in it, hide it otherwise
  // (the host provides its own way back out of fullscreen).
  function syncDisplayMode(ctx) {
    if (!expandBtn) return;
    const canFullscreen = !!ctx && Array.isArray(ctx.availableDisplayModes)
      && ctx.availableDisplayModes.includes("fullscreen");
    const inFullscreen = !!ctx && (ctx.displayMode === "fullscreen" || ctx.mode === "fullscreen");
    expandBtn.hidden = !canFullscreen || inFullscreen;
  }

  app.onhostcontextchanged = (ctx) => { if (ctx) { applyTheme(ctx.theme); syncDisplayMode(ctx); } };
  if (opts.onToolInput) app.ontoolinput = opts.onToolInput;
  if (opts.onToolResult) app.ontoolresult = opts.onToolResult;

  async function connect() {
    boot("连接中…");
    try {
      await app.connect();
    } catch (err) {
      fatal("连接宿主失败:" + (err && err.message ? err.message : err));
      throw err;
    }
    boot("已连接,等待数据…");
    applyTheme(app.getHostContext()?.theme);

    if (expandBtn) {
      expandBtn.addEventListener("click", () => app.requestDisplayMode({ mode: "fullscreen" }));
    }
    syncDisplayMode(app.getHostContext());
  }

  return {
    app,
    connect,
    isSuperseded: () => superseded,
    extractFirstPagePayload,
    parseSearchResult,
    withTimeout,
  };
}

return { mountWidget, withTimeout, parseSearchResult, extractFirstPagePayload };
})();
