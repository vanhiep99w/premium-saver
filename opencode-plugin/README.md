# Premium Request Saver — OpenCode Plugin

Plugin tiết kiệm premium requests khi dùng **GitHub Copilot** qua OpenCode.

## Cách hoạt động

Khi dùng GitHub Copilot, mỗi **user message** có thể tốn 1 premium request, nhưng **tool results** được gửi với `x-initiator: agent` — Copilot tính rẻ hơn hoặc miễn phí.

Plugin này inject system prompt bắt agent luôn gọi **tool `question` có sẵn** của OpenCode cuối mỗi response. Khi bạn trả lời qua `question` tool, response trả về dưới dạng tool result (miễn phí) thay vì user message (tốn premium request).

→ **Cả session chỉ tốn 1 premium request** (message đầu tiên).

## Cài đặt

### Cách 1: Global (áp dụng mọi project) — Khuyến nghị

```bash
mkdir -p ~/.config/opencode/plugins
cp premium-saver.ts ~/.config/opencode/plugins/

# Cần có dependency (nếu chưa có)
cat > ~/.config/opencode/package.json << 'EOF'
{
  "dependencies": {
    "@opencode-ai/plugin": "latest"
  }
}
EOF
```

### Cách 2: Theo project

```bash
# Trong thư mục project
mkdir -p .opencode/plugins
cp premium-saver.ts .opencode/plugins/

# Cần có dependency (nếu chưa có)
cat > .opencode/package.json << 'EOF'
{
  "dependencies": {
    "@opencode-ai/plugin": "latest"
  }
}
EOF
```

## Sử dụng

1. **Khởi động OpenCode** (restart nếu đang chạy):
   ```bash
   opencode
   ```

2. **Gửi message đầu tiên** — đây là lần duy nhất tốn premium request.

3. **Sau đó**, agent sẽ tự động gọi tool `question` cuối mỗi response.
   OpenCode sẽ hiện prompt native với ô nhập text cho bạn gõ.

4. **Gõ response** vào ô "Type your own answer" → nhấn Enter để gửi.
   Input của bạn được gửi dưới dạng tool result (miễn phí).

5. **Lặp lại** — mỗi lần trả lời qua `question`, bạn tiết kiệm 1 premium request.

## Lưu ý

| Provider | Có tiết kiệm? | Lý do |
|----------|---------------|-------|
| **GitHub Copilot** | ✅ Có | Copilot phân biệt `x-initiator: user` vs `agent` |
| **BYOK (Anthropic, OpenAI)** | ❌ Không | Billing theo token, không có premium requests |
| **Zen Balance** | ❌ Không | Billing theo token |

## Gỡ cài đặt

```bash
# Global
rm ~/.config/opencode/plugins/premium-saver.ts

# Hoặc theo project
rm .opencode/plugins/premium-saver.ts
```
