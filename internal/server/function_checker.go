package server

import "github.com/watzon/alyx/internal/functions"

type FunctionChecker struct {
	funcService *functions.Service
}

func NewFunctionChecker(funcService *functions.Service) *FunctionChecker {
	return &FunctionChecker{funcService: funcService}
}

func (f *FunctionChecker) FunctionExists(name string) bool {
	if f.funcService == nil {
		return false
	}
	_, exists := f.funcService.GetFunction(name)
	return exists
}
