package api

import (
	"encoding/json"
	"net/http"
	"reflect"

	"capper/internal/version"
)

// Version is the build-stamped control-plane version (see internal/version).
var Version = version.Version

type Envelope struct {
	Data         any            `json:"data,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	Error        string         `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeData(w http.ResponseWriter, data any, caps map[string]bool) {
	if data != nil {
		if v := reflect.ValueOf(data); v.Kind() == reflect.Slice && v.IsNil() {
			data = reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}
	}
	writeJSON(w, http.StatusOK, Envelope{Data: data, Capabilities: caps})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, Envelope{Error: msg})
}

func writeForbidden(w http.ResponseWriter, err error) {
	writeError(w, http.StatusForbidden, err.Error())
}

func writeNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, msg)
}

func writeBadRequest(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeInternal(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, err.Error())
}
