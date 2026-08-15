# groq-proxy

Groq API 中转代理（纯 Go 版），支持 Render 免费部署。

基于 [GA-Proxy](https://github.com/GrandpaEJ/GA-Proxy) 精简重写 —— **去掉了 Plan 9 汇编依赖**（原版 `obfs_amd64.s` 在部分 Go 版本下会编译失败，报错 `missing function body`）。

## 改动
- ✅ `Obfuscate`（amd64 汇编）→ `HashKey`（纯 Go 位运算），构建一次通过
- ✅ 删除 `obfs_amd64.s`
- ✅ 反向代理改用 `net/http`，SSE 流式响应正确透传
- ✅ 支持 Groq / OpenRouter / Cerebras 多提供商（按需填 Key）

## 文件
```
groq-proxy/
├── main.go        # Fiber 路由 + 鉴权 + 限流
├── proxy.go       # 手写反向代理（支持 SSE）
├── go.mod
├── render.yaml    # Render Blueprint 一键部署
├── .env.example
└── .gitignore
```

## Render 部署（自动）

### 方式 1：Blueprint（推荐）
1. 把本仓库 push 到 GitHub
2. Render Dashboard → New → Blueprint
3. 选择你的仓库 → 自动识别 `render.yaml`
4. 填环境变量 `GROQ_API_KEY` → Apply
5. 自动构建部署

### 方式 2：手动 Web Service
| 项 | 值 |
|---|---|
| Runtime | Go |
| Build Command | `go build -o proxy -mod=mod ./...` |
| Start Command | `./proxy` |

## 环境变量
| Key | 必填 | 说明 |
|-----|------|------|
| `GROQ_API_KEY` | ✅ | https://console.groq.com/keys |
| `PROXY_SECRET` | ❌ | 可选，启用后客户端需带 `X-GA-Secret` 头 |
| `OPENROUTER_API_KEY` | ❌ | 可选 |
| `CEREBRAS_API_KEY` | ❌ | 可选 |
| `RATE_LIMIT_MAX` | ❌ | 默认 20 req/min/IP |

## 使用
```bash
# 健康检查
curl https://your-app.onrender.com/

# Groq 聊天（OpenAI 兼容）
curl -X POST https://your-app.onrender.com/groq/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-3.1-8b-instant","messages":[{"role":"user","content":"hi"}]}'
```

## 本地运行
```bash
cp .env.example .env   # 填 GROQ_API_KEY
go mod tidy
go run .
```