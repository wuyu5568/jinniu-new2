package data

import "github.com/google/wire"

// ProviderSet is the data layer wire set (repos added incrementally).
var ProviderSet = wire.NewSet(
	NewData,
)