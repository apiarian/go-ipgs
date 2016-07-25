package state

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

// IPGSTime wraps around time.Time for consistent text formatting
type IPGSTime struct {
	time.Time
}

// MarshalJSON creates a UTC RFC-3339 representation of the time.Time embedded
// within IPGSTime
func (t IPGSTime) MarshalJSON() ([]byte, error) {
	s := t.UTC().Format(time.RFC3339Nano)

	b, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal time string")
	}

	return b, nil
}
