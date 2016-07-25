package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeMarshal(t *testing.T) {
	n := time.Now().Local()
	t.Logf("original: %s\n", n)

	it := IPGSTime{n}

	j, err := json.Marshal(it)
	fatalIfErr(t, "marshal IPGSTime", err)

	t.Logf("marshaled: %s\n", string(j))

	var l time.Time
	err = json.Unmarshal(j, &l)
	fatalIfErr(t, "unmarshal Time", err)

	t.Logf("unmarshaled: %s\n", l)

	if !l.Equal(n) {
		t.Fatalf("the original and loaded times are not equal")
	}

	if l.Location() != time.UTC {
		t.Fatalf("the loaded time is not in UTC format")
	}
}
