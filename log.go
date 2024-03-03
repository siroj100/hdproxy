package main

import (
	"io"
)

type (
	PrefixedWriter struct {
		writer io.Writer
		prefix string
	}
)

func NewPrefixedWriter(writer io.Writer, prefix string) PrefixedWriter {
	return PrefixedWriter{writer: writer, prefix: prefix}
}

func (w PrefixedWriter) Write(p []byte) (int, error) {
	_, err := w.writer.Write([]byte(w.prefix + ": "))
	if err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}
