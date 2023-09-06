module github.com/vito/bass

go 1.20

require (
	dagger.io/dagger v0.8.2
	github.com/Khan/genqlient v0.6.0
	github.com/iancoleman/strcase v0.3.0
	github.com/vektah/gqlparser/v2 v2.5.6
)

require (
	github.com/99designs/gqlgen v0.17.31 // indirect
	golang.org/x/sync v0.3.0 // indirect
)

// BEGIN SYNC buildkit

// END SYNC
replace dagger.io/dagger => github.com/vito/dagger/sdk/go v0.0.0-20230827023827-b95056f5ca03
