package service

import "github.com/google/wire"

// ProviderSet is the service layer wire set.
var ProviderSet = wire.NewSet(
	NewAppService,
)