package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type iamUserJSON struct {
	ID        uint              `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Status    model.UserStatus  `json:"status"`
	Role      model.IAMRoleCode `json:"role"`
	CreatedAt string            `json:"created_at"`
	LastLogin string            `json:"last_login,omitempty"`
	LoginIP   string            `json:"login_ip,omitempty"`
}

type organizationJSON struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    *uint  `json:"parent_id,omitempty"`
	CreatedAt   string `json:"created_at"`
	CanManage   bool   `json:"can_manage"`
	CanDelete   bool   `json:"can_delete"`
}

type membershipJSON struct {
	ID             uint                   `json:"id"`
	OrganizationID uint                   `json:"organization_id"`
	UserID         uint                   `json:"user_id"`
	Role           model.OrganizationRole `json:"role"`
	User           *iamUserJSON           `json:"user,omitempty"`
}

func (web *web) handleAccountHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": web.toIAMUser(user)})
}

func (web *web) toIAMUser(user *model.User) *iamUserJSON {
	if user == nil {
		return nil
	}
	role, _ := web.iamService.UserRole(user.ID)
	item := &iamUserJSON{ID: user.ID, Name: user.Name, Email: user.Email, Status: user.Status, Role: role, CreatedAt: user.CreatedAt.Format(time.DateTime), LoginIP: user.LoginIP}
	if user.LastLogin != nil {
		item.LastLogin = user.LastLogin.Format(time.DateTime)
	}
	return item
}

func toOrganization(org *model.Organization) organizationJSON {
	return organizationJSON{ID: org.ID, Name: org.Name, Description: org.Description, ParentID: org.ParentID, CreatedAt: org.CreatedAt.Format(time.DateTime)}
}

func (web *web) handleUsersHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, total, err := web.iamService.ListUsersFor(actor)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		items := make([]*iamUserJSON, 0, len(users))
		for _, user := range users {
			items = append(items, web.toIAMUser(user))
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": map[string]any{"users": items, "total": total}})
	case http.MethodPost:
		var input struct {
			Name, Email, Password string
			Role                  model.IAMRoleCode `json:"role"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		user, generated, err := web.iamService.CreateUserFor(actor, input.Name, input.Email, input.Password, input.Role)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"code": 201, "message": "created", "data": map[string]any{"user": web.toIAMUser(user), "initial_password": generated}})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (web *web) handleUserHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	id, err := pathUint(r, "id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	switch r.Method {
	case http.MethodPatch, http.MethodPut:
		var input struct {
			Name   string             `json:"name"`
			Status model.UserStatus   `json:"status"`
			Role   *model.IAMRoleCode `json:"role"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		user, err := web.iamService.UpdateUserFor(actor, id, input.Name, input.Status, input.Role)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": web.toIAMUser(user)})
	case http.MethodDelete:
		if err := web.iamService.DeleteUserFor(actor, id); err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (web *web) handleUserPasswordHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	id, err := pathUint(r, "id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeIAMError(w, err)
		return
	}
	if err := web.iamService.ResetUserPasswordFor(actor, id, input.Password); err != nil {
		writeIAMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "password updated"})
}

func (web *web) handleOrganizationsHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		orgs, err := web.iamService.ListOrganizationsFor(actor)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		items := make([]organizationJSON, 0, len(orgs))
		for _, org := range orgs {
			item := toOrganization(org)
			item.CanManage, err = web.iamService.CanManageOrganization(actor, org.ID)
			if err != nil {
				writeIAMError(w, err)
				return
			}
			item.CanDelete, err = web.iamService.CanDeleteOrganization(actor, org.ID)
			if err != nil {
				writeIAMError(w, err)
				return
			}
			items = append(items, item)
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": map[string]any{"organizations": items}})
	case http.MethodPost:
		var input struct {
			Name, Description string
			ParentID          *uint `json:"parent_id"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		org, err := web.iamService.CreateOrganizationFor(actor, input.Name, input.Description, input.ParentID)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"code": 201, "message": "created", "data": toOrganization(org)})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (web *web) handleOrganizationHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	id, err := pathUint(r, "id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	switch r.Method {
	case http.MethodPatch, http.MethodPut:
		var input struct {
			Name, Description string
			ParentID          *uint `json:"parent_id"`
			SetParent         bool  `json:"set_parent"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		org, err := web.iamService.UpdateOrganizationFor(actor, id, input.Name, input.Description, input.ParentID, input.SetParent)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": toOrganization(org)})
	case http.MethodDelete:
		if err := web.iamService.DeleteOrganizationFor(actor, id); err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "deleted"})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (web *web) handleOrganizationMembersHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	organizationID, err := pathUint(r, "id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		members, err := web.iamService.ListOrganizationMembersFor(actor, organizationID)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		items := make([]membershipJSON, 0, len(members))
		for _, member := range members {
			items = append(items, membershipJSON{ID: member.ID, OrganizationID: member.OrganizationID, UserID: member.UserID, Role: member.Role, User: web.toIAMUser(member.User)})
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": map[string]any{"members": items}})
	case http.MethodPost:
		var input struct {
			UserID uint                   `json:"user_id"`
			Email  string                 `json:"email"`
			Role   model.OrganizationRole `json:"role"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		var member *model.OrganizationMembership
		if input.UserID != 0 {
			member, err = web.iamService.UpsertOrganizationMemberFor(actor, organizationID, input.UserID, input.Role)
		} else {
			member, err = web.iamService.UpsertOrganizationMemberByEmailFor(actor, organizationID, input.Email, input.Role)
		}
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": membershipJSON{ID: member.ID, OrganizationID: member.OrganizationID, UserID: member.UserID, Role: member.Role}})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (web *web) handleOrganizationMemberHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	organizationID, err := pathUint(r, "id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	userID, err := pathUint(r, "user_id")
	if err != nil {
		writeIAMError(w, err)
		return
	}
	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		var input struct {
			Role model.OrganizationRole `json:"role"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeIAMError(w, err)
			return
		}
		member, err := web.iamService.UpsertOrganizationMemberFor(actor, organizationID, userID, input.Role)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": membershipJSON{ID: member.ID, OrganizationID: member.OrganizationID, UserID: member.UserID, Role: member.Role}})
	case http.MethodDelete:
		if err := web.iamService.RemoveOrganizationMemberFor(actor, organizationID, userID); err != nil {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 200, "message": "removed"})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.Join(iam.ErrInvalid, err)
	}
	return nil
}

func pathUint(r *http.Request, key string) (uint, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	raw := ""
	if key == "user_id" && len(parts) > 0 {
		raw = parts[len(parts)-1]
	} else {
		for index, part := range parts {
			if (part == "users" || part == "organizations") && index+1 < len(parts) {
				raw = parts[index+1]
				break
			}
		}
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.Join(iam.ErrInvalid, errors.New("invalid "+key))
	}
	return uint(value), nil
}

func writeIAMError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, iam.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, iam.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, iam.ErrInvalid):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
}
