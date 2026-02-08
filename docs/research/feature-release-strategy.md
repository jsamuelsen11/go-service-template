# Research: Feature Release Strategy (Front-book / Back-book)

> **Issue**: go-service-template-tl0
> **Date**: 2026-02-08
> **Status**: Complete
> **Next Step**: ADR-0002 (go-service-template-5dk)

---

## 1. Problem Statement

The service template needs a feature flag solution that supports three targeting dimensions:

| Dimension                     | Example                                                                                         | Who Decides         |
| ----------------------------- | ----------------------------------------------------------------------------------------------- | ------------------- |
| **User-level**                | Enable beta for user `user-123`                                                                 | Product/Engineering |
| **Team/LOB-level**            | Enable for `team=underwriting` AND `lob=commercial`                                             | Product/Business    |
| **Entity-level (front-book)** | New entities created after `2026-04-01` get new pricing rules; existing entities keep old rules | Business/Compliance |

These dimensions must be **composable** (e.g., "underwriting team + new entities only") and **shared across components**
(Go backend services, Web UI, potentially other services).

### Why Centralized

A local config-based solution is insufficient because:

- The **Web UI** needs the same flag state as the Go backend
- Multiple **Go services** may need consistent flag evaluation
- Flags need **real-time updates** without redeploying services
- Non-developers (product, business) may need to manage some flags

---

## 2. Library Evaluation

### 2.1 GO Feature Flag (Relay Proxy)

**How it works**: YAML flag definitions polled from file/S3/GitHub/HTTP. Relay proxy evaluates in-memory, exposes
REST/OFREP API. SDKs connect via OpenFeature.

**Architecture**:

```text
Go Service ──(OpenFeature Go SDK)──┐
                                   ├── GO Feature Flag Relay Proxy ── YAML flags (file/S3/Git)
Web UI ──(OpenFeature JS SDK)──────┘         (Docker, port 1031)
```

**User-level targeting**:

```yaml
premium-checkout:
  variations:
    enabled: true
    disabled: false
  targeting:
    - query: key eq "user-12345"
      variation: enabled
  defaultRule:
    variation: disabled
```

**Team/LOB targeting (AND logic)**:

```yaml
underwriting-automation:
  variations:
    enabled: true
    disabled: false
  targeting:
    - query: (team eq "underwriting") and (lob eq "commercial")
      variation: enabled
  defaultRule:
    variation: disabled
```

**Entity front-book targeting (date comparison)**:

```yaml
new-pricing-model:
  variations:
    new-rules: true
    old-rules: false
  targeting:
    - query: entity_created_at gt "2026-04-01T00:00:00Z"
      variation: new-rules
  defaultRule:
    variation: old-rules
```

**Combined: team + LOB + date**:

```yaml
ai-risk-scoring:
  variations:
    ai-score: true
    manual-score: false
  targeting:
    - query: (team eq "underwriting") and (lob eq "commercial") and (entity_created_at gt "2026-04-01T00:00:00Z")
      variation: ai-score
  defaultRule:
    variation: manual-score
```

**Query operators**: `eq`, `ne`, `gt`, `ge`, `lt`, `le`, `co` (contains), `sw` (starts with), `ew` (ends with), `in`,
`pr` (present). Logical: `and`, `or`.

**Multi-component access**: OpenFeature Go SDK provider
(`github.com/open-feature/go-sdk-contrib/providers/go-feature-flag`), OpenFeature Web SDK provider
(`@openfeature/go-feature-flag-web-provider`), React SDK (`@openfeature/react-sdk`).

**Infrastructure**: Single Docker container (`gofeatureflag/go-feature-flag`), distroless image. No database. Stateless.
Flags from file, S3, GitHub, HTTP, or PostgreSQL.

