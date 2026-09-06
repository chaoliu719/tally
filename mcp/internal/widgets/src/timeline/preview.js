// Demo data for opening the transaction timeline widget in a plain browser
// tab via the preview harness. Stateful (page counter) because the timeline
// preview needs to simulate paging through search_transactions.
globalThis.TallyPreviewFixture = (function () {
  let page = 0;
  function demoBatch(seed, n, cursor) {
    const now = Math.floor(Date.now() / 1000);
    const notes = ["麦当劳麦辣鸡腿堡", "便利店 可乐一瓶", "超市采购 蔬菜水果", "和朋友喝咖啡,备注写得比较长用来测试换行显示效果", "",
      "地铁充值", "美团外卖", "停车费", "工资", "话费充值", "网购 数据线", "AWS 账单", "药店", "水电费", "打车"];
    const t = [];
    for (let i = 0; i < n; i++) {
      const k = seed + i;
      t.push({
        id: String(1000 - k), type: k % 9 === 0 ? "income" : "expense", source_id: String((k % 2) + 1),
        category_id: String((k % 2) + 1), amount: (k % 9 === 0 ? "8000.00" : ((k * 7 % 400) + 3 + ".00")),
        currency: k % 13 === 0 ? "USD" : "CNY", time: now - k * 7000, comment: notes[k % notes.length],
      });
    }
    return { structuredContent: { next_cursor: cursor, transactions: t } };
  }
  return {
    firstPage() {
      const q = new URLSearchParams(location.search);
      if (q.get("payload")) return JSON.parse(q.get("payload"));
      return Object.assign({ total: 47 }, demoBatch(0, 20, "c1").structuredContent);
    },
    callServerTool(a) {
      if (a && a.name === "list_categories") {
        return { structuredContent: { categories: [{ id: "1", name: "餐饮", parent_id: "0" }, { id: "2", name: "外卖", parent_id: "1" }] } };
      }
      if (a && a.name === "list_sources") {
        return { structuredContent: { sources: [{ id: "1", name: "招行储蓄卡" }, { id: "2", name: "支付宝" }] } };
      }
      if (a && a.name === "search_transactions") {
        page = (page || 0) + 1;
        if (page === 1) return demoBatch(20, 20, "c2");
        return demoBatch(40, 7, "");
      }
      return { structuredContent: { transactions: [], next_cursor: "" } };
    },
  };
})();
