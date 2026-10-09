package web

import "net/http"

// Only used for the bounded JSON API, never for IDE streams or upgrades.
type ideResponseStatus struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *ideResponseStatus) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *ideResponseStatus) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
