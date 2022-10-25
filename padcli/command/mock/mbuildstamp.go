package mock

import (
	"github.com/stretchr/testify/mock"
)

type MockBuildStamp struct {
	mock.Mock

	VersionFunc              func() string
	IsCicdReleasedBinaryFunc func() bool
	IsDevBinaryFunc          func() bool
}

func (b *MockBuildStamp) Version() string {
	if b.VersionFunc != nil {
		return b.VersionFunc()
	}
	return "0.1.0"
}

func (b *MockBuildStamp) IsCicdReleasedBinary() bool {
	if b.IsCicdReleasedBinaryFunc != nil {
		return b.IsCicdReleasedBinaryFunc()
	}
	return true
}

func (b *MockBuildStamp) IsDevBinary() bool {
	if b.IsDevBinaryFunc != nil {
		return b.IsDevBinaryFunc()
	}
	return false
}
