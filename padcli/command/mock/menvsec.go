package mock

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.jetpack.io/envsec"
)

func NewEnvsecStore() envsec.Store {
	return &MockEnvsecStore{}
}

// MockEnvsecStore implements envsec.Store interface (compile-time check)
var _ envsec.Store = (*MockEnvsecStore)(nil)

type MockEnvsecStore struct {
	mock.Mock
}

func (s *MockEnvsecStore) List(ctx context.Context, envId envsec.EnvID) ([]envsec.EnvVar, error) {
	return nil, nil
}

func (s *MockEnvsecStore) Set(ctx context.Context, envId envsec.EnvID, name string, value string) error {
	return nil
}

func (s *MockEnvsecStore) SetAll(ctx context.Context, envId envsec.EnvID, values map[string]string) error {
	return nil
}

func (s *MockEnvsecStore) Get(ctx context.Context, envId envsec.EnvID, name string) (string, error) {
	return "", nil
}

func (s *MockEnvsecStore) GetAll(ctx context.Context, envId envsec.EnvID, names []string) ([]envsec.EnvVar, error) {
	return nil, nil
}

func (s *MockEnvsecStore) Delete(ctx context.Context, envId envsec.EnvID, name string) error {
	return nil
}

func (s *MockEnvsecStore) DeleteAll(ctx context.Context, envId envsec.EnvID, names []string) error {
	return nil
}
