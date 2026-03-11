# Initiator Policy Settings Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a runtime-configurable `x-initiator` policy with env default and a minimal admin settings page, while matching OpenCode's `User-Agent` and `Openai-Intent` headers.

**Architecture:** Introduce a small proxy policy object that owns the initiator cadence and request counter. Wire it into header injection, expose read/update methods to admin handlers, and render a lightweight settings form that updates only in-memory state.

**Tech Stack:** Go, `net/http`, Go templates, embedded static assets, existing admin session/CSRF flow.

---

### Task 1: Add failing tests for initiator cadence policy

**Files:**
- Create: `proxy/initiator_policy_test.go`
- Modify: `proxy/`

**Step 1: Write the failing test**

Add tests that define:
- default behavior emits `agent`
- every Nth request emits `user`
- updating cadence only affects future calls

**Step 2: Run test to verify it fails**

Run: `go test ./proxy -run TestInitiatorPolicy`
Expected: FAIL because policy type does not exist yet

**Step 3: Write minimal implementation**

Create a small policy type in `proxy` with:
- current `userEvery`
- request counter
- `NextInitiator() string`
- `GetUserEvery() int`
- `SetUserEvery(int)`

**Step 4: Run test to verify it passes**

Run: `go test ./proxy -run TestInitiatorPolicy`
Expected: PASS

### Task 2: Add failing tests for config parsing

**Files:**
- Create: `config/config_test.go`
- Modify: `config/config.go`

**Step 1: Write the failing test**

Add tests for env parsing:
- unset -> default `20`
- valid positive integer -> parsed value
- invalid or non-positive -> fallback `20`

**Step 2: Run test to verify it fails**

Run: `go test ./config -run TestInitiatorUserEvery`
Expected: FAIL because helper does not exist yet

**Step 3: Write minimal implementation**

Add config helper returning initiator cadence default.

**Step 4: Run test to verify it passes**

Run: `go test ./config -run TestInitiatorUserEvery`
Expected: PASS

### Task 3: Add failing tests for header injection

**Files:**
- Create: `proxy/headers_test.go`
- Modify: `proxy/headers.go`

**Step 1: Write the failing test**

Add tests that assert:
- `User-Agent` becomes `opencode/<version>`
- `Openai-Intent` remains `conversation-edits`
- `Authorization` is replaced
- `x-api-key` is removed
- `X-Initiator` uses the supplied initiator value

**Step 2: Run test to verify it fails**

Run: `go test ./proxy -run TestInjectHeaders`
Expected: FAIL because current function has fixed initiator/user-agent behavior

**Step 3: Write minimal implementation**

Update header injection to accept resolved initiator and OpenCode-like constants.

**Step 4: Run test to verify it passes**

Run: `go test ./proxy -run TestInjectHeaders`
Expected: PASS

### Task 4: Add failing tests for admin runtime settings update

**Files:**
- Create: `admin/settings_test.go`
- Modify: `admin/handlers.go`

**Step 1: Write the failing test**

Add handler tests that assert:
- authenticated settings page renders current cadence
- POST/PATCH settings with valid integer updates runtime policy
- invalid values return `400`

**Step 2: Run test to verify it fails**

Run: `go test ./admin -run TestSettings`
Expected: FAIL because settings routes and injected policy do not exist yet

**Step 3: Write minimal implementation**

Inject policy into admin package, add settings handlers/routes, and render a simple template.

**Step 4: Run test to verify it passes**

Run: `go test ./admin -run TestSettings`
Expected: PASS

### Task 5: Wire runtime policy into server startup and templates

**Files:**
- Modify: `main.go`
- Modify: `proxy/server.go`
- Modify: `admin/handlers.go`
- Modify: `web/templates/layout.html`
- Create: `web/templates/settings.html`

**Step 1: Write minimal integration test if needed**

If handler tests are insufficient, add one focused route wiring test.

**Step 2: Implement wiring**

- Build policy from config env in startup
- Pass policy into proxy server and admin setup
- Add nav link to settings page
- Add form with CSRF token and current value

**Step 3: Run targeted tests**

Run: `go test ./proxy ./admin ./config`
Expected: PASS

### Task 6: Verify whole app build

**Files:**
- Modify only if needed from previous tasks

**Step 1: Run broader verification**

Run: `go test ./...`
Expected: PASS

**Step 2: Run build**

Run: `go build ./...`
Expected: PASS
