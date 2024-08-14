// Code generated by mockery. DO NOT EDIT.

package tcontainer_mocks

import (
	tcontainer "github.com/kiteggrad/tcontainer"
	mock "github.com/stretchr/testify/mock"
)

// RunOption is an autogenerated mock type for the RunOption type
type RunOption struct {
	mock.Mock
}

type RunOption_Expecter struct {
	mock *mock.Mock
}

func (_m *RunOption) EXPECT() *RunOption_Expecter {
	return &RunOption_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: options
func (_m *RunOption) Execute(options *tcontainer.RunOptions) error {
	ret := _m.Called(options)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*tcontainer.RunOptions) error); ok {
		r0 = rf(options)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunOption_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type RunOption_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - options *tcontainer.RunOptions
func (_e *RunOption_Expecter) Execute(options interface{}) *RunOption_Execute_Call {
	return &RunOption_Execute_Call{Call: _e.mock.On("Execute", options)}
}

func (_c *RunOption_Execute_Call) Run(run func(options *tcontainer.RunOptions)) *RunOption_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*tcontainer.RunOptions))
	})
	return _c
}

func (_c *RunOption_Execute_Call) Return(err error) *RunOption_Execute_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *RunOption_Execute_Call) RunAndReturn(run func(*tcontainer.RunOptions) error) *RunOption_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewRunOption creates a new instance of RunOption. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRunOption(t interface {
	mock.TestingT
	Cleanup(func())
}) *RunOption {
	mock := &RunOption{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
