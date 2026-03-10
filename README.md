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

## Available Models

40+ models available via Copilot. Run `copilot-proxy serve` and query `http://localhost:8787/v1/models` for the full list.

### Anthropic

| Model ID | Name | Category |
|----------|------|----------|
| `claude-opus-4.6` | Claude Opus 4.6 | powerful |
| `claude-opus-4.6-fast` | Claude Opus 4.6 (fast mode) | powerful (preview) |
| `claude-sonnet-4.6` | Claude Sonnet 4.6 | versatile |
| `claude-sonnet-4.5` | Claude Sonnet 4.5 | versatile |
| `claude-sonnet-4` | Claude Sonnet 4 | versatile |
| `claude-opus-4.5` | Claude Opus 4.5 | powerful |
| `claude-haiku-4.5` | Claude Haiku 4.5 | versatile |

### OpenAI

| Model ID | Name | Category |
|----------|------|----------|
| `gpt-5.4` | GPT-5.4 | powerful |
| `gpt-5.3-codex` | GPT-5.3-Codex | powerful |
| `gpt-5.2` | GPT-5.2 | versatile |
| `gpt-5.2-codex` | GPT-5.2-Codex | powerful |
| `gpt-5.1` | GPT-5.1 | versatile |
| `gpt-5.1-codex` | GPT-5.1-Codex | powerful |
| `gpt-5.1-codex-max` | GPT-5.1-Codex-Max | powerful |
| `gpt-5.1-codex-mini` | GPT-5.1-Codex-Mini | powerful (preview) |
| `gpt-5-mini` | GPT-5 mini | lightweight |
| `gpt-4.1` | GPT-4.1 | versatile |
| `gpt-4o` | GPT-4o | versatile |

### Google

| Model ID | Name | Category |
|----------|------|----------|
| `gemini-3.1-pro-preview` | Gemini 3.1 Pro | powerful (preview) |
| `gemini-3-pro-preview` | Gemini 3 Pro | powerful (preview) |
| `gemini-3-flash-preview` | Gemini 3 Flash | lightweight (preview) |
| `gemini-2.5-pro` | Gemini 2.5 Pro | powerful |

### Microsoft / xAI

| Model ID | Name | Category |
|----------|------|----------|
| `oswe-vscode-prime` | Raptor mini | versatile (preview) |
| `grok-code-fast-1` | Grok Code Fast 1 | lightweight |

### Embeddings

| Model ID | Name |
|----------|------|
| `text-embedding-3-small` | Embedding V3 small |
| `text-embedding-3-small-inference` | Embedding V3 small (Inference) |

<details>
<summary><b>Legacy / internal models (14 more)</b></summary>

These are older model versions or internal aliases. They still work but aren't shown in Copilot's model picker:

`gpt-4o-2024-11-20`, `gpt-4o-2024-08-06`, `gpt-4o-2024-05-13`, `gpt-4o-mini-2024-07-18`, `gpt-4.1-2025-04-14`, `gpt-4-o-preview`, `gpt-4-0125-preview`, `gpt-4-0613`, `gpt-4`, `gpt-3.5-turbo-0613`, `gpt-3.5-turbo`, `gpt-4o-mini`, `oswe-vscode-secondary`, `text-embedding-ada-002`
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
