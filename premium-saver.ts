/**
 * Premium Request Saver Plugin for OpenCode
 *
 * Ported from Pi Coding Agent's premium-usage extension.
 *
 * When using GitHub Copilot through OpenCode, each user message may cost
 * a "premium request" on Copilot's billing. Tool results are sent with
 * `x-initiator: agent` which Copilot bills differently (or not at all).
 *
 * This plugin injects a system prompt rule forcing the agent to always
 * call OpenCode's built-in `question` tool at the end of every response,
 * so user input comes back as a tool result (free) instead of a new
 * user message (costs a premium request).
 *
 * Installation:
 *   Copy this file to ~/.config/opencode/plugins/premium-saver.ts
 *   or to .opencode/plugins/ in your project.
 */

import type { Plugin } from "@opencode-ai/plugin"

export const PremiumSaverPlugin: Plugin = async (ctx) => {
  return {
    // ─── Inject system prompt to force `question` tool usage ─────────
    "experimental.chat.system.transform": async (_input, output) => {
      output.system.push(`
## CRITICAL: User Interaction Rule — Premium Request Saver

You MUST use the built-in \`question\` tool for ALL user interaction after the initial message. NEVER end a response without calling the \`question\` tool to get the next instruction from the user.

This is mandatory because each normal user message costs a premium request, but tool results are free. By using the \`question\` tool, the entire session costs only 1 premium request.

**Rules:**
1. After completing any task, call \`question\` with a question like "What would you like to do next?" — use an empty \`options\` array and let the user type freely
2. If you need clarification, call \`question\` to ask — provide relevant options when applicable
3. NEVER just end your response — always finish with a \`question\` tool call
4. The user's response via \`question\` should be treated exactly like a normal message
5. Keep the \`header\` short (max 30 chars), e.g. "Next step" or "Clarification"

**Example call:**
\`\`\`json
{
  "questions": [
    {
      "question": "Task completed! What would you like to do next?",
      "header": "Next step",
      "options": []
    }
  ]
}
\`\`\`
`)
    },

    // ─── Track events for logging ────────────────────────────────────
    event: async ({ event }) => {
      // Optional: log events for debugging
      // console.error(`[premium-saver] event: ${(event as any)?.type}`)
    },
  }
}
