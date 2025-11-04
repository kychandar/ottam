package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockWebSocketBridge struct {
	mock.Mock
}

func (m *MockWebSocketBridge) ProcessMessagesFromServer(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockWebSocketBridge) ProcessMessagesFromClient(ctx context.Context) {
	m.Called(ctx)
}
