// Code generated by mockery. DO NOT EDIT.

package tcontainer_mocks

import (
	docker "github.com/ory/dockertest/v3/docker"
	mock "github.com/stretchr/testify/mock"

	tcontainer "github.com/kiteggrad/tcontainer"
)

// ContainerConfigCheck is an autogenerated mock type for the ContainerConfigCheck type
type ContainerConfigCheck struct {
	mock.Mock
}

type ContainerConfigCheck_Expecter struct {
	mock *mock.Mock
}

func (_m *ContainerConfigCheck) EXPECT() *ContainerConfigCheck_Expecter {
	return &ContainerConfigCheck_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: container, expectedOptions
func (_m *ContainerConfigCheck) Execute(container *docker.Container, expectedOptions tcontainer.RunOptions) error {
	ret := _m.Called(container, expectedOptions)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*docker.Container, tcontainer.RunOptions) error); ok {
		r0 = rf(container, expectedOptions)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ContainerConfigCheck_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type ContainerConfigCheck_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - container *docker.Container
//   - expectedOptions tcontainer.RunOptions
func (_e *ContainerConfigCheck_Expecter) Execute(container interface{}, expectedOptions interface{}) *ContainerConfigCheck_Execute_Call {
	return &ContainerConfigCheck_Execute_Call{Call: _e.mock.On("Execute", container, expectedOptions)}
}

func (_c *ContainerConfigCheck_Execute_Call) Run(run func(container *docker.Container, expectedOptions tcontainer.RunOptions)) *ContainerConfigCheck_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*docker.Container), args[1].(tcontainer.RunOptions))
	})
	return _c
}

func (_c *ContainerConfigCheck_Execute_Call) Return(err error) *ContainerConfigCheck_Execute_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *ContainerConfigCheck_Execute_Call) RunAndReturn(run func(*docker.Container, tcontainer.RunOptions) error) *ContainerConfigCheck_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewContainerConfigCheck creates a new instance of ContainerConfigCheck. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewContainerConfigCheck(t interface {
	mock.TestingT
	Cleanup(func())
}) *ContainerConfigCheck {
	mock := &ContainerConfigCheck{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}