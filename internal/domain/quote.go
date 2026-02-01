// Package domain contains core business entities and rules.
package domain

// Quote represents a quotation with its author.
// This is a domain entity - it has no knowledge of external systems.
type Quote struct {
	// ID is the unique identifier for this quote.
	ID string

	// Content is the text of the quote.
	Content string

	// Author is who said or wrote the quote.
	Author string

	// Tags are categories or themes associated with the quote.
	Tags []string
}
