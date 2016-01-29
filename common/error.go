package common

import "fmt"

type ErrMissingParam string

func (emp ErrMissingParam) Error() string {
	return fmt.Sprintf("param %s not given", string(emp))
}

type ErrMissingParameterMap struct{}

func (emp ErrMissingParameterMap) Error() string {
	return fmt.Sprintf("parameter map not passed from engine")
}

type ErrNetlinkError struct {
	Action string
	Err    error
}

func (ene ErrNetlinkError) Error() string {
	return fmt.Sprintf("netlink: %s error: %s", ene.Action, ene.Err)
}
