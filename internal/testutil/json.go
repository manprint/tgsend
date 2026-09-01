package testutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodeOneJSON decodes exactly one JSON document and requires its raw input
// to end in the newline emitted by the CLI. The caller retains data unchanged
// for byte-level assertions.
func DecodeOneJSON(data []byte, target any) error {
	if !bytes.HasSuffix(data, []byte("\n")) {
		return errors.New("JSON document must end with a newline")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON document: %w", err)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON input contains more than one document")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}
