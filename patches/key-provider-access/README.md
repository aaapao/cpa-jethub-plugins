# key-provider-access v0.0.5 本地补丁

上游 `key-provider-access` 用于按下游 Key 限制可访问的上游渠道（用法见 `docs/DEPLOYMENT.md` 第 3.6 节）。上游 `v0.0.5` 有两处缺陷，本目录的补丁修掉它们，构建产物版本标识记为 `0.0.5-cpamp-v3`。

| # | 缺陷 | 表现 | 补丁 |
| --- | --- | --- | --- |
| 1 | 配置页只识别 `enc::v1::` 会话 | CPAMC 自 2026-09-17 起改写为 `enc::v2::`，页面恒提示"未找到可复用的 CPAMC 会话" | `web/settings.js` 的 `decodeStoredValue` 增加 v2 分支（密钥材料 = 盐 + `v2` + 主机名），保留 v1 与明文两条旧路径 |
| 2 | 调度插件默认只收到最高优先级档的候选 | Key 允许的渠道处于低档、被拒绝的渠道占更高档时，请求返回 403 `no allowed upstream profile is available for this API key` | `capabilities.scheduler_across_priorities: true`，取得全档候选；`Pick` 在策略过滤后的允许集合内自行按"最高档 + 档内轮询"选择，与 CPA 原生档位语义一致 |

补丁文件 `v0.0.5-cpamp-v3.patch` 基于上游 `v0.0.5` tag，共 6 个文件。

## 构建

```bash
git clone --depth 50 https://github.com/LTbinglingfeng/key-model-access.git kpa-src
cd kpa-src
git checkout v0.0.5
git apply /path/to/v0.0.5-cpamp-v3.patch

export PATH=/path/to/go/bin:$PATH CGO_ENABLED=1
VERSION=0.0.5-cpamp-v3 make build     # 产物 dist/key-provider-access.so
```

`go test ./...` 应通过。

`make build` 默认带 `-buildvcs`，Go 会把当前 checkout 的 VCS 修订写进二进制，同一个补丁在不同 checkout 上构建出的 `.so` 哈希不同。需要可复现的哈希时改用：

```bash
CGO_ENABLED=1 go build -trimpath -buildvcs=false -buildmode=c-shared \
  -ldflags "-s -w -X main.pluginVersion=0.0.5-cpamp-v3" -o dist/key-provider-access.so .
```

该命令在任意干净的 `v0.0.5` + 本补丁上产出 sha256 `ac0a1cdb7975218802e8750616312d9c02c28c3050f69422afc9fdd213ac6a02`（5 606 632 字节），已实测两个独立 checkout 结果一致。构建工具链 Go 1.26.8。

## 安装

把构建出的 `dist/key-provider-access.so` 放到 CPA 插件目录，文件名保持 `key-provider-access-v0.0.5.so`（插件按文件名的 `-v<版本>` 段识别加载版本）：

```bash
cp dist/key-provider-access.so <插件目录>/linux/amd64/key-provider-access-v0.0.5.so
podman restart cli-proxy-api
```

重启后 `GET /v0/management/plugins/key-provider-access/status` 的 `version` 应为 `0.0.5-cpamp-v3`。

## 注意

- 补丁不在上游发布通道内。重装或升级官方插件会覆盖它，两个缺陷都会回来。
- 上游修掉任一缺陷后，应改为使用官方版本并重新验证第 3.6 节的测试项。
