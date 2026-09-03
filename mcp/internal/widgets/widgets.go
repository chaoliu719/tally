// Package widgets serves the HTML for tally-mcp's MCP Apps (interactive UI
// widgets rendered inline by hosts that support the apps surface, e.g.
// claude.ai). Each widget is a single self-contained HTML file under this
// directory with a /*__EXT_APPS_BUNDLE__*/ placeholder; at load time the
// placeholder is replaced with the vendored ext-apps browser runtime rewritten
// to expose globalThis.ExtApps, because the iframe CSP blocks module imports.
//
// This is a deliberate, scoped exception to "tally-mcp is a pure fact core
// that serves no frontend UI" -- see openspec change add-transaction-timeline-
// widget. Widgets are read-only and reach ledger data only through the
// server's own tools via callServerTool.
package widgets

import (
	"fmt"
	"regexp"
	"strings"

	_ "embed"
)

// MIMEType is the mime type a host uses to recognize a resource as an
// interactive MCP App widget rather than plain HTML source.
const MIMEType = "text/html;profile=mcp-app"

// BundlePlaceholder is the token each widget HTML file contains where the
// ext-apps browser runtime is spliced in.
const BundlePlaceholder = "/*__EXT_APPS_BUNDLE__*/"

//go:embed vendor/ext-apps-app-with-deps.js
var extAppsBundle string

//go:embed timeline.html
var timelineHTML string

// exportRewrite matches the trailing `export{a as B,c as D};` of the ES-module
// bundle so it can be turned into a `globalThis.ExtApps={B:a,D:c}` assignment.
var exportRewrite = regexp.MustCompile(`export\{([^}]+)\};?\s*$`)

func browserBundle() string {
	return exportRewrite.ReplaceAllStringFunc(extAppsBundle, func(match string) string {
		inner := exportRewrite.FindStringSubmatch(match)[1]
		var pairs []string
		for _, part := range strings.Split(inner, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			local := part
			exported := part
			if i := strings.Index(part, " as "); i >= 0 {
				local = strings.TrimSpace(part[:i])
				exported = strings.TrimSpace(part[i+4:])
			}
			pairs = append(pairs, exported+":"+local)
		}
		return "globalThis.ExtApps={" + strings.Join(pairs, ",") + "};"
	})
}

// rendered holds each widget's HTML with the bundle already spliced in,
// built once at package init.
var rendered = func() map[string]string {
	bundle := browserBundle()
	splice := func(html string) string {
		return strings.Replace(html, BundlePlaceholder, bundle, 1)
	}
	return map[string]string{
		"timeline": splice(timelineHTML),
	}
}()

// HTML returns the fully rendered HTML for the named widget. The second
// return is false if no widget by that name exists.
func HTML(name string) (string, bool) {
	html, ok := rendered[name]
	return html, ok
}

// previewShim is a stand-in for globalThis.ExtApps used by PreviewHTML: it
// lets a widget be opened in an ordinary browser tab (no host, no iframe
// bridge) for styling work. It fires ontoolresult from a ?payload= query
// param and logs the widget->host calls instead of performing them.
const previewShim = `globalThis.ExtApps={App:class{
  constructor(){this._h={}}
  set ontoolresult(f){this._h.result=f} get ontoolresult(){return this._h.result}
  set ontoolinput(f){this._h.input=f} get ontoolinput(){return this._h.input}
  set onhostcontextchanged(f){this._h.ctx=f} get onhostcontextchanged(){return this._h.ctx}
  _demoBatch(seed,n,cursor){
    const now=Math.floor(Date.now()/1000);
    const notes=["麦当劳麦辣鸡腿堡","便利店 可乐一瓶","超市采购 蔬菜水果","和朋友喝咖啡,备注写得比较长用来测试换行显示效果","",
      "地铁充值","美团外卖","停车费","工资","话费充值","网购 数据线","AWS 账单","药店","水电费","打车"];
    const t=[];
    for(let i=0;i<n;i++){const k=seed+i;
      t.push({id:String(1000-k),type:k%9===0?"income":"expense",source_id:String((k%2)+1),
        category_id:String((k%2)+1),amount:(k%9===0?"8000.00":((k*7%400)+3+".00")),
        currency:k%13===0?"USD":"CNY",time:now-k*7000,comment:notes[k%notes.length]});}
    return{structuredContent:{next_cursor:cursor,transactions:t}};
  }
  async connect(){
    const q=new URLSearchParams(location.search);
    const demo=q.get("payload")||JSON.stringify(Object.assign({total:47},this._demoBatch(0,20,"c1").structuredContent));
    this._page=0;
    if(this._h.input)this._h.input({arguments:{ledger_id:q.get("ledger_id")||"1"}});
    if(this._h.result)this._h.result({content:[{type:"text",text:"preview\n\n"+demo}]});
  }
  getHostContext(){return{theme:new URLSearchParams(location.search).get("theme")||"light",availableDisplayModes:[]}}
  async callServerTool(a){console.log("callServerTool",a);
    if(a&&a.name==="list_categories")return{structuredContent:{categories:[{id:"1",name:"餐饮",parent_id:"0"},{id:"2",name:"外卖",parent_id:"1"}]}};
    if(a&&a.name==="list_sources")return{structuredContent:{sources:[{id:"1",name:"招行储蓄卡"},{id:"2",name:"支付宝"}]}};
    if(a&&a.name==="search_transactions"){this._page=(this._page||0)+1;
      if(this._page===1)return this._demoBatch(20,20,"c2");
      return this._demoBatch(40,7,"");}
    return{structuredContent:{transactions:[],next_cursor:""}};
  }
  sendMessage(m){console.log("sendMessage",m)}
  updateModelContext(m){console.log("updateModelContext",m)}
  requestDisplayMode(m){console.log("requestDisplayMode",m)}
  openLink(l){console.log("openLink",l)}
}};`

// PreviewHTML returns the named widget wired to previewShim instead of the
// real ext-apps runtime, for opening in a plain browser tab.
func PreviewHTML(name string) (string, bool) {
	html, ok := map[string]string{"timeline": timelineHTML}[name]
	if !ok {
		return "", false
	}
	return strings.Replace(html, BundlePlaceholder, previewShim, 1), true
}

// URI returns the ui:// resource URI for the named widget.
func URI(name string) string {
	return fmt.Sprintf("ui://widgets/%s.html", name)
}
