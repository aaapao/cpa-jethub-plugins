# 部署指南：给 CPA 安装 Jet-Hub 插件

本指南覆盖从零开始的完整流程：下载插件 → 安装到 CPA → 配置模型/别名/优先级 → 验证。

---

## 1. 下载插件

从 [GitHub Release](https://github.com/collegeming/cpa-jethub-plugins/releases) 下载最新版本（当前 `v0.3.0`）：

```bash
# 下载所有插件（以 linux/amd64 为例）
BASE="https://github.com/collegeming/cpa-jethub-plugins/releases/download/v0.3.0"
for p in cline codearts codebuddy codebuddy-intl hub lobsterai loomy qoder raccoon trae workbuddy workbuddy-cn; do
  curl -LO "$BASE/${p}_0.3.0_linux_amd64.zip"
done
# 校验
curl -LO "$BASE/checksums.txt"
sha256sum -c checksums.txt 2>/dev/null | grep -v FAILED
```

每个 zip 包含一个 `.so` 文件（解压后在 zip 根目录，文件名即插件 ID）。

**需要安装的插件**（按需选择）：

| 插件 | 渠道 | 说明 |
|---|---|---|
| `cline` | Cline | WorkOS 设备码登录 |
| `codearts` | 华为云 CodeArts | 额度包计费 |
| `codebuddy` | 腾讯 CodeBuddy | 签到 + 积分 |
| `codebuddy-intl` | WorkBuddy（国际） | 无签到，按需 |
| `hub` | 渠道总览 | 管理面板入口，**建议安装** |
| `lobsterai` | LobsterAI（有道） | 签到 + 积分 |
| `loomy` | Loomy | 仅探测，无续期 |
| `qoder` | Qoder | 含内嵌 WASM 签名，免费模型可用 |
| `raccoon` | Raccoon（SenseNova） | 免费模型多 |
| `trae` | TRAE（字节） | 需配置登录回调端口 |
| `workbuddy` / `workbuddy-cn` | WorkBuddy 变体 | 按需 |

---

## 2. 安装到 CPA

### 2.1 找到插件目录

CPA 配置文件 `config.yaml` 中 `plugins.dir` 指定的路径。常见位置：

| 部署方式 | 插件目录 |
|---|---|
| 容器（Podman/Docker） | 宿主机上挂载到容器 `/CLIProxyAPI/plugins` 的目录 |
| 直接运行 | `config.yaml` 中 `plugins.dir` 的值 |

### 2.2 解压插件

```bash
# 容器部署示例（插件目录挂载在宿主机 /data/CLIProxyAPI/plugins）
PLUGIN_DIR=/data/CLIProxyAPI/plugins

for p in cline codearts codebuddy hub lobsterai loomy qoder raccoon trae; do
  unzip -o ${p}_0.3.0_linux_amd64.zip -d ${PLUGIN_DIR}/linux/amd64/
done
```

目录结构应为：

```
<插件目录>/
└── linux/
    └── amd64/
        ├── cline.so
        ├── codearts.so
        ├── codebuddy.so
        ├── hub.so
        ├── lobsterai.so
        ├── loomy.so
        ├── qoder.so
        ├── raccoon.so
        └── trae.so
```

### 2.3 重启 CPA

```bash
podman restart cli-proxy-api    # 容器部署
# 或
systemctl restart cli-proxy-api # systemd 部署
```

---

## 3. 配置 CPA（config.yaml）

### 3.1 启用插件

在 `config.yaml` 的 `plugins.configs` 下启用需要的渠道：

```yaml
plugins:
  enabled: true
  dir: "/CLIProxyAPI/plugins"
  configs:
    cline:
      enabled: true
      model_prefix: false        # 关闭账号 ID 前缀
    codearts:
      enabled: true
      model_prefix: false
    codebuddy:
      enabled: true
      model_prefix: false
    codebuddy-intl:
      enabled: false
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
      region: qoder-cn              # `qoder`=国际版；`qoder-cn`=国内版
      machine_token_path: /CLIProxyAPI/auths/machine_token.json  # 官方客户端设备身份（可选）
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

### 3.2 登录各渠道账号

每个渠道需要通过各自的登录流程获取凭据：

| 渠道 | 登录方式 | 管理页路径 |
|---|---|---|
| cline | WorkOS 设备码 | `/v0/resource/plugins/cline/login` |
| codearts | 华为云 OAuth | `/v0/resource/plugins/codearts/login` |
| codebuddy | 浏览器登录 | `/v0/resource/plugins/codebuddy/login` |
| lobsterai | 浏览器两步式 | `/v0/resource/plugins/lobsterai/login` |
| loomy | 浏览器登录 | `/v0/resource/plugins/loomy/login` |
| qoder | 设备码（PKCE） | `/v0/resource/plugins/qoder/login` |
| raccoon | 浏览器登录 | `/v0/resource/plugins/raccoon/login` |
| trae | 回调端口登录 | `/v0/resource/plugins/trae/login` |

在浏览器中打开管理面板（CPAMP），进入「插件管理」→ 对应渠道 → 「登录」，按页面提示完成授权。

#### Qoder 的两个版本（`region`）

| | 国际版 `qoder` | 国内版 `qoder-cn` |
|---|---|---|
| 认证站点 | qoder.com | qoder.cn |
| clientId | `e883ade2-…` | `732aef47-…`（不同！） |
| 推理 | 加密端点（内嵌 WASM） | 加密端点（**无公开端点**） |
| 模型目录 | 17 条 | 14 条（独有 `q37fmodel`/`gm51model`，`mmodel`=MiniMax-M2.7） |
| 免费模型 | `qfmodel`、`qmodel_38max` | 同 |

#### 设备身份（machine_token，强烈建议配置）

Qoder 服务端要求请求携带**官方客户端的设备身份**（`Cosy-MachineToken` + `Cosy-MachineType` 成对出现），否则：积分页看不到每日领取活动、推理会话更容易被风控作废。

设备身份来自本机 Qoder IDE 的 `machine_token.json`（由官方 `runtime-info` 生成，插件无法自造）：

- Windows：`%APPDATA%\Qoder\SharedClientCache\cache\machine_token.json`
- macOS：`~/Library/Application Support/Qoder/SharedClientCache/cache/machine_token.json`

把它复制到 CPA 的 `auths/` 目录（容器内路径 `/CLIProxyAPI/auths/` 或 `/root/.cli-proxy-api/`，按挂载为准）并配置 `machine_token_path`。该文件与账号无关（设备级），**可跨机器复用**，旧文件也依然有效。

### 3.3 配置模型排除（oauth-excluded-models）

用于隐藏不需要的模型（如付费模型、重复模型）。支持 `*` 通配符：

```yaml
oauth-excluded-models:
  cline:
    - "cline-pass/*"
    # ... 按需排除
  lobsterai:
    - "kimi-k3"           # x20 倍率
  loomy:
    - "qwen-3.8-max"      # x12 倍率
    - "MiniMax-M3"        # x4.0 倍率
  qoder:
    - "kmodel_latest"     # x1.4 倍率
  raccoon:
    - "sn-kimi-k3"
```

### 3.4 配置模型别名（oauth-model-alias）

将各渠道的上游模型名映射为统一的 `-Oauth` 命名。同一模型名由多个渠道提供时，各渠道的别名保持一致（CPA 自动合并为一个模型条目，按优先级选择渠道）：

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

### 3.5 设置凭据优先级

在每个凭据 JSON 文件（`auths/` 目录下）中添加 `priority` 字段。**优先级语义**：CPA 选择渠道时只取该模型可用的最高档，档内轮询；高档不可用时自动降档。

| 档位 | 渠道 | 说明 |
|---|---|---|
| **6** | cline | 最高档（免费模型多） |
| **5** | codearts, codebuddy, lobsterai | 第二档 |
| **4** | qoder, raccoon, loomy | 第三档 |

```bash
cd <CPA数据目录>/auths
python3 -c "
import json, os, stat
prio = {
    'cline-*.json': 6,
    'codearts-*.json': 5,
    'codebuddy-*.json': 5,
    'lobsterai-*.json': 5,
    'qoder-*.json': 4,
    'raccoon-*.json': 4,
    'loomy-*.json': 4,
}
import glob
for pattern, p in prio.items():
    for f in glob.glob(pattern):
        st = os.stat(f)
        d = json.load(open(f))
        d['priority'] = p
        json.dump(d, open(f, 'w'), ensure_ascii=False, indent=1)
        os.chmod(f, stat.S_IMODE(st.st_mode))
        print(f'{f} -> priority={p}')
"
```

> **注意**：插件自动续期时会重写凭据文件。v0.3.0+ 的插件已修复为保留 `priority` 等宿主托管字段，无需重复设置。

---

## 4. 验证

### 4.1 检查插件加载

```bash
curl -s -H "Authorization: Bearer <管理密钥>" \
  http://localhost:8317/v0/management/plugins | python3 -c "
import sys, json
for p in json.load(sys.stdin)['plugins']:
    if p.get('effective_enabled'):
        print(f\"  ✓ {p['id']}\")
"
```

### 4.2 检查模型列表

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

### 4.3 检查凭据优先级

```bash
curl -s -H "Authorization: Bearer <管理密钥>" \
  http://localhost:8317/v0/management/auth-files | python3 -c "
import sys, json
for a in sorted(json.load(sys.stdin).get('files') or [], key=lambda x:-(x.get('priority') or 0)):
    print(f\"  {a['name']:48s} priority={a.get('priority')}\")
"
```

### 4.4 测试推理

```bash
curl -s -X POST http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer <API密钥>" \
  -H "Content-Type: application/json" \
  -d '{"model":"DeepSeek-V4.1-Flash-Oauth","messages":[{"role":"user","content":"说三个字"}],"max_tokens":16}'
```

### 4.5 测试流式

```bash
curl -s -N -X POST http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer <API密钥>" \
  -H "Content-Type: application/json" \
  -d '{"model":"Kimi-K3-Oauth","messages":[{"role":"user","content":"hi"}],"max_tokens":8,"stream":true}'
```

确认响应是标准 SSE 格式（`data: {...}` 帧 + `data: [DONE]`），无双重 `data: data:` 前缀。

---

## 5. 常见问题

| 问题 | 原因 | 解决 |
|---|---|---|
| 模型列表出现 `<账号ID>/<模型名>` | `model_prefix` 未设为 `false` | 在 `plugins.configs.<渠道>.model_prefix: false` |
| 凭据优先级续期后丢失 | 插件版本过旧 | 使用 v0.3.0+ |
| 流式响应 `data: data:` 双重前缀 | 插件版本过旧 | 使用 v0.3.0+ |
| qoder 推理返回 `Unsupported model` | 走了公开端点 | 使用 v0.3.0+（内嵌 WASM，自动走加密端点） |
| qoder 推理返回 `quota exceeded` | 账号 0 额度 | 免费模型（qfmodel/qmodel_38max）可用；或充值 |
| cline 续期后 token 过期 | 有效期未随新 token 更新 | 使用 v0.3.0+ |
| 插件登录回调不通 | 容器端口未映射 | 确保 `callback_port` 映射到宿主机 `0.0.0.0` |
| 别名模型调用返回 `model not found` | 上游模型名不匹配 | 检查 `oauth-model-alias` 中的 `name` 是否与上游一致 |
| qoder 每日领取显示「无可领取活动」 | 未配置设备身份 | 配置 `machine_token_path` 指向官方客户端的 machine_token.json |
| qoder 凭证反复失效（重登录后几小时又 401） | 会话被风控作废（自造设备身份易触发） | 配置 `machine_token_path` 复用官方客户端设备身份；避免与 IDE 频繁交替登录 |
| codearts 报 `Message role cannot empty`（HTTP 500） | 上游不认 OpenAI 的 `developer` 角色 | 使用 v0.3.1+（插件把 developer 归一为语义等价的 system） |
| lobsterai 报 `角色信息不正确`（HTTP 502） | 同上 | 使用 v0.3.1+ |
| 客户端（如 DSH）把系统提示词发成 `developer` 角色时的通用说明 | 部分上游只认 system | 使用 v0.3.1+；全部插件已在请求侧把 developer 归一为 system |

---

## 6. 更新插件

```bash
# 下载新版本 zip，解压覆盖，重启
unzip -o <插件>_0.3.0_linux_amd64.zip -d <插件目录>/linux/amd64/
podman restart cli-proxy-api
```

**注意**：替换 `.so` 文件必须重启 CPA；改 `config.yaml` 无需重启（热加载）。

---

## 7. 从源码构建（可选）

仅在需要修改插件代码或目标平台无预编译产物时使用。

**前置条件**：Go ≥ 1.23（需 CGO）、GCC。

```bash
git clone git@github.com:collegeming/cpa-jethub-plugins.git
cd cpa-jethub-plugins

export CGO_ENABLED=1
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off

# 构建
bash scripts/build.sh        # 产物在 dist/linux/amd64/

# 打包发布
VERSION=0.3.0 bash scripts/release.sh --skip-build
```

交叉编译：`GOOS=darwin GOARCH=arm64 bash scripts/build.sh`（Apple Silicon）。
