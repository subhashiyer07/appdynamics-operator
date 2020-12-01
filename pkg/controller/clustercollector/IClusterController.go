package clustercollector

import "github.com/go-logr/logr"

type IClusterController interface {
	Init(logger logr.Logger) (bool,error)
	Update(logger logr.Logger) (bool, error)
	Create(logger logr.Logger) error
	RestartCollector() error
}
