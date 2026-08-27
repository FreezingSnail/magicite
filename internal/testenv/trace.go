package testenv

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Entry is one recorded child-process invocation.
type Entry struct {
	Seq  int
	Dir  string
	Argv []string
}

// Record appends argv and dir to tracePath as one locked trace line.
func Record(tracePath string, argv []string, dir string) error {
	file, err := os.OpenFile(tracePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open trace: %w", err)
	}
	defer file.Close()
	if err := lock(file); err != nil {
		return err
	}
	defer unlock(file)

	contents, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read trace sequence: %w", err)
	}
	entries, err := decode(contents)
	if err != nil {
		return err
	}
	seq := 1
	if len(entries) > 0 {
		seq = entries[len(entries)-1].Seq + 1
	}
	line := encode(seq, dir, argv)
	written, err := file.Write(line)
	if err != nil {
		return fmt.Errorf("append trace: %w", err)
	}
	if written != len(line) {
		return io.ErrShortWrite
	}
	return nil
}

// Read returns trace entries in append order. A missing trace is empty.
func Read(tracePath string) ([]Entry, error) {
	contents, err := os.ReadFile(tracePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read trace: %w", err)
	}
	return decode(contents)
}

// Reset removes all entries from tracePath.
func Reset(tracePath string) error {
	file, err := os.OpenFile(tracePath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open trace: %w", err)
	}
	defer file.Close()
	if err := lock(file); err != nil {
		return err
	}
	defer unlock(file)
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate trace: %w", err)
	}
	return nil
}

func lock(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock trace: %w", err)
	}
	return nil
}

func unlock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func encode(seq int, dir string, argv []string) []byte {
	fields := make([]string, 0, len(argv)+2)
	fields = append(fields, strconv.Itoa(seq), strconv.Quote(dir))
	for _, arg := range argv {
		fields = append(fields, strconv.Quote(arg))
	}
	return append([]byte(strings.Join(fields, "\t")), '\n')
}

func decode(contents []byte) ([]Entry, error) {
	var entries []Entry
	for len(contents) > 0 {
		end := bytes.IndexByte(contents, '\n')
		if end == -1 {
			break // A terminated writer can leave a partial final line.
		}
		line := contents[:end]
		contents = contents[end+1:]
		if len(line) == 0 {
			return nil, fmt.Errorf("invalid empty trace line")
		}
		entry, err := decodeLine(string(line))
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeLine(line string) (Entry, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		return Entry{}, fmt.Errorf("invalid trace line %q", line)
	}
	seq, err := strconv.Atoi(fields[0])
	if err != nil || seq < 1 {
		return Entry{}, fmt.Errorf("invalid trace sequence %q", fields[0])
	}
	values := make([]string, len(fields)-1)
	for i, field := range fields[1:] {
		value, err := strconv.Unquote(field)
		if err != nil {
			return Entry{}, fmt.Errorf("invalid escaped trace field %q: %w", field, err)
		}
		values[i] = value
	}
	return Entry{Seq: seq, Dir: values[0], Argv: values[1:]}, nil
}
