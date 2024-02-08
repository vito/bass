module github.com/vito/bass

go 1.20

require github.com/Khan/genqlient v0.6.0

require github.com/vektah/gqlparser/v2 v2.5.6

require (
	github.com/99designs/gqlgen v0.17.31
	github.com/stretchr/testify v1.8.3 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa
	golang.org/x/sync v0.6.0
)

// BEGIN SYNC buildkit

// END SYNC
replace dagger.io/dagger => github.com/vito/dagger/sdk/go v0.0.0-20230827023827-b95056f5ca03
