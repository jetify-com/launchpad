package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirSha1(t *testing.T) {
	t.Run("TestDirSha1", func(t *testing.T) {

		// Test that directories with identical components have same checksum
		checksum1 := createFilesAndGetChecksum(t, "", "")
		checksum2 := createFilesAndGetChecksum(t, "", "")
		assert.Equal(t, checksum1, checksum2)

		// Test that directories with different components have different checksum
		checksum3 := createFilesAndGetChecksum(t, "other", "")
		if checksum1 == checksum3 {
			t.Errorf(
				"checksums were expected to be different, but they "+
					"match. checksum1 = %v checksum3 = %v",
				checksum1,
				checksum3,
			)
		}

		expected := "1c6d075dde80b9cb8992776eea61d3bdf4b820a7"
		// Test that the checksum is stable over time
		if checksum1 != expected {
			t.Errorf(
				"got unexpected hash %v but expected %v",
				checksum1,
				expected,
			)
		}

		// Test that directories with identical components but different seeds
		// have different checksum
		checksum4 := createFilesAndGetChecksum(t, "", "foo")
		checksum5 := createFilesAndGetChecksum(t, "", "bar")
		assert.NotEqual(t, checksum4, checksum5)
	})
}

func createFilesAndGetChecksum(t *testing.T, data string, seed string) string {
	tmpDir1 := t.TempDir()
	defer func() {
		if err := os.RemoveAll(tmpDir1); err != nil {
			t.Errorf("got error %v", err)
		}
	}()

	// Hardcode the name, instead of making another ioutil.TempDir,
	// because our checksum logic uses the directory's relative-path.
	tmpDir2Name := "tmpDir2"
	tmpDir2 := filepath.Join(tmpDir1, tmpDir2Name)
	err := os.Mkdir(tmpDir2, 0700)
	if err != nil {
		t.Errorf("got error %v", err)
	}

	f1, err := os.CreateTemp(tmpDir1, "1")
	if err != nil {
		t.Errorf("got error %v", err)
	}
	_, err = f1.WriteString("foobar1" + data)
	if err != nil {
		t.Errorf("got error %v", err)
	}
	defer func() {
		if err = f1.Close(); err != nil {
			t.Errorf("got error %v", err)
		}
	}()

	f2, err := os.CreateTemp(tmpDir2, "2")
	if err != nil {
		t.Errorf("got error %v", err)
	}
	_, err = f2.WriteString("foobar2" + data)
	if err != nil {
		t.Errorf("got error %v", err)
	}
	defer func() {
		if err = f2.Close(); err != nil {
			t.Errorf("got error %v", err)
		}
	}()

	checksum, err := DirSha1(tmpDir1, seed)
	if err != nil {
		t.Errorf("got error %v", err)
	}

	return checksum
}
