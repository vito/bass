package srv

import (
	"net/http"
)

func Mux(call *CallHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: embed a canonical favicon
		w.WriteHeader(http.StatusNotFound)
	}))
	mux.Handle("/", call)
	return mux
}
