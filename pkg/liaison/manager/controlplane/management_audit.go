package controlplane

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type ManagementAudit struct {
	UserID     uint
	UserEmail  string
	Module     string
	Action     string
	Resource   string
	Method     string
	ClientIP   string
	Success    bool
	StatusCode int
	ElapsedMS  int64
}

type ManagementAuditListQuery struct {
	Module    string
	Action    string
	Success   *bool
	Keyword   string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

type ManagementAuditEntry struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	UserEmail  string `json:"user_email"`
	Module     string `json:"module"`
	Action     string `json:"action"`
	Resource   string `json:"resource"`
	Method     string `json:"method"`
	ClientIP   string `json:"client_ip"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	CreatedAt  string `json:"created_at"`
}

type ManagementAuditList struct {
	Items    []*ManagementAuditEntry `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func (cp *controlPlane) RecordManagementAudit(ctx context.Context, audit *ManagementAudit) error {
	if audit == nil {
		return errors.New("management audit is nil")
	}
	if audit.UserID == 0 {
		if userID, ok := ctx.Value("user_id").(uint); ok {
			audit.UserID = userID
		}
	}
	if audit.UserEmail == "" {
		if email, ok := ctx.Value("user_email").(string); ok {
			audit.UserEmail = strings.TrimSpace(email)
		}
	}
	if audit.UserID == 0 && audit.UserEmail != "" {
		if user, err := cp.repo.GetUserByEmail(audit.UserEmail); err == nil && user != nil {
			audit.UserID = user.ID
		}
	}
	if audit.UserEmail == "" && audit.UserID > 0 {
		if user, err := cp.repo.GetUserByID(audit.UserID); err == nil && user != nil {
			audit.UserEmail = user.Email
		}
	}
	return cp.repo.CreateManagementAudit(&model.ManagementAudit{
		UserID: audit.UserID, UserEmail: strings.TrimSpace(audit.UserEmail), Module: strings.TrimSpace(audit.Module), Action: strings.TrimSpace(audit.Action), Resource: strings.TrimSpace(audit.Resource), Method: strings.TrimSpace(audit.Method), ClientIP: strings.TrimSpace(audit.ClientIP), Success: audit.Success, StatusCode: audit.StatusCode, ElapsedMS: audit.ElapsedMS,
	})
}

func (cp *controlPlane) ListManagementAudits(ctx context.Context, query *ManagementAuditListQuery) (*ManagementAuditList, error) {
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	if query == nil {
		query = &ManagementAuditListQuery{}
	}
	page, pageSize := query.Page, query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	daoQuery := &dao.ListManagementAuditsQuery{UserID: userID, Module: strings.TrimSpace(query.Module), Action: strings.TrimSpace(query.Action), Success: query.Success, Keyword: strings.TrimSpace(query.Keyword), StartTime: query.StartTime, EndTime: query.EndTime, Limit: pageSize, Offset: (page - 1) * pageSize}
	items, err := cp.repo.ListManagementAudits(daoQuery)
	if err != nil {
		return nil, err
	}
	total, err := cp.repo.CountManagementAudits(daoQuery)
	if err != nil {
		return nil, err
	}
	entries := make([]*ManagementAuditEntry, 0, len(items))
	for _, item := range items {
		entries = append(entries, &ManagementAuditEntry{ID: item.ID, UserID: item.UserID, UserEmail: item.UserEmail, Module: item.Module, Action: item.Action, Resource: item.Resource, Method: item.Method, ClientIP: item.ClientIP, Success: item.Success, StatusCode: item.StatusCode, ElapsedMS: item.ElapsedMS, CreatedAt: item.CreatedAt.Format(time.RFC3339)})
	}
	return &ManagementAuditList{Items: entries, Total: total, Page: page, PageSize: pageSize}, nil
}
