// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// ChatOpsCom is an autogenerated mock type for the ChatOpsCom type
type ChatOpsCom struct {
	mock.Mock
}

// EnvironmentParams provides a mock function with given fields:
func (_m *ChatOpsCom) EnvironmentParams() map[string]string {
	ret := _m.Called()

	var r0 map[string]string
	if rf, ok := ret.Get(0).(func() map[string]string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	return r0
}

// KafkaProduce provides a mock function with given fields: topic, message
func (_m *ChatOpsCom) KafkaProduce(topic string, message string) {
	_m.Called(topic, message)
}
