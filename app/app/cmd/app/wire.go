//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/data"
	"github.com/jinniu/app/app/app/internal/server"
	"github.com/jinniu/app/app/app/internal/service"
)

func wireApp(*conf.Bootstrap, *data.Data) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		provideRepos,
	))
}