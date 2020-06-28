package controller

import (
	"github.com/mgoltzsche/ktransform/pkg/controller/secrettransform"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, secrettransform.Add)
}
