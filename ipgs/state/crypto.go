package state

import (
	"bytes"
	"encoding/json"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/pkg/errors"
)

type PublicKey struct {
	*crypto.PublicKey
	Hash string
}

type PrivateKey struct {
	*crypto.PrivateKey
}

func (k *PublicKey) MarshalJSON() ([]byte, error) {
	b := bytes.NewBuffer(nil)

	err := k.Write(b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write key to buffer")
	}

	s := b.String()

	o, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal string")
	}

	return o, nil
}

func (k *PublicKey) UnmarshalJSON(d []byte) error {
	var s string
	err := json.Unmarshal(d, &s)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal string")
	}

	b := bytes.NewBufferString(s)
	pk, err := crypto.ReadPublicKey(b)
	if err != nil {
		return errors.Wrap(err, "failed to read key")
	}

	k.PublicKey = pk

	return nil
}

func (k *PublicKey) Publish(s *cachedshell.Shell) (string, error) {
	if k.Hash != "" {
		return k.Hash, nil
	}

	b := bytes.NewBuffer(nil)

	err := k.Write(b)
	if err != nil {
		return "", errors.Wrap(err, "failed to write key to buffer")
	}

	h, err := s.Add(b)
	if err != nil {
		return "", errors.Wrap(err, "failed to add key buffer")
	}

	k.Hash = h

	return h, nil
}

func (k *PublicKey) Get(h string, s *cachedshell.Shell) error {
	d, err := s.Cat(h)
	if err != nil {
		return errors.Wrap(err, "failed to get key buffer")
	}
	defer d.Close()

	pk, err := crypto.ReadPublicKey(d)
	if err != nil {
		return errors.Wrap(err, "failed to read key")
	}

	k.PublicKey = pk
	k.Hash = h

	return nil
}

func (k *PublicKey) Equals(o *PublicKey) bool {
	return (k.Curve == o.Curve && k.X.Cmp(o.X) == 0 && k.Y.Cmp(o.Y) == 0)
}
