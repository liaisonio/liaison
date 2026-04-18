package dao

import (
	"errors"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func (d *dao) GetFirewallRuleByProxyID(proxyID uint) (*model.ProxyFirewallRule, error) {
	var rule model.ProxyFirewallRule
	err := d.getDB().Where("proxy_id = ?", proxyID).First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &rule, err
}

// UpsertFirewallRule updates the existing rule for rule.ProxyID if one
// exists, otherwise creates a new row. UserID/AllowedCIDRs are overwritten.
//
// Looks at soft-deleted rows too — the UNIQUE index on proxy_id is not
// composite with deleted_at, so a stale soft-deleted row would collide
// with a new Create. When we find such a row, we resurrect it (clear
// DeletedAt) and Save over the fields.
func (d *dao) UpsertFirewallRule(rule *model.ProxyFirewallRule) error {
	var existing model.ProxyFirewallRule
	err := d.getDB().
		Unscoped().
		Where("proxy_id = ?", rule.ProxyID).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d.getDB().Create(rule).Error
	}
	if err != nil {
		return err
	}
	existing.UserID = rule.UserID
	existing.AllowedCIDRs = rule.AllowedCIDRs
	existing.DeletedAt = gorm.DeletedAt{} // resurrect if soft-deleted
	return d.getDB().Unscoped().Save(&existing).Error
}

// DeleteFirewallRuleByProxyID hard-deletes the row so subsequent Upserts
// for the same proxy_id cannot hit the UNIQUE index via stale soft-deleted
// rows. Firewall rules carry no audit value that justifies soft delete.
func (d *dao) DeleteFirewallRuleByProxyID(proxyID uint) error {
	return d.getDB().
		Unscoped().
		Where("proxy_id = ?", proxyID).
		Delete(&model.ProxyFirewallRule{}).Error
}

func (d *dao) ListFirewallRulesByUserID(userID uint) ([]*model.ProxyFirewallRule, error) {
	var rules []*model.ProxyFirewallRule
	err := d.getDB().Where("user_id = ?", userID).Find(&rules).Error
	return rules, err
}

// ListAllFirewallRules returns every rule in the table — used on startup to
// rehydrate the data-plane firewall state after the process restarts.
func (d *dao) ListAllFirewallRules() ([]*model.ProxyFirewallRule, error) {
	var rules []*model.ProxyFirewallRule
	err := d.getDB().Find(&rules).Error
	return rules, err
}
