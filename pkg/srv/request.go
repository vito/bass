package srv

import (
	"fmt"
	"io"
	"net/http"

	"github.com/vito/bass/pkg/bass"
)

func requestToScope(r *http.Request) (*bass.Scope, error) {
	request := bass.NewEmptyScope()

	headers := bass.NewEmptyScope()
	for k := range r.Header {
		headers.Set(bass.Symbol(k), bass.String(r.Header.Get(k)))
	}
	request.Set("headers", headers)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	request.Set("body", bass.String(body))

	request.Set("path", bass.ParseFileOrDirPath(r.URL.Path).ToValue())

	query := bass.NewEmptyScope()
	for k, v := range r.URL.Query() {
		vals, err := bass.ValueOf(v)
		if err != nil {
			return nil, fmt.Errorf("value of %v: %w", v, err)
		}

		request.Set(bass.Symbol(k), vals)
	}
	request.Set("query", query)

	return request, nil
}
