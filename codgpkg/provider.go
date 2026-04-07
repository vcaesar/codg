package codgpkg

import "encoding/json"

// ProviderOptionsData is the constraint for provider-specific option
// values. Implementations must support JSON round-tripping via
// [json.Marshaler] and [json.Unmarshaler].
type ProviderOptionsData interface {
	// Options is a marker method that tags concrete implementations.
	Options()
	json.Marshaler
	json.Unmarshaler
}

// ProviderMetadata maps provider names to their response metadata.
type ProviderMetadata map[string]ProviderOptionsData

// ProviderOptions maps provider names to their request-time options.
type ProviderOptions map[string]ProviderOptionsData
