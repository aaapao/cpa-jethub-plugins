# 部署指南：在新服务器上复刻 CPA + Jet-Hub 插件

本指南覆盖从零开始的完整流程：拉取代码 → 构建插件 → 安装到 CPA → 配置模型/别名/优先级 → 配置 DSH → 验证。

目标读者：需要在另一台服务器上复刻当前 OAuth 渠道接入效果的运维/开发人员。

---

## 1. 前置条件

| 依赖 | 版本要求 | 检查命令 |
|---|---|---|
| Go | ≥ 1.23（需 CGO） | `go version` |
| GCC | 任意近期版本（CGO 交叉编译用） | `gcc --version` |
| CPA (CLIProxyAPI) | ≥ v7.3.x（需支持 native plugin） | `cli-proxy-api --version` 或看容器日志 |
| Podman / Docker | 可选（容器部署时） | `podman --version` |
| Git | 任意近期版本 | `git --version` |

Go 工具链环境变量（构建时需要）：

```bash
export PATH=$HOME/.local/go/bin:$PATH   # 若 Go 装在非标准路径
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off
export GOFLAGS=-mod=mod
export GOTOOLCHAIN=local
export CGO_ENABLED=1
```

---

## 2. 拉取仓库

```bash
git clone git@github.com:collegeming/cpa-jethub-plugins.git
cd cpa-jethub-plugins
```

如果使用 HTTPS：

```bash
git clone https://github.com/collegeming/cpa-jethub-plugins.git
```

仓库结构：

```
cpa-jethub-plugins/
├── plugins/            # 8 个 OAuth 渠道插件 + hub（渠道总览）
│   ├── cline/          # Cline（WorkOS 设备码登录）
│   ├── codearts/       # 华为云 CodeArts
│   ├── codebuddy/      # 腾讯 CodeBuddy / WorkBuddy
│   ├── lobsterai/      # LobsterAI（有道）
│   ├── loomy/          # Loomy
│   ├── qoder/          # Qoder（国际/国内版，含内嵌 WASM 签名）
│   ├── raccoon/        # Raccoon（SenseNova）
│   ├── trae/           # TRAE（字节）
│   └── hub/            # 渠道总览管理页
├── internal/           # 共享库（认证、流式、SSE、UI 组件等）
├── scripts/build.sh    # 一键构建脚本
├── docs/               # 文档
└── registry.json       # 插件商店元数据
```

---

## 3. 构建插件

```bash
bash scripts/build.sh
```

构建产物在 `dist/linux/amd64/`，共 12 个 `.so` 文件（8 个渠道插件 + hub + 3 个变体）。

**验证构建成功**：

```bash
ls dist/linux/amd64/*.so | wc -l   # 应输出 12
```

**交叉编译到其他平台**（如 macOS / Windows）：

```bash
GOOS=darwin GOARCH=arm64 bash scripts/build.sh   # Apple Silicon
GOOS=windows GOARCH=amd64 bash scripts/build.sh   # Windows
```

注意：Go 的 `-buildmode=c-shared` 产物**不能跨平台使用**，必须在目标平台构建或交叉编译。

---

## 4. 安装到 CPA

### 4.1 找到 CPA 的插件目录

CPA 配置文件 `config.yaml` 中 `plugins.dir` 指定的路径。常见位置：

| 部署方式 | 插件目录 |
|---|---|
| 容器（Podman/Docker） | 宿主机上挂载到容器 `/CLIProxyAPI/plugins` 的目录 |
| 直接运行 | `config.yaml` 中 `plugins.dir` 的值 |

### 4.2 复制插件

```bash
# 容器部署示例（插件目录挂载在宿主机 /data/CLIProxyAPI/plugins）
cp dist/linux/amd64/*.so /data/CLIProxyAPI/plugins/linux/amd64/
```

目录结构应为：

```
<插件目录>/
└── linux/
    └── amd64/
        ├── cline-v0.1.0.so
        ├── codearts-v0.1.0.so
        ├── codebuddy-v0.1.0.so
        ├── hub-v0.1.0.so
        ├── lobsterai-v0.1.0.so
        ├── loomy-v0.1.0.so
        ├── qoder-v0.1.0.so
        ├── raccoon-v0.1.0.so
        └── trae-v0.1.0.so
```

### 4.3 重启 CPA

