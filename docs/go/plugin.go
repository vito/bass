package plugin

import (
	"sync"

	"github.com/vito/booklit"
)

func init() {
	booklit.RegisterPlugin("bass-www", Init)
}

type Plugin struct {
	Section *booklit.Section

	toggleID int
	lock     sync.Mutex
}

func Init(section *booklit.Section) booklit.Plugin {
	return &Plugin{
		Section: section,
	}
}
