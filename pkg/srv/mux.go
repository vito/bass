package srv

import (
	"net/http"
)

func Mux(run *RunHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: embed a canonical favicon
		w.WriteHeader(http.StatusNotFound)
	}))
	mux.Handle("/", run)
	return mux
}