```bash
podman restart cli-proxy-api   # 容器部署
# 或
systemctl restart cli-proxy-api   # systemd 部署
```

---

## 5. 配置 CPA（config.yaml）

### 5.1 启用插件

在 `config.yaml` 的 `plugins.configs` 下启用需要的渠道：

```yaml
plugins:
  enabled: true
  dir: "/CLIProxyAPI/plugins"
  configs:
    cline:
      enabled: true
      model_prefix: false        # 关闭账号 ID 前缀（<账号>/<模型>）
    codearts:
      enabled: true
      model_prefix: false
    codebuddy:
      enabled: true
      model_prefix: false
    codebuddy-intl:
      enabled: false             # 国际版 WorkBuddy（无签到，按需启用）
      model_prefix: false
    lobsterai:
      enabled: true
      model_prefix: false
    loomy:
      enabled: true
      model_prefix: false
    qoder:
      enabled: true
      model_prefix: false
    raccoon:
      enabled: true
      model_prefix: false
    trae:
      enabled: true
      model_prefix: false
      callback_port: 18092       # TRAE 登录回调端口
      callback_bind_host: "0.0.0.0"
```

**`model_prefix: false`** 是关键：关闭后模型列表只显示模型名，不会出现 `<账号ID>/<模型名>` 的冗余条目。

### 5.2 配置模型排除（oauth-excluded-models）

用于隐藏不需要的模型（如付费模型、重复模型、废弃模型）。支持 `*` 通配符：

```yaml
oauth-excluded-models:
  cline:
    - "cline-pass/*"
    - "usr-*/glm-4.7"
    - "usr-*/gemini-2.5-flash"
    # ... 按需排除
  lobsterai:
    - "kimi-k3"           # x20 倍率，太贵
  loomy:
    - "qwen-3.8-max"      # x12 倍率
    - "MiniMax-M3"        # x4.0 倍率
  qoder:
    - "kmodel_latest"     # x1.4 倍率
  raccoon:
    - "sn-kimi-k3"        # x1 倍率
  # ... 其他渠道
```

### 5.3 配置模型别名（oauth-model-alias）

将上游模型名映射为统一的 `-Oauth` 命名：

```yaml
oauth-model-alias:
  cline:
    - name: "usr-*/deepseek-v4.1-flash"
      alias: "DeepSeek-V4.1-Flash-Oauth"
    - name: "usr-*/glm-5.2"
      alias: "GLM-5.2-Oauth"
    - name: "usr-*/glm-5.3"
      alias: "GLM-5.3-Oauth"
    - name: "usr-*/glm-5.3-flash"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "usr-*/kimi-k3"
      alias: "Kimi-K3-Oauth"
    - name: "usr-*/minimax-m3"
      alias: "MiniMax-M3-Oauth"
    - name: "usr-*/qwen3.8-max"
      alias: "Qwen3.8-Max-Oauth"
    - name: "usr-*/qwen3.8-flash"
      alias: "Qwen3.8-Flash-Next-Oauth"
    - name: "usr-*/mimo-v2.6-pro"
      alias: "MiMo-V2.6-Pro-Oauth"
    - name: "usr-*/mimo-v2.6-flash"
      alias: "MiMo-V2.6-Flash-Oauth"
  codebuddy:
    - name: "kimi-k3-1"
      alias: "Kimi-K3-Oauth"
    - name: "deepseek-v4.1-flash"
      alias: "DeepSeek-V4.1-Flash-Oauth"
    - name: "glm-5.2"
      alias: "GLM-5.2-Oauth"
    - name: "glm-5.3-flash"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "minimax-m3"
      alias: "MiniMax-M3-Oauth"
  lobsterai:
    - name: "deepseek-flash"
      alias: "DeepSeek-V4.1-Flash-Oauth"
    - name: "glm-5.3-flash"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "glm-5.3"
      alias: "GLM-5.3-Oauth"
    - name: "qwen3.8-max"
      alias: "Qwen3.8-Max-Oauth"
    - name: "qwen3.8-flash"
      alias: "Qwen3.8-Flash-Next-Oauth"
    - name: "MiniMax-M3"
      alias: "MiniMax-M3-Oauth"
  qoder:
    - name: "qmodel_38max"
      alias: "Qwen3.8-Max-Oauth"
    - name: "qfmodel"
      alias: "Qwen3.8-Flash-Next-Oauth"
    - name: "gmodel"
      alias: "GLM-5.3-Oauth"
    - name: "gfmodel"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "mmodel"
      alias: "MiniMax-M3-Oauth"
  raccoon:
    - name: "sn-deepseek-v4.1-flash"
      alias: "DeepSeek-V4.1-Flash-Oauth"
    - name: "sn-glm-5.2"
      alias: "GLM-5.2-Oauth"
    - name: "sn-glm-5.3-flash"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "sn-kimi-k3"
      alias: "Kimi-K3-Oauth"
    - name: "sn-minimax-m3"
      alias: "MiniMax-M3-Oauth"
    - name: "sn-sensenova-6-8-flash"
      alias: "SenseNova-6.8-Flash-Oauth"
    - name: "sn-sensenova-6-8-flash-lite"
      alias: "SenseNova-6.8-Flash-Lite-Oauth"
  loomy:
    - name: "glm-5.3-flash"
      alias: "GLM-5.3-Flash-Oauth"
    - name: "qwen3.8-flash"
      alias: "Qwen3.8-Flash-Next-Oauth"
```

