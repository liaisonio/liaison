package web

import "net/http"

// @Summary Get current effective feature permissions
// @Router /api/v1/iam/permissions [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handlePermissionsHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	permissions, err := web.iamService.EffectiveFeatures(actor)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": permissions})
}

// @Summary Read or replace ordinary users' optional feature permissions
// @Router /api/v1/iam/roles/user/permissions [get]
// @Router /api/v1/iam/roles/user/permissions [put]
// @Success 200 {object} map[string]interface{}
func (web *web) handleUserPermissionsHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		var input struct {
			Enabled []string `json:"enabled"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		if err := web.iamService.SetUserFeaturePolicy(actor, input.Enabled); err != nil {
			writeIAMError(w, err)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	policy, err := web.iamService.UserFeaturePolicy(actor)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": policy})
}
