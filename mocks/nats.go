package mocks

import (
	"github.com/stretchr/testify/mock"
)

type MockPubSubProvider struct {
	mock.Mock
}

func (m *MockPubSubProvider) CreateStream(streamName string, subjects []string) error {
	args := m.Called(streamName, subjects)
	return args.Error(0)
}

func (m *MockPubSubProvider) Publish(subjectName string, data []byte) error {
	args := m.Called(subjectName, data)
	return args.Error(0)
}

func (m *MockPubSubProvider) Subscribe(consumerName string, subjectName string, callBack func(msg []byte) bool) error {
	args := m.Called(consumerName, subjectName, callBack)
	return args.Error(0)
}

func (m *MockPubSubProvider) UnSubscribe(consumerName string) error {
	args := m.Called(consumerName)
	return args.Error(0)
}

func (m *MockPubSubProvider) Close() error {
	args := m.Called()
	return args.Error(0)
}
