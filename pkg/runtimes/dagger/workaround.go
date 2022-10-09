package dagger

import (
	"github.com/Khan/genqlient/graphql"
	"go.dagger.io/dagger/sdk/go/dagger/querybuilder"
)

// TODO(vito): why are these not being added?
type CacheID string

type CacheVolume struct {
	q *querybuilder.Selection
	c graphql.Client
}
