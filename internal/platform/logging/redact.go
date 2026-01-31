package logging

import (
	"log/slog"
	"regexp"

	"github.com/m-mizutani/masq"
)

// Common regex patterns for sensitive data.
var (
	// JWT pattern: three base64 segments separated by dots
	jwtPattern = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*$`)

	// Bearer token pattern
	bearerPattern = regexp.MustCompile(`(?i)^bearer\s+.+$`)

	// Basic auth pattern
	basicAuthPattern = regexp.MustCompile(`(?i)^basic\s+.+$`)
)

// DefaultRedactOptions returns the default masq options for secret redaction.
// Projects should extend this list based on their specific requirements.
//
// To add project-specific redaction, combine with additional options:
//
//	opts := append(logging.DefaultRedactOptions(),
//	    masq.WithFieldName("MySecretField"),
//	    masq.WithType[MySecretType](),
//	)
func DefaultRedactOptions() []masq.Option {
	return []masq.Option{
		// Common sensitive field names
		masq.WithFieldName("password"),
		masq.WithFieldName("secret"),
		masq.WithFieldName("token"),
		masq.WithFieldName("apiKey"),
		masq.WithFieldName("apikey"),
		masq.WithFieldName("api_key"),
		masq.WithFieldName("accessToken"),
		masq.WithFieldName("access_token"),
		masq.WithFieldName("refreshToken"),
		masq.WithFieldName("refresh_token"),
		masq.WithFieldName("credential"),
		masq.WithFieldName("credentials"),
		masq.WithFieldName("authorization"),
		masq.WithFieldName("auth"),
		masq.WithFieldName("bearer"),
		masq.WithFieldName("cookie"),
		masq.WithFieldName("session"),
		masq.WithFieldName("privateKey"),
		masq.WithFieldName("private_key"),
		masq.WithFieldName("secretKey"),
		masq.WithFieldName("secret_key"),

		// Field name prefixes for sensitive data
		masq.WithFieldPrefix("secret"),
		masq.WithFieldPrefix("private"),

		// Regex patterns for sensitive values
		masq.WithRegex(jwtPattern),
		masq.WithRegex(bearerPattern),
		masq.WithRegex(basicAuthPattern),
	}
}

// NewReplaceAttr creates a ReplaceAttr function for slog.HandlerOptions
// that redacts sensitive data. Uses DefaultRedactOptions which can be
// extended for project-specific needs.
//
// Usage:
//
//	opts := &slog.HandlerOptions{
//	    ReplaceAttr: logging.NewReplaceAttr(),
//	}
//	handler := slog.NewJSONHandler(os.Stdout, opts)
func NewReplaceAttr(opts ...masq.Option) func(groups []string, a slog.Attr) slog.Attr {
	allOpts := append(DefaultRedactOptions(), opts...)
	return masq.New(allOpts...)
}
