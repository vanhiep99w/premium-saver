# Copilot Proxy

Local proxy that saves GitHub Copilot premium requests by forwarding all requests with `X-Initiator: agent` header.

Works with any OpenAI-compatible client (OpenCode, Cursor, Continue, Cline, etc.).

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/vanhiep99w/premium-saver/main/install.sh | sh
```

Or download from [GitHub Releases](https://github.com/vanhiep99w/premium-saver/releases).

## Usage

```bash
# 1. Login with GitHub Copilot
copilot-proxy login

# 2. Start the proxy
copilot-proxy serve              # port 8787 (default)
copilot-proxy serve -p 9090      # custom port
```

Then configure your AI client:

| Setting | Value |
|---------|-------|
| **Base URL** | `http://localhost:8787/v1` |
| **API Key** | `any-value` (proxy handles auth) |

## Client Examples

<details>
<summary><b>OpenCode</b></summary>

`opencode.json`:
```json
{
  "provider": {
    "copilot-proxy": {
      "id": "custom",
      "name": "Copilot Proxy",
      "api": { "url": "http://localhost:8787/v1" },
      "models": {
        "claude-sonnet-4": {
          "id": "claude-sonnet-4",
          "name": "Claude Sonnet 4",
          "api_key_env": "DUMMY_KEY"
        }
      }
    }
  }
}
```
```bash
export DUMMY_KEY=sk-dummy
```
</details>

<details>
<summary><b>Cursor</b></summary>

Settings > Models > OpenAI API Key: `any-value`
Override API Base URL: `http://localhost:8787/v1`
</details>

<details>
<summary><b>Continue (VS Code)</b></summary>

`~/.continue/config.json`:
```json
{
  "models": [{
    "title": "Copilot Claude",
    "provider": "openai",
    "model": "claude-sonnet-4",
    "apiBase": "http://localhost:8787/v1",
    "apiKey": "any-value"
  }]
}
```
</details>

<details>
<summary><b>Cline</b></summary>

Settings > API Provider: OpenAI Compatible
- Base URL: `http://localhost:8787/v1`
- API Key: `any-value`
- Model: `claude-sonnet-4`
</details>

<details>
<summary><b>Claude Code</b></summary>

Edit `~/.claude/settings.json`:
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:8787/v1",
    "ANTHROPIC_AUTH_TOKEN": "any-value"
  }
}
```

Then open a new terminal and run `claude`.
</details>

<details>
<summary><b>Python / Node.js / curl</b></summary>

```python
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8787/v1", api_key="any-value")
response = client.chat.completions.create(
    model="claude-sonnet-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

```javascript
import OpenAI from 'openai';
const client = new OpenAI({ baseURL: 'http://localhost:8787/v1', apiKey: 'any-value' });
const response = await client.chat.completions.create({
  model: 'claude-sonnet-4',
  messages: [{ role: 'user', content: 'Hello!' }],
});
```

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4","messages":[{"role":"user","content":"Hello!"}]}'
```
</details>

## Other Commands

```bash
copilot-proxy status    # Check auth status
copilot-proxy logout    # Clear tokens
copilot-proxy help      # Show help
```

## How It Works

GitHub Copilot bills `X-Initiator: user` requests as premium (limited quota), but `X-Initiator: agent` requests differently. This proxy forces all requests to use `agent`, so they don't count toward your premium quota.

Tokens are stored at `~/.config/copilot-proxy/auth.json`. You only need to login once — the proxy auto-refreshes tokens.

## License

MIT
