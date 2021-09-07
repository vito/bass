package plugin

import (
	"sync"

	"github.com/gertd/go-pluralize"
	"github.com/vito/booklit"
	"github.com/vito/booklit/baselit"
)

func init() {
	booklit.RegisterPlugin("bass-www", Init)
}

type Plugin struct {
	Section *booklit.Section
	Base    *baselit.Plugin

	toggleID int
	paraID   int
	lock     sync.Mutex

	plural *pluralize.Client
}

func Init(section *booklit.Section) booklit.Plugin {
	plural := pluralize.NewClient()
	plural.AddSingularRule("cons", "cons")

	return &Plugin{
		Section: section,
		plural:  plural,
	}
}
