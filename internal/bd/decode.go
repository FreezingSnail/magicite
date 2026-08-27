package bd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrMalformedOutput reports stdout that is not the expected bd JSON shape.
var ErrMalformedOutput = errors.New("bd: malformed output")

// DecodeBeads decodes the JSON array emitted by bd issue commands.
func DecodeBeads(stdout []byte) ([]Bead, error) {
	var beads []Bead
	if err := decodeArray(stdout, &beads); err != nil {
		return nil, err
	}
	return beads, nil
}

// DecodeDeps decodes the JSON array emitted by bd dependency commands.
func DecodeDeps(stdout []byte) ([]Dependency, error) {
	var dependencies []Dependency
	if err := decodeArray(stdout, &dependencies); err != nil {
		return nil, err
	}
	return dependencies, nil
}

// DecodeIDs returns non-empty IDs from a bd issue JSON array.
func DecodeIDs(stdout []byte) ([]string, error) {
	beads, err := DecodeBeads(stdout)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(beads))
	for _, bead := range beads {
		if bead.ID != "" {
			ids = append(ids, bead.ID)
		}
	}
	return ids, nil
}

// DecodeLabels decodes either label-array shape emitted by bd.
func DecodeLabels(stdout []byte) ([]string, error) {
	var values []json.RawMessage
	if err := decodeArray(stdout, &values); err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(string(value))
		if len(trimmed) == 0 {
			return nil, malformedOutput(stdout)
		}

		var label string
		switch trimmed[0] {
		case '"':
			if err := json.Unmarshal(value, &label); err != nil {
				return nil, malformedOutput(stdout)
			}
		case '{':
			var record struct {
				Label string `json:"label"`
				Name  string `json:"name"`
			}
			if err := json.Unmarshal(value, &record); err != nil {
				return nil, malformedOutput(stdout)
			}
			label = record.Label
			if strings.TrimSpace(label) == "" {
				label = record.Name
			}
		default:
			return nil, malformedOutput(stdout)
		}

		if strings.TrimSpace(label) != "" {
			labels = append(labels, label)
		}
	}
	return labels, nil
}

// DecodeEnvelope recognizes bd's non-empty JSON error envelope.
func DecodeEnvelope(stdout []byte) (Envelope, bool) {
	if len(strings.TrimSpace(string(stdout))) == 0 || firstNonSpace(stdout) != '{' {
		return Envelope{}, false
	}

	var envelope Envelope
	if err := json.Unmarshal(stdout, &envelope); err != nil || envelope.Error == "" {
		return Envelope{}, false
	}
	return envelope, true
}

func decodeArray[T any](stdout []byte, values *[]T) error {
	if len(strings.TrimSpace(string(stdout))) == 0 || firstNonSpace(stdout) != '[' {
		return malformedOutput(stdout)
	}
	if err := json.Unmarshal(stdout, values); err != nil || *values == nil {
		return malformedOutput(stdout)
	}
	return nil
}

func firstNonSpace(value []byte) byte {
	for _, b := range value {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}
	return 0
}

func malformedOutput(stdout []byte) error {
	const limit = 200
	if len(stdout) > limit {
		stdout = stdout[:limit]
	}
	return fmt.Errorf("%w: %q", ErrMalformedOutput, stdout)
}
