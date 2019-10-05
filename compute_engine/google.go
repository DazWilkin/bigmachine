package compute_engine

import (
	"github.com/grailbio/bigmachine"
)

type Instance struct{}

func (i *Instance) Create(name string) (*bigmachine.Machine, error) {
	return &bigmachine.Machine{}, nil
}
func (i *Instance) Machine() *bigmachine.Machine {
	return nil
}

func (i *Instance) MachineType() string {
	return ""
}
