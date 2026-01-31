# Secret Redaction in Logs

This service uses [masq](https://github.com/m-mizutani/masq) to automatically redact sensitive data from logs.

> **Warning:** Automatic redaction is not foolproof. Developers must understand and extend the redaction
> rules for their specific use cases.

## How It Works

The logging package wraps slog handlers with masq, which inspects logged values and redacts matches based on:

1. **Field names** - Exact matches like `password`, `token`, `apiKey`
2. **Field prefixes** - Fields starting with `secret`, `private`
3. **Regex patterns** - Values matching JWT, Bearer token, Basic auth patterns
4. **Struct tags** - Fields tagged with `masq:"secret"`
5. **Custom types** - Types registered for redaction

## Default Redaction (Built-in)

The following are redacted by default:

### Field Names

- `password`, `secret`, `token`
- `apiKey`, `api_key`, `apikey`
- `accessToken`, `access_token`
- `refreshToken`, `refresh_token`
- `credential`, `credentials`
- `authorization`, `auth`, `bearer`
- `cookie`, `session`
- `privateKey`, `private_key`
- `secretKey`, `secret_key`

### Field Prefixes

- `secret*` (e.g., `secretValue`, `secretData`)
- `private*` (e.g., `privateKey`, `privateToken`)

### Value Patterns

- JWT tokens (`eyJ...`)
- Bearer tokens (`Bearer ...`)
- Basic auth (`Basic ...`)

## Adding Project-Specific Redaction

### Option 1: Use Struct Tags (Recommended)

Mark sensitive fields in your structs:

```go
type UserCredentials struct {
    Username string `json:"username"`
    Password string `json:"password" masq:"secret"`
    APIKey   string `json:"api_key" masq:"secret"`
}
```

### Option 2: Register Custom Types

For types that are always sensitive:

```go
type APIToken string

opts := append(logging.DefaultRedactOptions(),
    masq.WithType[APIToken](),
)
handler := logging.NewRedactingHandler(baseHandler, opts...)
```

### Option 3: Add Field Names

For additional field names:

```go
opts := append(logging.DefaultRedactOptions(),
    masq.WithFieldName("ssn"),
    masq.WithFieldName("creditCard"),
    masq.WithFieldName("cvv"),
)
handler := logging.NewRedactingHandler(baseHandler, opts...)
```

### Option 4: Add Regex Patterns

For value patterns:

```go
// SSN pattern
ssnPattern := regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)

opts := append(logging.DefaultRedactOptions(),
    masq.WithRegex(ssnPattern),
)
```

## Developer Responsibilities

1. **Review logged data** - Before adding log statements, consider what data is being logged
2. **Tag sensitive structs** - Use `masq:"secret"` tags on sensitive fields
3. **Extend redaction rules** - Add project-specific patterns in your initialization code
4. **Test redaction** - Write tests that verify secrets are redacted
5. **Never log raw requests/responses** - Parse and selectively log safe fields

## What Redaction Does NOT Catch

- Secrets in free-form string messages (use structured logging)
- Secrets in field names not in the default list
- Secrets in nested maps with dynamic keys
- Base64-encoded secrets (unless they match known patterns)
- Encrypted values that look like random strings

## Example: Safe Logging

```go
// BAD: Logging raw request body
slog.Info("received request", "body", string(requestBody))

// GOOD: Logging specific safe fields
slog.Info("received request",
    "method", req.Method,
    "path", req.URL.Path,
    "user_id", claims.Subject,
)

// BAD: Logging entire struct with secrets
slog.Info("user authenticated", "user", user)

// GOOD: Using struct with masq tags
type SafeUser struct {
    ID       string `json:"id"`
    Email    string `json:"email"`
    Password string `json:"-" masq:"secret"` // Won't be logged
}
```

## Testing Redaction

```go
func TestSecretRedaction(t *testing.T) {
    var buf bytes.Buffer
    handler := logging.NewRedactingHandler(
        slog.NewJSONHandler(&buf, nil),
    )
    logger := slog.New(handler)

    logger.Info("test", "password", "supersecret")

    output := buf.String()
    assert.NotContains(t, output, "supersecret")
    assert.Contains(t, output, "[REDACTED]")
}
```
