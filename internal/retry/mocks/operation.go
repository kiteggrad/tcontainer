// Code generated by mockery. DO NOT EDIT.

package retry_mocks

import mock "github.com/stretchr/testify/mock"

// Operation is an autogenerated mock type for the operation type
type Operation struct {
	mock.Mock
}

type Operation_Expecter struct {
	mock *mock.Mock
}

func (_m *Operation) EXPECT() *Operation_Expecter {
	return &Operation_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields:
func (_m *Operation) Execute() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Operation_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type Operation_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
func (_e *Operation_Expecter) Execute() *Operation_Execute_Call {
	return &Operation_Execute_Call{Call: _e.mock.On("Execute")}
}

func (_c *Operation_Execute_Call) Run(run func()) *Operation_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Operation_Execute_Call) Return(_a0 error) *Operation_Execute_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Operation_Execute_Call) RunAndReturn(run func() error) *Operation_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewOperation creates a new instance of Operation. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewOperation(t interface {
	mock.TestingT
	Cleanup(func())
}) *Operation {
	mock := &Operation{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
