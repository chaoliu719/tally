// Demo data for opening the transaction timeline widget in a plain browser
// tab via the preview harness. Stateful (page counter) because the timeline
// preview needs to simulate paging through search_transactions.
//
// ?scope= picks what the panel is "opened with":
//   (none)    no scope key at all, like a server that predates scoping
//   all       a scope object with nothing set (the whole ledger)
//   range     start_date / end_date only (September 2026)
//   category  range + a category (with subcategories)
//   source    range + a source
//   both      range + category + source
//   empty     a range with no transactions in it
// ?payload= still overrides the whole first page with raw JSON.
globalThis.TallyPreviewFixture = (function () {
  let page = 0;
  const pad = (n) => String(n).padStart(2, "0");
  // k-th newest demo transaction, as the naive local date-time string the
  // server returns (no timezone), stepping back ~2h per row from 2026-09-30.
  function timeOf(k) {
    const d = new Date(2026, 8, 30, 23, 0, 0);
    d.setMinutes(d.getMinutes() - k * 127);
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:00`;
  }
  function demoBatch(seed, n, cursor) {
    const notes = ["麦当劳麦辣鸡腿堡", "便利店 可乐一瓶", "超市采购 蔬菜水果", "和朋友喝咖啡,备注写得比较长用来测试换行显示效果", "",
      "地铁充值", "美团外卖", "停车费", "工资", "话费充值", "网购 数据线", "AWS 账单", "药店", "水电费", "打车"];
    const t = [];
    for (let i = 0; i < n; i++) {
      const k = seed + i;
      t.push({
        id: String(1000 - k), type: k % 9 === 0 ? "income" : "expense", source_id: String((k % 2) + 1),
        category_id: String((k % 2) + 1), amount: (k % 9 === 0 ? "8000.00" : ((k * 7 % 400) + 3 + ".00")),
        currency: k % 13 === 0 ? "USD" : "CNY", time: timeOf(k), comment: notes[k % notes.length],
      });
    }
    return { structuredContent: { next_cursor: cursor, transactions: t } };
  }
  const range = { start_date: "2026-09-01", end_date: "2026-09-30", start_time: "2026-09-01 00:00:00", end_time: "2026-09-30 23:59:59" };
  const scopes = {
    all: {},
    range,
    category: Object.assign({}, range, { category_id: "1", include_descendants: true }),
    source: Object.assign({}, range, { source_id: "2" }),
    both: Object.assign({}, range, { category_id: "1", include_descendants: true, source_id: "2" }),
    empty: { start_date: "2027-01-01", end_date: "2027-01-31", start_time: "2027-01-01 00:00:00", end_time: "2027-01-31 23:59:59" },
  };
  const lookups = {
    categories: [{ id: "1", name: "餐饮", parent_id: "0" }, { id: "2", name: "外卖", parent_id: "1" }],
    sources: [{ id: "1", name: "招行储蓄卡" }, { id: "2", name: "支付宝" }],
  };
  return {
    firstPage() {
      const q = new URLSearchParams(location.search);
      if (q.get("payload")) return JSON.parse(q.get("payload"));
      const which = q.get("scope");
      const first = which === "empty"
        ? { total: 0, transactions: [], next_cursor: "" }
        : Object.assign({ total: 47 }, demoBatch(0, 20, "c1").structuredContent);
      const out = Object.assign(first, lookups);
      if (which && scopes[which]) out.scope = scopes[which];
      return out;
    },
    callServerTool(a) {
      if (a && a.name === "list_categories") return { structuredContent: { categories: lookups.categories } };
      if (a && a.name === "list_sources") return { structuredContent: { sources: lookups.sources } };
      if (a && a.name === "search_transactions") {
        console.log("search_transactions args " + JSON.stringify(a.arguments));
        page = (page || 0) + 1;
        if (page === 1) return demoBatch(20, 20, "c2");
        return demoBatch(40, 7, "");
      }
      return { structuredContent: { transactions: [], next_cursor: "" } };
    },
  };
})();
