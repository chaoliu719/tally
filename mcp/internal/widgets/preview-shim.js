// Generic fake host for opening a widget in an ordinary browser tab (no real
// host, no iframe bridge) during styling / behaviour work. It implements
// every App method the shared runtime touches, reads ?theme= / ?ledger_id= /
// ?payload= from the URL, fires ontoolinput then ontoolresult, and delegates
// all data to the per-widget globalThis.TallyPreviewFixture (set by that
// widget's src/<name>/preview.js). A widget with no fixture still renders,
// just with empty payloads.
globalThis.ExtApps = { App: class TallyPreviewHost {
  constructor() { this._h = {}; }
  set ontoolresult(f) { this._h.result = f; } get ontoolresult() { return this._h.result; }
  set ontoolinput(f) { this._h.input = f; } get ontoolinput() { return this._h.input; }
  set onhostcontextchanged(f) { this._h.ctx = f; } get onhostcontextchanged() { return this._h.ctx; }

  _fixture() { return globalThis.TallyPreviewFixture; }

  async connect() {
    const q = new URLSearchParams(location.search);
    const fx = this._fixture();
    const payload = (fx && fx.firstPage) ? fx.firstPage() : { transactions: [] };
    if (this._h.input) this._h.input({ arguments: { ledger_id: q.get("ledger_id") || "1" } });
    if (this._h.result) {
      this._h.result({ content: [{ type: "text", text: "preview\n\n" + JSON.stringify(payload) }] });
    }
  }

  getHostContext() {
    return {
      theme: new URLSearchParams(location.search).get("theme") || "light",
      availableDisplayModes: [],
    };
  }

  async callServerTool(a) {
    console.log("callServerTool", a);
    const fx = this._fixture();
    if (fx && fx.callServerTool) return fx.callServerTool(a, this);
    return { structuredContent: { transactions: [], next_cursor: "" } };
  }

  sendMessage(m) { console.log("sendMessage", m); }
  updateModelContext(m) { console.log("updateModelContext", m); }
  requestDisplayMode(m) { console.log("requestDisplayMode", m); }
  openLink(l) { console.log("openLink", l); }
}};
