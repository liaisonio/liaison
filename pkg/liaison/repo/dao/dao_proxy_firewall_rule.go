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
func (d *dao) UpsertFirewallRule(rule *model.ProxyFirewallRule) error {
	existing, err := d.GetFirewallRuleByProxyID(rule.ProxyID)
	if err != nil {
		return err
	}
	if existing != nil {
		existing.UserID = rule.UserID
		existing.AllowedCIDRs = rule.AllowedCIDRs
		return d.getDB().Save(existing).Error
	}
	return d.getDB().Create(rule).Error
}

func (d *dao) DeleteFirewallRuleByProxyID(proxyID uint) error {
	return d.getDB().
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
