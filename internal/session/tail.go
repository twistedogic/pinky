package session

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// tailJSONL reads JSONL lines from path starting at *offset,
// calls parse on each line, and advances *offset to the new end
// of file. Returns the collected messages. Shared by piSource and
// codexSource — they differ only in how each line decodes to
// []Message, so the open / seek / scan / advance skeleton lives
// here. The scanner cap mirrors scannerMaxLine (16 MiB).
func tailJSONL(path string, offset *int64, parse func(line []byte) []Message) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(*offset, io.SeekStart); err != nil {
		return nil, err
	}
	var out []Message
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), scannerMaxLine)
	for sc.Scan() {
		out = append(out, parse(sc.Bytes())...)
	}
	end, _ := f.Seek(0, io.SeekCurrent)
	*offset = end
	return out, sc.Err()
}

// extractJSON unmarshals a JSONL line into the given target,
// returning the raw bytes for further inspection. Returns
// (false) on unmarshal failure so the caller can skip the line
// silently.
func extractJSON[T any](line []byte, out *T) bool {
	return json.Unmarshal(line, out) == nil
}
