package web

import (
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/modelsettings"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"net/http"
)

// handleModelSettingsHTTP manages the global model without exposing credentials.
// @Summary Read, save or test model settings
// @Router /api/v1/settings/model [get]
// @Router /api/v1/settings/model [put]
// @Router /api/v1/settings/model/test [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleModelSettingsHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	service, ok := web.agentService.(interface{ ModelSettings() *modelsettings.Manager })
	if !ok || service.ModelSettings() == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	manager := service.ModelSettings()
	var data modelsettings.View
	switch {
	case r.URL.Path == "/api/v1/settings/model" && r.Method == http.MethodGet:
		data, err = manager.Get(r.Context(), actor.ID)
	case r.URL.Path == "/api/v1/settings/model" && r.Method == http.MethodPut,
		r.URL.Path == "/api/v1/settings/model/test" && r.Method == http.MethodPost:
		var update modelsettings.Update
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&update) != nil {
			err = modelsettings.ErrInvalid
			break
		}
		if r.Method == http.MethodPost {
			err = manager.Test(r.Context(), actor.ID, update)
		} else {
			data, err = manager.Save(r.Context(), actor.ID, update)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		status, message := http.StatusInternalServerError, "Unable to process model settings"
		switch {
		case errors.Is(err, iam.ErrForbidden):
			status, message = http.StatusForbidden, "Only root organization administrators can manage models"
		case errors.Is(err, modelsettings.ErrInvalid):
			status, message = http.StatusBadRequest, "Check model settings; changing endpoint requires a new key or clearing the saved key"
		case errors.Is(err, modelsettings.ErrProbe):
			status, message = http.StatusBadGateway, modelsettings.ErrProbe.Error()
		}
		writeJSON(w, status, map[string]any{"code": status, "message": message})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": data})
}
