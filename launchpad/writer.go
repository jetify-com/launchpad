package launchpad

import (
	"io"
	"os"
)

type dockerWriter struct {
}

func (writer dockerWriter) Write(data []byte) (int, error) {
	tab := []byte("\t")
	data = append(tab, data...)
	n, err := io.Writer.Write(os.Stderr, data)
	return n, err
}