**别名命名规则**：统一用 `<模型名>-Oauth` 后缀，与 DSH 里的模型名对应。同一模型名由多个渠道提供时，各渠道的别名保持一致（CPA 会自动合并为一个模型条目，按优先级选择渠道）。

### 5.4 设置凭据优先级

在每个凭据 JSON 文件（`<插件目录>/../auths/` 下）中添加 `priority` 字段：

| 档位 | 渠道 | 说明 |
|---|---|---|
| **6** | cline | 最高档（免费模型多） |
| **5** | codearts ×2, codebuddy ×2, lobsterai | 第二档 |
| **4** | qoder, raccoon, loomy, codebuddy-intl | 第三档 |

```bash
# 在 auths 目录下
python3 -c "
import json, os, stat
prio = {
    'cline-collegeming@outlook.com.json': 6,
    'codearts-HSTAANV6KVKBXL9FKENJ.json': 5,
    'codearts-HSTAVFVPPZRCVTMS4TVV.json': 5,
    'codebuddy-18695721767.json': 5,
    'codebuddy-account-39cf135e.json': 5,
    'lobsterai-101989.json': 5,
    'qoder-c51b62a6-95aa-427e-8ffa-de4ff89ee260.json': 4,
    'raccoon-7450998.json': 4,
    'loomy-13275631767.json': 4,
    'codebuddy-intl-collegeming@outlook.com.json': 4,
}
for f, p in prio.items():
    if not os.path.exists(f): continue
    st = os.stat(f)
    d = json.load(open(f))
    d['priority'] = p
    json.dump(d, open(f, 'w'), ensure_ascii=False, indent=1)
    os.chmod(f, stat.S_IMODE(st.st_mode))
    print(f'{f} -> priority={p}')
"
```

**优先级语义**：CPA 选择渠道时**只取该模型可用的最高档**，档内轮询；高档不可用时自动降档。同一档内多个渠道会平分请求。

**注意**：插件自动续期时会重写凭据文件。我们的插件已修复为保留 `priority` 等宿主托管字段（提交 `b5ca9bd`–`0c9a577`），但如果你使用旧版插件，续期后可能丢失优先级，需要重新设置。

---

## 6. 配置 DSH（可选）

如果用 DSH 作为客户端，需要在 `cordis.patch.yml` 中添加 `-Oauth` 模型条目：

```yaml
providers:
  cpa:
    baseURL: "http://localhost:8317/v1"
    models:
      - id: DeepSeek-V4.1-Flash-Oauth
        name: DeepSeek-V4.1-Flash-Oauth
        contextWindow: 1000000
        maxTokens: 8192
        reasoningEfforts:
          low: low
          high: high
          max: max
        input:
          - text
          - image
      - id: Kimi-K3-Oauth
        name: Kimi-K3-Oauth
        contextWindow: 1048576
        maxTokens: 8192
        reasoningEfforts:
          low: low
          high: high
          max: max
        input:
          - text
          - image
      # ... 其他 -Oauth 模型
```

DSH 发送 `reasoning_effort` 字段控制思考强度（`low`/`medium`/`high`/`xhigh`/`max`）。各渠道支持的档位不同，详见 `docs/THINKING.md`（如已创建）。

