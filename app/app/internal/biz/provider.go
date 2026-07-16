package biz

import "github.com/google/wire"

// ProviderSet is the biz layer wire set.
var ProviderSet = wire.NewSet(
	NewUserUseCase,
	NewRecordUseCase,
)