**Management UI**: No built-in UI. Uses [editor.gofeatureflag.org](https://editor.gofeatureflag.org) for visual editing.
Config-as-code workflow (Git PR to change flags).

**Dependency footprint**: Heavy. The core module (`github.com/thomaspoignant/go-feature-flag`) pulls AWS SDK, GCP SDK,
Azure SDK, Kafka, etc. However, using only the OpenFeature provider (`go-sdk-contrib/providers/go-feature-flag`) is much
lighter since it just makes HTTP calls to the relay proxy.

**Additional features**: Scheduled rollouts (time-based activation dates), percentage rollouts, experimentation (A/B
testing with time bounds), custom bucketing (group by team ID), data export (S3, CloudWatch), notifications (Slack,
Discord, Teams, webhook).

---

### 2.2 flagd (OpenFeature Reference Implementation)

**How it works**: JSON flag definitions loaded from file/ConfigMap/HTTP. flagd evaluates using JSON Logic rules. SDKs
connect via gRPC or HTTP/OFREP.

**Architecture**:

```text
Go Service ──(OpenFeature Go SDK + flagd provider)──┐
                                                     ├── flagd ── JSON flags (file/ConfigMap/HTTP)
Web UI ──(OpenFeature Web SDK + flagd provider)──────┘    (Docker, ports 8013/8016)
```

**User-level targeting**:

```json
{
  "flags": {
    "premium-feature": {
      "state": "ENABLED",
      "variants": { "on": true, "off": false },
      "defaultVariant": "off",
      "targeting": {
        "if": [{ "==": [{ "var": "targetingKey" }, "user-123"] }, "on", "off"]
      }
    }
  }
}
```

**Team/LOB targeting (AND)**:

```json
{
  "flags": {
    "underwriting-feature": {
      "state": "ENABLED",
      "variants": { "enabled": true, "disabled": false },
      "defaultVariant": "disabled",
      "targeting": {
        "if": [
          { "and": [{ "==": [{ "var": "team" }, "underwriting"] }, { "==": [{ "var": "lob" }, "commercial"] }] },
          "enabled",
          "disabled"
        ]
      }
    }
  }
}
```

**Entity front-book targeting**:

```json
{
  "flags": {
    "new-pricing": {
      "state": "ENABLED",
      "variants": { "new": true, "old": false },
      "defaultVariant": "old",
      "targeting": {
        "if": [{ ">": [{ "var": "entity_created_at" }, "2026-04-01T00:00:00Z"] }, "new", "old"]
      }
    }
  }
}
```

**Combined: team + date**:

```json
{
  "targeting": {
    "if": [
      {
        "and": [
          { "==": [{ "var": "team" }, "underwriting"] },
          { ">": [{ "var": "entity_created_at" }, "2026-04-01T00:00:00Z"] }
        ]
      },
      "enabled",
      "disabled"
    ]
  }
}
```

**Targeting engine**: Modified JSON Logic. Operators: `==`, `!=`, `<`, `<=`, `>`, `>=`, `and`, `or`, `not`, `in`,
`starts_with`, `ends_with`, `contains`. Special: `fractional` (deterministic percentage rollout), `sem_ver` (semantic
version comparison).

**Multi-component access**: OpenFeature Go SDK + flagd provider (gRPC/HTTP), OpenFeature Web SDK + flagd web provider
(HTTP/OFREP). Kubernetes: OpenFeature Operator auto-injects flagd sidecar.

**Infrastructure**: Single Docker container. Ports: 8013 (gRPC/HTTP), 8014 (metrics), 8016 (OFREP). No database. Can run
as sidecar, standalone, or in-process.

**Management UI**: None. JSON file editing only. Designed for GitOps/config-as-code.

**Dependency footprint**: Light. The flagd Go provider (`go-sdk-contrib/providers/flagd`) uses gRPC to connect to flagd.
No heavy cloud SDKs.

**Additional features**: In-process evaluation mode (embed flagd engine in Go app), offline mode (load from local file),
Kubernetes-native via OpenFeature Operator, gRPC streaming for real-time updates.

---

### 2.3 Flipt

**How it works**: Single Go binary with built-in UI, REST API, and gRPC API. Flags stored in database (SQLite/PostgreSQL)
or Git (v2). Evaluation via segments with typed constraints.

**Architecture**:

```text
Go Service ──(Flipt Go SDK, gRPC/HTTP)──┐
                                          ├── Flipt ── DB or Git storage
Web UI ──(Flipt JS SDK / REST API)───────┘    (Docker, ports 8080/9000)
                                              Built-in management UI at :8080
```

**User-level targeting**:

```yaml
segments:
  - key: specific-user
    match_type: ANY_MATCH_TYPE
    constraints:
      - type: ENTITY_COMPARISON_TYPE
        operator: eq
        value: "user-123"
```

**Team/LOB targeting (AND)**:

```yaml
segments:
  - key: underwriting-commercial
    match_type: ALL_MATCH_TYPE
    constraints:
      - property: team
        type: STRING_COMPARISON_TYPE
        operator: eq
        value: "underwriting"
      - property: lob
        type: STRING_COMPARISON_TYPE
        operator: eq
        value: "commercial"
```

**Entity front-book targeting (DateTime)**:

```yaml
segments:
  - key: front-book-2026
    match_type: ANY_MATCH_TYPE
    constraints:
      - property: entity_created_at
        type: DATETIME_COMPARISON_TYPE
        operator: gt
        value: "2026-04-01T00:00:00Z"
```

**Combined: team + LOB + date**:

```yaml
segments:
  - key: underwriting-commercial-2026
    match_type: ALL_MATCH_TYPE
    constraints:
      - property: team
        type: STRING_COMPARISON_TYPE
        operator: eq
        value: "underwriting"
      - property: lob
        type: STRING_COMPARISON_TYPE
        operator: eq
        value: "commercial"
      - property: entity_created_at
        type: DATETIME_COMPARISON_TYPE
        operator: gte
        value: "2026-04-01T00:00:00Z"

flags:
  - key: front-book-redesign
    type: BOOLEAN_FLAG_TYPE
    enabled: true
    rollouts:
      - segment:
          key: underwriting-commercial-2026
          value: true
        rank: 1
```

**Constraint types**: STRING (eq, neq, contains, prefix, suffix, in), NUMBER (eq, neq, lt, lte, gt, gte), BOOLEAN (eq,
neq), DATETIME (eq, neq, lt, lte, gt, gte), ENTITY (eq, neq, in). Match types: ALL_MATCH_TYPE (AND), ANY_MATCH_TYPE (OR).

**Multi-component access**: Flipt Go SDK (`go.flipt.io/flipt/sdk/go`) with gRPC or HTTP transport. JS SDK
(`@flipt-io/flipt-client`) for Node.js. Browser WASM client (`@flipt-io/flipt-client-js`) for client-side evaluation.
React hooks (`@flipt-io/flipt-client-react`).

**Infrastructure**: Single binary. No external deps for SQLite backend. Optional PostgreSQL for HA. Docker image:
`docker.flipt.io/flipt/flipt:v2`. Ports: 8080 (REST + UI), 9000 (gRPC).

**Management UI**: Built-in React/TailwindCSS UI. Create/edit flags, segments, rules. Visual percentage sliders.
Evaluation console for testing. Namespace support. v2 adds Git-backed flag management with branching.

**Dependency footprint**: Moderate. The Go SDK uses protobuf/gRPC (`google.golang.org/grpc`,
`google.golang.org/protobuf`). Standard for any gRPC service.

**Additional features**: Flipt v2 is Git-native (flags stored in Git, UI writes become Git commits). Batch evaluation.
Namespace isolation. OpenFeature provider. SSE streaming for real-time updates. OFREP support.

---

### 2.4 Unleash

**How it works**: Central server with PostgreSQL. Admin UI for flag management. Backend SDKs cache flags in-memory,
evaluate locally. Frontend uses Unleash Proxy/Edge.

**Architecture**:

```text
Go Service ──(Unleash Go SDK, HTTP polling)──┐
                                              ├── Unleash Server ── PostgreSQL
Web UI ──(Unleash Proxy/Edge + JS SDK)───────┘    (Docker, port 3000)
                                                  Built-in admin UI at :3000
```

**User-level targeting**:

```json
{
  "name": "userWithId",
  "parameters": { "userIds": "user-123,user-456" }
}
```

**Team/LOB targeting (constraints = AND)**:

```json
{
  "name": "flexibleRollout",
  "parameters": { "rollout": 100, "stickiness": "userId" },
  "constraints": [
    { "contextName": "team", "operator": "IN", "values": ["underwriting"] },
    { "contextName": "lob", "operator": "IN", "values": ["commercial"] }
  ]
}
```

**Entity front-book targeting**:

```json
{
  "name": "flexibleRollout",
  "parameters": { "rollout": 100, "stickiness": "userId" },
  "constraints": [{ "contextName": "entity_created_at", "operator": "DATE_AFTER", "values": ["2026-04-01T00:00:00Z"] }]
}
```

**Combined: team + date**:

```json
{
  "constraints": [
    { "contextName": "team", "operator": "IN", "values": ["underwriting"] },
    { "contextName": "entity_created_at", "operator": "DATE_AFTER", "values": ["2026-04-01T00:00:00Z"] }
  ]
}
```

**Operators**: IN, NOT_IN, STR_CONTAINS, STR_STARTS_WITH, STR_ENDS_WITH, DATE_AFTER, DATE_BEFORE, NUM_EQ, NUM_LT,
NUM_LTE, NUM_GT, NUM_GTE, SEMVER_EQ, SEMVER_LT, SEMVER_GT.

**Multi-component access**: Go SDK (`github.com/Unleash/unleash-go-sdk/v5`) polls and caches flags. JS SDK for Node.js
and browser (via Unleash Proxy/Edge). React SDK. OpenFeature providers for Go and JS.

**Infrastructure**: Unleash Server (Node.js) + PostgreSQL. Heavier than alternatives. For frontend: Unleash Proxy (OSS)
or Edge (Enterprise) required as intermediary.

**Management UI**: Full-featured admin dashboard. Visual strategy builder, drag-and-drop rules. Segment management.
Environment management. Change requests (Enterprise). RBAC. Analytics dashboards.

**Dependency footprint**: Light for Go SDK. The Unleash Go SDK is self-contained with HTTP polling.

**Additional features**: Reusable segments, multiple activation strategies (standard, flexible rollout, gradual rollout,
user-targeting), impact metrics, lifecycle management, project organization, SSO/SCIM (Enterprise).

---

## 3. Hexagonal Architecture Integration

All four solutions integrate with the template's existing architecture the same way:

### Port Interface (already exists)

`internal/ports/featureflags.go` defines the contract:

```go
type FeatureFlags interface {
    IsEnabled(ctx context.Context, flag string, defaultValue bool) bool
    GetString(ctx context.Context, flag string, defaultValue string) string
    GetInt(ctx context.Context, flag string, defaultValue int) int
    GetFloat(ctx context.Context, flag string, defaultValue float64) float64
    GetJSON(ctx context.Context, flag string, target any) error
}
```

### Adapter Pattern

The adapter wraps the chosen SDK and implements `ports.FeatureFlags`:

```go
// internal/adapters/featureflags/openfeature_adapter.go
type OpenFeatureAdapter struct {
    client *openfeature.Client
    logger *slog.Logger
}

func (a *OpenFeatureAdapter) IsEnabled(ctx context.Context, flag string, defaultValue bool) bool {
    user := ports.GetFeatureFlagUser(ctx)
    evalCtx := a.buildEvalContext(user)
    value, err := a.client.BooleanValue(ctx, flag, defaultValue, evalCtx)
    if err != nil {
        a.logger.WarnContext(ctx, "feature flag evaluation failed", "flag", flag, "error", err)
        return defaultValue
    }
    return value
}

func (a *OpenFeatureAdapter) buildEvalContext(user *ports.FeatureFlagUser) openfeature.EvaluationContext {
    if user == nil {
        return openfeature.EvaluationContext{}
    }
    attrs := make(map[string]any)
    for k, v := range user.Attributes {
        attrs[k] = v
    }
    return openfeature.NewEvaluationContext(user.ID, attrs)
}
```

### Context Flow

Targeting attributes flow through the request:

```text
HTTP Request (with headers: X-User-ID, X-Team, etc.)
    |
    v
Middleware: Extract user info, set FeatureFlagUser on context
    |
    v
Application Service: Add entity attributes to context
    ctx = ports.WithFeatureFlagUser(ctx, &ports.FeatureFlagUser{
        ID: userID,
        Attributes: map[string]any{
            "team":               "underwriting",
            "lob":                "commercial",
            "entity_created_at":  entity.CreatedAt.Format(time.RFC3339),
        },
    })
    |
    v
Feature Flag Check: flags.IsEnabled(ctx, "new-pricing-model", false)
    |
    v
Adapter: Extract FeatureFlagUser from ctx, build EvalContext, call provider
```

### Wiring in main.go

```go
// Create OpenFeature provider (swap this line to change providers)
provider, _ := gofeatureflag.NewProvider(gofeatureflag.ProviderOptions{
    Endpoint: cfg.FeatureFlags.Endpoint,
})
openfeature.SetProvider(provider)

// Create adapter implementing ports.FeatureFlags
flagAdapter := featureflags.NewOpenFeatureAdapter(
    openfeature.NewClient("my-service"),
    logger,
)

// Inject into services
svc := app.NewService(repo, client, flagAdapter, &app.ServiceConfig{Logger: logger})
```

---

## 4. Front-book / Back-book Pattern

### Concept

- **Front-book**: New entities (policies, accounts, transactions) created after a rule change date. They operate under
  new rules from creation.
- **Back-book**: Existing entities created before the rule change date. They continue under old rules.

### Implementation Pattern

The key insight: the **entity's creation date** determines which rules apply, not the user's identity. The application
service knows the entity and sets its attributes on the evaluation context before checking flags.

```go
// Application service - process a claim
func (s *ClaimsService) ProcessClaim(ctx context.Context, claimID string) error {
    claim, err := s.repo.GetClaim(ctx, claimID)
    if err != nil {
        return err
    }

    // Set entity attributes for feature flag evaluation
    ctx = ports.WithFeatureFlagUser(ctx, &ports.FeatureFlagUser{
        ID: claim.PolicyHolderID,
        Attributes: map[string]any{
            "team":               s.getCurrentUserTeam(ctx),
            "lob":                claim.LineOfBusiness,
            "entity_created_at":  claim.Policy.CreatedAt.Format(time.RFC3339),
            "policy_type":        claim.Policy.Type,
        },
    })

    // Feature flag determines which pricing rules to use
    if s.flags.IsEnabled(ctx, "new-pricing-model", false) {
        return s.processWithNewPricing(ctx, claim)
    }
    return s.processWithLegacyPricing(ctx, claim)
}
```

### Server-side vs Client-side Evaluation

For the Web UI, the same flag is evaluated client-side:

```javascript
// Web UI - show different UI based on entity's front-book status
const evalContext = {
  targetingKey: userId,
  team: currentUser.team,
  lob: currentPolicy.lineOfBusiness,
  entity_created_at: currentPolicy.createdAt, // ISO 8601
};

const useNewPricingUI = await client.getBooleanValue("new-pricing-model", false, evalContext);
```

Both the Go backend and Web UI evaluate the same flag with the same entity context, ensuring consistent behavior.

---

## 5. Comparison Matrix

| Dimension                     |                                      GO Feature Flag                                      |                                      flagd                                       |                                           Flipt                                            |                                          Unleash                                          |
| ----------------------------- | :---------------------------------------------------------------------------------------: | :------------------------------------------------------------------------------: | :----------------------------------------------------------------------------------------: | :---------------------------------------------------------------------------------------: |
| **Targeting richness**        |                                         Excellent                                         |                                    Excellent                                     |                                         Excellent                                          |                                         Excellent                                         |
|                               |        Query engine with AND/OR, date comparisons, percentage, scheduled rollouts         |            JSON Logic with full boolean algebra, fractional rollouts             | Typed constraints (String, Number, DateTime, Boolean, Entity), segments with Match All/Any |           Strategies + constraints + segments, DATE_AFTER/DATE_BEFORE operators           |
| **Multi-component support**   |                                         Excellent                                         |                                       Good                                       |                                         Excellent                                          |                                           Good                                            |
|                               | Relay proxy with REST/OFREP API. OpenFeature SDKs for Go, JS (web + Node), React, Angular |      gRPC + HTTP/OFREP. OpenFeature SDKs for Go, JS. Web provider available      |             REST + gRPC APIs. Go SDK, JS SDK, Browser WASM client, React hooks             | Server + Proxy/Edge. Go SDK, JS SDK, React SDK. Frontend requires Proxy/Edge intermediary |
| **Infrastructure overhead**   |                                            Low                                            |                                       Low                                        |                                            Low                                             |                                        Medium-High                                        |
|                               |                     Single Docker container. No database. Stateless.                      |                Single Docker container (or sidecar). No database.                |       Single binary. SQLite default (no external deps). Optional PostgreSQL for HA.        |            Server (Node.js) + PostgreSQL required. Frontend needs Proxy/Edge.             |
| **Dependency footprint (Go)** |                              Heavy (core) / Light (provider)                              |                                      Light                                       |                                      Moderate (gRPC)                                       |                                           Light                                           |
|                               |        Core module pulls cloud SDKs. OpenFeature provider is HTTP-only and light.         |                   flagd provider uses gRPC. Standard Go deps.                    |                          SDK uses protobuf/gRPC. ~15-20 modules.                           |                             HTTP polling SDK. Self-contained.                             |
| **OpenFeature compatibility** |                                          Native                                           |                             Native (reference impl)                              |                                     Provider available                                     |                                    Provider available                                     |
|                               |                  Official OpenFeature Go + JS providers. OFREP support.                   |              _Is_ the OpenFeature reference. Full spec compliance.               |               Go + JS OpenFeature providers in contrib repo. OFREP support.                |                     Go + JS OpenFeature providers. Community-driven.                      |
| **Config-as-code (GitOps)**   |                                         Excellent                                         |                                       Good                                       |                                       Excellent (v2)                                       |                                           Poor                                            |
|                               |                YAML flags in Git/S3/HTTP. Relay proxy polls. No UI needed.                |                 JSON flags in files/ConfigMaps. GitOps-friendly.                 |    v2 is Git-native: flags stored in Git, UI writes become commits. Branching support.     |         Flags managed via API/UI. No native Git storage. Export/import possible.          |
| **UI for flag management**    |                                           None                                            |                                       None                                       |                                          Built-in                                          |                                         Built-in                                          |
|                               |         Editor at editor.gofeatureflag.org (external). Config-as-code philosophy.         |                          No UI. File-based management.                           |         Built-in React UI. Create/edit flags, segments, rules. Evaluation console.         |         Full admin dashboard. Visual strategy builder. Drag-and-drop. Analytics.          |
| **Front-book date support**   |                                         Excellent                                         |                                    Excellent                                     |                                         Excellent                                          |                                         Excellent                                         |
|                               |          `entity_created_at gt "date"` in query syntax. Also scheduled rollouts.          |          `{">": [{"var": "entity_created_at"}, "date"]}` in JSON Logic.          |                   DATETIME_COMPARISON_TYPE with gt/gte/lt/lte operators.                   |                       DATE_AFTER/DATE_BEFORE constraint operators.                        |
| **Community / maturity**      |                                           Good                                            |                                       Good                                       |                                            Good                                            |                                         Excellent                                         |
|                               |          Active maintainer (thomaspoignant). Grafana uses it. Growing community.          | CNCF project (OpenFeature). Active development. Reference implementation status. |                Well-funded. Git-native v2 is innovative. Active community.                 |       500K+ Docker pulls. Large enterprise user base. 10+ years. OSS + Enterprise.        |
| **Template suitability**      |                                         Excellent                                         |                                       Good                                       |                                         Excellent                                          |                                           Fair                                            |
|                               |        Lightweight, config-as-code, no infra beyond Docker. Perfect for template.         |        Lightweight but JSON config is more verbose. Great for Kubernetes.        |          Single binary with built-in UI. Excellent for template (self-contained).          |         Requires PostgreSQL. Heavier setup. Better for production than template.          |

---

## 6. Tradeoff Analysis

### GO Feature Flag

|                    |                                                                                                                                                                                                                 |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Strengths**      | YAML config is simple and readable. Scheduled rollouts for time-based activation. Lightweight relay proxy. Percentage rollouts + targeting composable. Notifications (Slack, Teams). Data export for analytics. |
| **Weaknesses**     | No built-in management UI. Heavy core dependency tree (though provider-only is light). Smaller community than Unleash.                                                                                          |
| **Best fit**       | Teams that prefer config-as-code, Git-based workflows, and don't need a UI for flag management.                                                                                                                 |
| **Migration path** | OpenFeature interface means swapping providers only changes `main.go` wiring. OFREP support enables switching to any OFREP-compatible provider.                                                                 |

### flagd

|                    |                                                                                                                                                                                                           |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Strengths**      | Official OpenFeature reference implementation. Lightest infrastructure. In-process mode available (no sidecar needed). Kubernetes-native with OpenFeature Operator. gRPC streaming for real-time updates. |
| **Weaknesses**     | No management UI. JSON Logic configuration is verbose for complex rules. Smaller ecosystem. Less feature-rich than alternatives (no scheduled rollouts, no notifications).                                |
| **Best fit**       | Kubernetes-native deployments. Teams standardizing on OpenFeature. Low-latency requirements (in-process mode).                                                                                            |
| **Migration path** | OpenFeature standard means any provider swap is code-level compatible.                                                                                                                                    |

### Flipt

|                    |                                                                                                                                                                                                                   |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Strengths**      | Built-in management UI. Single binary, zero external deps (SQLite default). Git-native v2 (flags in Git with branching). Typed constraint system (DateTime, Number, etc.). Batch evaluation. Namespace isolation. |
| **Weaknesses**     | gRPC dependency in Go SDK. Smaller OpenFeature provider ecosystem. Less mature than Unleash for enterprise use.                                                                                                   |
| **Best fit**       | Teams wanting a self-contained solution with a management UI. Git-native workflows. Teams that want both UI management and config-as-code.                                                                        |
| **Migration path** | Has OpenFeature provider. OFREP support. Can migrate to/from other providers.                                                                                                                                     |

### Unleash

|                    |                                                                                                                                                                                                |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Strengths**      | Most mature platform. Best management UI. Reusable segments. Impact metrics and analytics. Large community. Enterprise features (RBAC, SSO, change requests). Non-developers can manage flags. |
| **Weaknesses**     | Requires PostgreSQL. Heavier infrastructure. Frontend requires Proxy/Edge. Enterprise features behind paywall ($75/seat/month). More operational overhead.                                     |
| **Best fit**       | Enterprise teams with non-developer flag managers. Organizations needing compliance/audit trails. Multi-team organizations with complex flag governance.                                       |
| **Migration path** | Has OpenFeature provider. REST API for integration. Export/import for flag definitions.                                                                                                        |

---

## References

### GO Feature Flag Resources

- [Official Site](https://gofeatureflag.org/)
- [GitHub](https://github.com/thomaspoignant/go-feature-flag)
- [Relay Proxy Config](https://gofeatureflag.org/docs/relay-proxy/configure-relay-proxy)
- [Targeting Rules](https://gofeatureflag.org/docs/configure_flag/target-with-flags)
- [Scheduled Rollout](https://gofeatureflag.org/docs/configure_flag/rollout-strategies/scheduled)
- [OpenFeature Go Provider](https://gofeatureflag.org/docs/sdk/server_providers/openfeature_go)
- [OpenFeature JS Provider](https://gofeatureflag.org/docs/sdk/client_providers/openfeature_javascript)

### flagd Resources

- [Official Site](https://flagd.dev/)
- [GitHub](https://github.com/open-feature/flagd)
- [Flag Definitions](https://flagd.dev/reference/flag-definitions/)
- [JSON Targeting Schema](https://flagd.dev/reference/schema/)
- [Go Provider](https://pkg.go.dev/github.com/open-feature/go-sdk-contrib/providers/flagd)

### Flipt Resources

- [Official Site](https://www.flipt.io/)
- [GitHub](https://github.com/flipt-io/flipt)
- [Concepts (Constraints/Segments)](https://docs.flipt.io/concepts)
- [Go SDK](https://pkg.go.dev/go.flipt.io/flipt/sdk/go)
- [v2 Introduction](https://docs.flipt.io/v2/introduction)
- [OpenFeature Provider](https://blog.flipt.io/announcing-the-flipt-openfeature-go-provider)

### Unleash Resources

- [Official Site](https://www.getunleash.io/)
- [GitHub](https://github.com/Unleash/unleash)
- [Activation Strategies](https://docs.getunleash.io/reference/activation-strategies)
- [Segments](https://docs.getunleash.io/concepts/segments)
- [Go SDK](https://docs.getunleash.io/reference/sdks/go)
- [Custom Context Fields](https://docs.getunleash.io/how-to/how-to-define-custom-context-fields)

### OpenFeature

- [Official Site](https://openfeature.dev/)
- [Go SDK](https://openfeature.dev/docs/reference/sdks/server/go/)
- [Evaluation Context Spec](https://openfeature.dev/specification/sections/evaluation-context/)
- [CNCF Project Page](https://www.cncf.io/projects/openfeature/)
