package repo

import (
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
)

type Repo dao.Dao

// Close() error

// // edge
// CreateEdge(edge *model.Edge) error
// GetEdge(id uint64) (*model.Edge, error)
// ListEdges(page, pageSize int) ([]*model.Edge, error)
// UpdateEdge(edge *model.Edge) error
// DeleteEdge(id uint64) error

// CreateDevice(device *model.Device) error
// GetDevice(id uint64) (*model.Device, error)
// ListDevices(page, pageSize int) ([]*model.Device, error)
// UpdateDevice(device *model.Device) error
// DeleteDevice(id uint64) error

// CreateApplication(application *model.Application) error
// GetApplication(id uint64) (*model.Application, error)
// ListApplications(page, pageSize int) ([]*model.Application, error)
// UpdateApplication(application *model.Application) error
// DeleteApplication(id uint64) error

// CreateProxy(proxy *model.Proxy) error
// GetProxy(id uint64) (*model.Proxy, error)
// ListProxies(page, pageSize int) ([]*model.Proxy, error)
// UpdateProxy(proxy *model.Proxy) error
// DeleteProxy(id uint64) error

func NewRepo(config *config.Configuration) (Repo, error) {
	dao, err := dao.NewDao(config)
	if err != nil {
		return nil, err
	}
	return dao, nil
}
