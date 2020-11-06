package controller

import (
	"github.com/Appdynamics/appdynamics-operator/pkg/controller/hostcollector"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, hostcollector.Add)
}
