package mocks

import "github.com/stretchr/testify/mock"

// MockCentralisedSubscriber is a mock implementation of CentralisedSubscriber.
type MockCentralisedSubscriber struct {
	mock.Mock
}

func (m *MockCentralisedSubscriber) Subscribe(clientID string, channelName string) {
	m.Called(clientID, channelName)
}

func (m *MockCentralisedSubscriber) UnSubscribe(clientID string, channelName string) {
	m.Called(clientID, channelName)
}

func (m *MockCentralisedSubscriber) UnsubscribeAll(clientID string) {
	m.Called(clientID)
}
