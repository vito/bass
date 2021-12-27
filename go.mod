module github.com/vito/bass

go 1.16

require (
	github.com/adrg/xdg v0.3.4
	github.com/ajstarks/svgo v0.0.0-20210406150507-75cfd577ce75
	github.com/alecthomas/chroma v0.9.2
	github.com/c-bata/go-prompt v0.2.6
	github.com/docker/docker v20.10.7+incompatible
	github.com/gertd/go-pluralize v0.1.7
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/google/go-cmp v0.5.6
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jonboulle/clockwork v0.2.2
	github.com/mattn/go-colorable v0.1.8
	github.com/mattn/go-isatty v0.0.13
	github.com/mattn/go-unicodeclass v0.0.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/buildkit v0.9.1-0.20211211190310-8700be396100
	github.com/morikuni/aec v1.0.0
	github.com/neovim/go-client v1.1.8-0.20210614202543-e3638e2a1819
	github.com/opencontainers/go-digest v1.0.0
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/sourcegraph/jsonrpc2 v0.1.0
	github.com/spf13/cobra v1.2.1
	github.com/spy16/slurp v0.2.3
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/vito/booklit v0.12.1-0.20210822231131-09aacdc3c48f
	github.com/vito/invaders v0.0.2
	github.com/vito/is v0.0.5
	github.com/vito/progrock v0.0.0-20211227224338-39b42111ea3e
	github.com/vito/vt100 v0.0.0-20211217051322-45a31b434dad
	go.etcd.io/bbolt v1.3.6
	go.uber.org/zap v1.19.1
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

// keep in sync with upstream buildkit
replace (
	github.com/docker/docker => github.com/docker/docker v20.10.3-0.20211208011758-87521affb077+incompatible
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace => github.com/tonistiigi/opentelemetry-go-contrib/instrumentation/net/http/httptrace/otelhttptrace v0.0.0-20211026174723-2f82a1e0c997
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => github.com/tonistiigi/opentelemetry-go-contrib/instrumentation/net/http/otelhttp v0.0.0-20211026174723-2f82a1e0c997
)

// fix for progrock in tests
replace github.com/charmbracelet/bubbletea => github.com/vito/bubbletea v0.19.3-0.20211219025709-ab0c00747aa6
