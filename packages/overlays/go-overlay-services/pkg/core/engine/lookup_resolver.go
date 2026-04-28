package engine

import (
	"context"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
)

// LookupResolver wraps the underlying lookup.LookupResolver to expose
// a simplified interface for querying and managing SLAP trackers.
type LookupResolver struct {
	resolver *lookup.LookupResolver
}

// NewLookupResolver creates and initializes a LookupResolver with a default HTTPS facilitator.
func NewLookupResolver() *LookupResolver {
	return NewLookupResolverWithNetwork(overlay.NetworkMainnet)
}

// NewLookupResolverWithNetwork creates and initializes a LookupResolver with a default HTTPS facilitator
// and appropriate SLAP trackers for the specified network.
func NewLookupResolverWithNetwork(network overlay.Network) *LookupResolver {
	cfg := &lookup.LookupResolver{
		Facilitator: &lookup.HTTPSOverlayLookupFacilitator{
			Client: http.DefaultClient,
		},
		NetworkPreset: network,
	}

	// Use NewLookupResolver from the go-sdk to get proper network defaults
	resolver := lookup.NewLookupResolver(cfg)

	return &LookupResolver{
		resolver: resolver,
	}
}

// SetSLAPTrackers configures the SLAP trackers for the resolver.
// If the given slice is empty, it leaves the resolver unchanged.
func (l *LookupResolver) SetSLAPTrackers(trackers []string) {
	if len(trackers) == 0 {
		return
	}
	l.resolver.SLAPTrackers = trackers
}

// SLAPTrackers returns the currently configured SLAP trackers.
func (l *LookupResolver) SLAPTrackers() []string {
	return l.resolver.SLAPTrackers
}

// Query performs a lookup using the configured resolver with the given question and timeout.
func (l *LookupResolver) Query(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	return l.resolver.Query(ctx, question)
}
