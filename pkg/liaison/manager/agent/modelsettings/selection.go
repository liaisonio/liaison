package modelsettings

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"slices"
)

// Choice 仅包含可公开的模型标识，不包含服务地址、密钥或密钥状态。
type Choice struct {
	runtime.ModelSelection
	ProviderType string `json:"provider_type"`
	IsDefault    bool   `json:"is_default"`
}

func (m *Manager) Choices(ctx context.Context) ([]Choice, error) {
	c, err := m.load(ctx)
	if err != nil {
		return nil, err
	}
	result := []Choice{}
	if !c.Enabled {
		return result, nil
	}
	c = normalize(c)
	for _, p := range c.Providers {
		for _, name := range p.Models {
			result = append(result, Choice{runtime.ModelSelection{ProviderID: p.ID, Model: name}, p.Type, p.ID == c.DefaultProvider && name == p.Model})
		}
	}
	return result, nil
}

func resolveSelection(c Config, selection runtime.ModelSelection) (Config, string, runtime.ModelSelection, error) {
	c = normalize(c)
	if !c.Enabled {
		return Config{}, "", runtime.ModelSelection{}, ErrDisabled
	}
	if selection.ProviderID == "" && selection.Model == "" {
		selection.ProviderID = c.DefaultProvider
	}
	for _, p := range c.Providers {
		if p.ID != selection.ProviderID {
			continue
		}
		if selection.Model == "" {
			selection.Model = p.Model
		}
		if !slices.Contains(p.Models, selection.Model) {
			break
		}
		return Config{Enabled: true, BaseURL: p.BaseURL, Model: selection.Model, APIKey: p.APIKey}, p.Type, selection, nil
	}
	return Config{}, "", runtime.ModelSelection{}, ErrInvalid
}

func (m *Manager) ResolveSelection(ctx context.Context, selection runtime.ModelSelection) (runtime.ModelSelection, error) {
	c, err := m.load(ctx)
	if err != nil {
		return runtime.ModelSelection{}, err
	}
	_, _, resolved, err := resolveSelection(c, selection)
	return resolved, err
}
