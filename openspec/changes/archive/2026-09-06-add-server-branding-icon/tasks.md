## 1. 图标资产

- [x] 1.1 写 `mcp/internal/assets/generate.py`(Pillow):从 `../../../Tally.png` 去浅色/透明背景 →
  按 alpha 裁剪 + ~8% 边距 → 缩放 512×512 → 圆角矩形遮罩(半径 ≈ 0.22×512,无方形底板)→ 写
  `icon.png`;并导出多尺寸 `favicon.ico`(16/32/48)。脚本顶部集中放阈值/半径常量。
  验证:`python3 mcp/internal/assets/generate.py` 无报错,生成两个文件。
- [x] 1.2 肉眼验收产物:`icon.png` 为 512×512、透明背景、圆角非直角、插画清晰无白边;
  `favicon.ico` 含三个尺寸(`python3 -c "from PIL import Image; im=Image.open('mcp/internal/assets/favicon.ico'); print(im.size, getattr(im,'n_frames',1))"` 或用 `sips`/`file` 检查)。

## 2. assets 包

- [x] 2.1 写 `mcp/internal/assets/assets.go`:`import _ "embed"`;`//go:embed icon.png favicon.ico`
  到 `[]byte`;导出 `Bytes(name string) (data []byte, contentType string, ok bool)`
  (`icon.png`→`image/png`,`favicon.ico`→`image/x-icon`,其它→`ok=false`)。
  验证:`cd mcp && go build ./internal/assets/` 通过。
- [x] 2.2 加 `mcp/internal/assets/assets_test.go`:断言两个已知名字返回非空字节 + 正确
  `Content-Type`,未知名字返回 `ok=false`。验证:`go test ./internal/assets/` 通过。

## 3. serverInfo 品牌字段

- [x] 3.1 `mcp/internal/mcpserver/mcpserver.go`:`New` 签名改为 `New(impl *mcp.Implementation) *mcp.Server`,
  内部 `return mcp.NewServer(impl, nil)`;更新 doc 注释。
  验证:`go build ./internal/mcpserver/` 通过。
- [x] 3.2 `mcp/internal/tools/testutil_test.go:60`:调用改为
  `mcpserver.New(&mcp.Implementation{Name: "tally-mcp-test", Version: "0.0.0"})`,补 import。
  验证:`go build ./... && go vet ./internal/tools/` 通过。
- [x] 3.3 `mcp/cmd/tally-mcp/main.go:buildMux`:`mcpserver.New(...)` 改为传
  `&mcp.Implementation{Name: serverName, Version: serverVersion, Title: "Tally",
  WebsiteURL: "https://github.com/chaoliu719/tally",
  Icons: []mcp.Icon{{Source: cfg.PublicBaseURL + "/icon.png", MIMEType: "image/png", Sizes: []string{"512x512"}}}}`;
  补 `github.com/modelcontextprotocol/go-sdk/mcp` import。
  验证:`go build ./cmd/tally-mcp/` 通过。

## 4. HTTP 路由

- [x] 4.1 `buildMux`(main.go:67 之后、`return mux` 之前):加一个文件内辅助
  `serveAsset(name string) http.HandlerFunc` —— 查 `assets.Bytes`,404 on miss,否则 set
  `Content-Type` + `Cache-Control: public, max-age=86400` 并写字节;注册
  `mux.HandleFunc("/icon.png", serveAsset("icon.png"))` 与
  `mux.HandleFunc("/favicon.ico", serveAsset("favicon.ico"))`。
  验证:`go build ./cmd/tally-mcp/` 通过。

## 5. 测试

- [x] 5.1 `mcp/cmd/tally-mcp/main_test.go`:仿 `TestWellKnownEndpointsAreServedUnauthenticated`
  (main_test.go:229),用 `buildMux` + `httptest.NewServer` + `getStatus` 断言 `/icon.png`、
  `/favicon.ico` 无 `Authorization` 头时返回 200,且各自 `Content-Type` 为 `image/png` /
  `image/x-icon`;并断言不带凭据的 `/mcp` 仍是 401(免认证不外溢)。
  验证:`go test ./cmd/tally-mcp/ -run 'Icon|Unauthenticated'` 通过。
- [x] 5.2 `main_test.go` 的 `TestMCPHandshakeFlow`:对 `InitializeResult().ServerInfo` 加断言 ——
  `Title == "Tally"`、`WebsiteURL` 以 `https://` 开头、`len(Icons) > 0` 且
  `Icons[0].Source` 以 `https://` 开头且以 `/icon.png` 结尾。
  验证:`go test ./cmd/tally-mcp/ -run TestMCPHandshakeFlow` 通过。

## 6. 收尾与验收

- [x] 6.1 全量:`cd mcp && go build ./... && go test ./...` 全绿。
- [x] 6.2 本地跑通:起服务后 `curl` 确认 `/icon.png` 200 + `image/png` +
  `Cache-Control: public, max-age=86400`、`/favicon.ico` 200 + `image/x-icon`、
  `initialize` 的 serverInfo 含 `title: "Tally"` / `websiteUrl` / `icons[0].src =
  https://tally.liuchao.life/icon.png`,且无凭据 `/mcp` 仍 401。
- [x] 6.3 `openspec validate add-server-branding-icon --strict` 通过。
- [ ] 6.4 部署(见 memory: tally-mcp deploy flow):build 镜像推 GHCR `:latest` → `ssh aliyun`
  `cd ~/tally-mcp && docker compose pull && docker compose up -d` → `curl https://tally.liuchao.life/healthz`
  → `curl -s https://tally.liuchao.life/icon.png -o /tmp/icon.png && file /tmp/icon.png` 确认是 PNG。
- [x] 6.5 归档:`openspec-archive-change`,把 delta 同步进
  `openspec/specs/mcp-transport/spec.md`;`openspec list --specs` 无警告。
