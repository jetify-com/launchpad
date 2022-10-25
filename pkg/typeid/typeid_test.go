package typeid

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestType(t *testing.T) {
	tid := New("prefix")
	assert.Equal(t, "prefix", tid.Type())
}

func TestString_ExpectedFormat(t *testing.T) {
	tid := New("prefix")
	assert.Regexp(t, "^prefix_[a-zA-_Z0-9]{24}$", tid.String())
}

func TestEncodeDecode_Random(t *testing.T) {
	for i := 0; i < 1000; i++ {
		tid := New("prefix")
		// Check that decoding the slug matches the first 4 bytes
		s, err := decodeSlug(tid.Slug())
		assert.NoError(t, err)
		assert.Equal(t, s, tid.value[:4])

		// Check that decoding the suffix matches the last 12 bytes
		b := decodeSuffix(tid.suffix())
		assert.Equal(t, b, tid.value[4:])
	}
}
