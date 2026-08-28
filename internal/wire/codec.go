package wire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrSchema = errors.New("wire: schema mismatch")

type Frame struct {
	Response *Response
	Event    *Event
}

type Encoder struct {
	writer *bufio.Writer
	flush  func() error
}

func NewEncoder(w io.Writer) *Encoder {
	encoder := &Encoder{writer: bufio.NewWriter(w)}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		encoder.flush = flusher.Flush
	}
	return encoder
}

func (e *Encoder) Encode(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte{'\n'}) {
		return errors.New("wire: encoded frame contains newline")
	}
	if _, err := e.writer.Write(data); err != nil {
		return err
	}
	if err := e.writer.WriteByte('\n'); err != nil {
		return err
	}
	if err := e.writer.Flush(); err != nil {
		return err
	}
	if e.flush != nil {
		return e.flush()
	}
	return nil
}

type Decoder struct {
	reader *bufio.Reader
	line   int
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{reader: bufio.NewReader(r)}
}

func (d *Decoder) Request() (Request, error) {
	line, err := d.readLine()
	if err != nil {
		return Request{}, err
	}

	var header struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(line, &header); err != nil {
		return Request{}, d.lineError(err)
	}
	if header.Schema != Schema {
		return Request{}, ErrSchema
	}

	var request Request
	if err := json.Unmarshal(line, &request); err != nil {
		return Request{}, d.lineError(err)
	}
	return request, nil
}

func (d *Decoder) Frame() (Frame, error) {
	line, err := d.readLine()
	if err != nil {
		return Frame{}, err
	}

	var header struct {
		Schema int             `json:"schema"`
		ID     json.RawMessage `json:"id"`
		Seq    json.RawMessage `json:"seq"`
	}
	if err := json.Unmarshal(line, &header); err != nil {
		return Frame{}, d.lineError(err)
	}
	if header.Schema != Schema {
		return Frame{}, ErrSchema
	}

	if header.ID != nil {
		var response Response
		if err := json.Unmarshal(line, &response); err != nil {
			return Frame{}, d.lineError(err)
		}
		return Frame{Response: &response}, nil
	}
	if header.Seq != nil {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return Frame{}, d.lineError(err)
		}
		return Frame{Event: &event}, nil
	}
	return Frame{}, d.lineError(errors.New("frame has neither id nor seq"))
}

func (d *Decoder) readLine() ([]byte, error) {
	line, err := d.reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, d.lineError(err)
	}
	if len(line) == 0 {
		return nil, io.EOF
	}
	d.line++
	return bytes.TrimSuffix(line, []byte{'\n'}), nil
}

func (d *Decoder) lineError(err error) error {
	return fmt.Errorf("wire: line %d: %w", d.line, err)
}
