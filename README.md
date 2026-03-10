# Copilot Proxy

A local proxy server that saves GitHub Copilot premium requests. It works by forwarding all API requests with `X-Initiator: agent` header, which GitHub Copilot bills differently from regular user requests.

Works with **any OpenAI-compatible client** (OpenCode, Cursor, Continue, Cline, custom scripts, etc.).

## How It Works

GitHub Copilot distinguishes between two types of requests:
- `X-Initiator: user` — counts as a **premium request** (limited quota)
- `X-Initiator: agent` — billed differently (cheaper or free)

This proxy sits between your AI client and GitHub Copilot's API, forcing all requests to use `X-Initiator: agent`.

```
┌──────────────┐     ┌─────────────────────┐     ┌──────────────────────┐
│  Any Client  │────▶│   Copilot Proxy      │────▶│ api.githubcopilot.com│
│  (Cursor,    │◀────│   (localhost:8787)    │◀────│                      │
│   OpenCode,  │     │                      │     │                      │
│   Cline...) │     │  - Auth management   │     │                      │
└──────────────┘     │  - Header injection  │     └──────────────────────┘
                     │  - Stream forwarding │
                     └─────────────────────┘
```

## Requirements

- A GitHub account with Copilot subscription

## Installation

### One-line install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/vanhiep99w/premium-saver/main/install.sh | sh
```

This auto-detects your OS and architecture, downloads the latest binary, and installs it to `/usr/local/bin`.

**Custom install directory:**
```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/vanhiep99w/premium-saver/main/install.sh | sh
```

### Download from GitHub Releases

Pre-built binaries are available for Linux, macOS, and Windows at:

https://github.com/vanhiep99w/premium-saver/releases

Download the binary for your platform, make it executable, and move it to your PATH.

### Build from source

Requires Go 1.21+.

```bash
git clone https://github.com/vanhiep99w/premium-saver.git
cd premium-saver
go build -o copilot-proxy .
```

## Quick Start

### 1. Login with GitHub

```bash
copilot-proxy login
```

This starts the GitHub OAuth device flow:
1. A URL and code will be displayed
2. Open the URL in your browser
3. Enter the code to authorize
4. The proxy stores your token locally

### 2. Start the proxy

```bash
copilot-proxy serve
```

Default port is `8787`. Use `-p` to change:

```bash
copilot-proxy serve -p 9090
```

### 3. Configure your AI client

Point your client to the proxy as an OpenAI-compatible endpoint:

| Setting | Value |
|---------|-------|
| **Base URL** | `http://localhost:8787/v1` |
| **API Key** | `any-value` (the proxy handles authentication) |

## Client Configuration Examples

### OpenCode

In your `opencode.json`:

```json
{
  "provider": {
    "copilot-proxy": {
      "id": "custom",
      "name": "Copilot Proxy",
      "api": {
        "url": "http://localhost:8787/v1"
      },
      "models": {
        "claude-sonnet-4": {
          "id": "claude-sonnet-4",
          "name": "Claude Sonnet 4 (via Proxy)",
          "api_key_env": "DUMMY_KEY"
        }
      }
    }
  }
}
```

Set any dummy key:
```bash
export DUMMY_KEY=sk-dummy
```

### Cursor

Settings → Models → OpenAI API Key: `any-value`

Override API Base URL: `http://localhost:8787/v1`

### Continue (VS Code)

In `~/.continue/config.json`:

```json
{
  "models": [
    {
      "title": "Copilot Claude",
      "provider": "openai",
      "model": "claude-sonnet-4",
      "apiBase": "http://localhost:8787/v1",
      "apiKey": "any-value"
    }
  ]
}
```

### Cline

Settings → API Provider: OpenAI Compatible

- Base URL: `http://localhost:8787/v1`
- API Key: `any-value`
- Model: `claude-sonnet-4` (or any model from `/v1/models`)

### curl

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any-value" \
  -d '{
    "model": "claude-sonnet-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="any-value"
)

response = client.chat.completions.create(
    model="claude-sonnet-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### Node.js (OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8787/v1',
  apiKey: 'any-value',
});

const response = await client.chat.completions.create({
  model: 'claude-sonnet-4',
  messages: [{ role: 'user', content: 'Hello!' }],
});
console.log(response.choices[0].message.content);
```

## Available Commands

| Command | Description |
|---------|-------------|
| `copilot-proxy login` | Authenticate with GitHub Copilot via OAuth device flow |
| `copilot-proxy serve` | Start the proxy server (default port: 8787) |
| `copilot-proxy serve -p PORT` | Start on a custom port |
| `copilot-proxy status` | Show authentication status and token expiry |
| `copilot-proxy logout` | Clear all stored tokens |
| `copilot-proxy help` | Show help message |

## API Endpoints

The proxy supports these OpenAI-compatible endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (streaming & non-streaming) |
| `/v1/responses` | POST | Responses API (for GPT-5+ models) |
| `/v1/models` | GET | List available models |
| `/health` | GET | Proxy health check |

## Available Models

The proxy gives you access to all models available in your Copilot subscription. Run the proxy and check:

```bash
curl -s http://localhost:8787/v1/models | python3 -m json.tool
```

Typical models include:
- `claude-sonnet-4`, `claude-sonnet-4.6`, `claude-opus-4.6`
- `gpt-4o`, `gpt-4o-mini`, `gpt-5.1`, `gpt-5.4`
- `gemini-3.1-pro-preview`
- `grok-code-fast-1`
- And more...

## Token Management

- **OAuth token** (long-lived): Stored after login, persists across sessions
- **Copilot API token** (short-lived, ~30 min): Auto-refreshed 5 minutes before expiry
- **Storage location**: `~/.config/copilot-proxy/auth.json`

You only need to run `login` once. The proxy automatically handles token refresh.

## Troubleshooting

### "Not authenticated" error
Run `copilot-proxy login` to authenticate.

### "OAuth token expired or revoked"
Your GitHub OAuth token has expired. Run `copilot-proxy login` again.

### Connection refused
Make sure the proxy is running (`copilot-proxy serve`) and you're using the correct port.

### 404 errors from the API
Check that you're using a valid model name. List available models with `/v1/models`.

## How It Saves Premium Requests

Without the proxy, every request from your AI client to Copilot counts as a premium request against your quota. With the proxy, all requests are tagged as `X-Initiator: agent` (simulating tool/agent calls), which are billed at a lower rate or not counted toward the premium quota at all.

**Note**: This only benefits GitHub Copilot subscriptions. BYOK (Bring Your Own Key) providers like direct Anthropic or OpenAI API access are billed per token regardless.

## License

MIT
