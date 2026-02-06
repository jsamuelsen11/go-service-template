package context

import "context"

// DataProvider defines a typed data fetching contract.
// Useful for pre-registering known data sources.
type DataProvider interface {
	// Key returns the cache key for this provider.
	Key() string

	// Fetch retrieves the data.
	Fetch(ctx context.Context) (any, error)
}

// GetOrFetchProvider is a convenience method for DataProvider types.
func (rc *RequestContext) GetOrFetchProvider(provider DataProvider) (any, error) {
	return rc.GetOrFetch(provider.Key(), provider.Fetch)
}
