// Code generated by mockery. DO NOT EDIT.

package tcontainer_mocks

import (
	tcontainer "github.com/kiteggrad/tcontainer"
	mock "github.com/stretchr/testify/mock"
)

// PruneOption is an autogenerated mock type for the PruneOption type
type PruneOption struct {
	mock.Mock
}

type PruneOption_Expecter struct {
	mock *mock.Mock
}

func (_m *PruneOption) EXPECT() *PruneOption_Expecter {
	return &PruneOption_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: options
func (_m *PruneOption) Execute(options *tcontainer.PruneOptions) error {
	ret := _m.Called(options)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*tcontainer.PruneOptions) error); ok {
		r0 = rf(options)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PruneOption_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type PruneOption_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - options *tcontainer.PruneOptions
func (_e *PruneOption_Expecter) Execute(options interface{}) *PruneOption_Execute_Call {
	return &PruneOption_Execute_Call{Call: _e.mock.On("Execute", options)}
}

func (_c *PruneOption_Execute_Call) Run(run func(options *tcontainer.PruneOptions)) *PruneOption_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*tcontainer.PruneOptions))
	})
	return _c
}

func (_c *PruneOption_Execute_Call) Return(err error) *PruneOption_Execute_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *PruneOption_Execute_Call) RunAndReturn(run func(*tcontainer.PruneOptions) error) *PruneOption_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewPruneOption creates a new instance of PruneOption. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPruneOption(t interface {
	mock.TestingT
	Cleanup(func())
}) *PruneOption {
	mock := &PruneOption{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}