---

## 7. 验证

### 7.1 检查插件加载

```bash
curl -s -H "Authorization: Bearer <管理密钥>" \
  http://localhost:8317/v0/management/plugins | python3 -m json.tool
```

确认所有插件的 `effective_enabled` 为 `true`。

### 7.2 检查模型列表

```bash
curl -s -H "Authorization: Bearer <API密钥>" \
  http://localhost:8317/v1/models | python3 -c "
import sys, json
d = json.load(sys.stdin)
oauth = [m['id'] for m in d['data'] if m['id'].endswith('-Oauth')]
print(f'总模型数: {len(d[\"data\"])}  |  -Oauth: {len(oauth)}')
for m in sorted(oauth): print(f'  {m}')
"
```

### 7.3 检查凭据优先级

```bash
curl -s -H "Authorization: Bearer <管理密钥>" \
  http://localhost:8317/v0/management/auth-files | python3 -c "
import sys, json
for a in sorted(json.load(sys.stdin).get('files') or [], key=lambda x:-(x.get('priority') or 0)):
    print(f\"  {a['name']:48s} priority={a.get('priority')}\")
"
```

### 7.4 测试推理

```bash
curl -s -X POST http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer <API密钥>" \
  -H "Content-Type: application/json" \
  -d '{"model":"DeepSeek-V4.1-Flash-Oauth","messages":[{"role":"user","content":"说三个字"}],"max_tokens":16}'
```

### 7.5 测试流式

```bash
curl -s -N -X POST http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer <API密钥>" \
  -H "Content-Type: application/json" \
  -d '{"model":"Kimi-K3-Oauth","messages":[{"role":"user","content":"hi"}],"max_tokens":8,"stream":true}'
```

确认响应是标准 SSE 格式（`data: {...}` 帧 + `data: [DONE]`），无双重 `data: data:` 前缀。

---

## 8. 常见问题

| 问题 | 原因 | 解决 |
|---|---|---|
| 模型列表出现 `<账号ID>/<模型名>` | `model_prefix` 未设为 `false` | 在 `plugins.configs.<渠道>.model_prefix: false` |
| 凭据优先级续期后丢失 | 插件版本过旧 | 使用本仓库最新构建（含 `b5ca9bd`+ 修复） |
| 流式响应 `data: data:` 双重前缀 | 插件版本过旧 | 使用本仓库最新构建（含 `dd745bb` 修复） |
| qoder 推理返回 `Unsupported model` | 走了公开端点 | 使用本仓库最新构建（含 `47be25b` 内嵌 WASM） |
| qoder 推理返回 `quota exceeded` | 账号 0 额度 | 免费模型 `qfmodel`/`qmodel_38max` 可用（需加密路径）；或充值 |
| cline 续期后 token 过期 | 有效期未随新 token 更新 | 使用本仓库最新构建（含 `aea1f48` 修复） |
| 插件登录回调不通 | 容器端口未映射 | 确保 `callback_port` 映射到宿主机 `0.0.0.0` |
| 别名模型调用返回 `model not found` | 上游模型名不匹配 | 检查 `oauth-model-alias` 中的 `name` 是否与上游一致 |

---

## 9. 更新插件

```bash
cd cpa-jethub-plugins
git pull origin main
bash scripts/build.sh
cp dist/linux/amd64/*.so <插件目录>/linux/amd64/
podman restart cli-proxy-api
```

**注意**：插件热加载（改配置无需重启），但**替换 `.so` 文件必须重启 CPA**。

---

## 10. 当前环境参考值

以下为当前生产环境的实际配置，供参考：

| 项 | 值 |
|---|---|
| CPA 版本 | v7.3.18 |
| 插件数 | 9 个启用（cline, codearts, codebuddy, hub, lobsterai, loomy, qoder, raccoon, trae） |
| 凭据数 | 10 个（codebuddy-intl 已禁用） |
| 模型总数 | 73 个（36 个 `-Oauth`） |
| 凭据优先级 | cline 6 / codearts·codebuddy·lobsterai 5 / qoder·raccoon·loomy 4 |
| qoder WASM | 内嵌（无需外部文件） |
| 流式格式 | 裸 JSON payload（宿主负责 SSE 帧） |
