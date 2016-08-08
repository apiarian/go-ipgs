package state

import (
	"bytes"
	"encoding/json"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/pkg/errors"
)

type PrivateKey struct {
	key *crypto.PrivateKey
}

func NewPrivateKey(k *crypto.PrivateKey) *PrivateKey {
	return &PrivateKey{
		key: k,
	}
}

func (k *PrivateKey) Key() *crypto.PrivateKey {
	return k.key
}

type PublicKey struct {
	key  *crypto.PublicKey
	hash string
}

func NewPublicKey(k *crypto.PublicKey, h string) *PublicKey {
	return &PublicKey{
		key:  k,
		hash: h,
	}
}

func (k *PublicKey) Key() *crypto.PublicKey {
	return k.key
}

func (k *PublicKey) Hash() string {
	return k.hash
}

type jsonPublicKey struct {
	PEM  string
	Hash string
}

func (k *PublicKey) MarshalJSON() ([]byte, error) {
	b := bytes.NewBuffer(nil)

	err := k.key.Write(b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write key to buffer")
	}

	s := b.String()

	jPk := jsonPublicKey{
		PEM:  s,
		Hash: k.hash,
	}

	o, err := json.Marshal(jPk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal JSON Public Key")
	}

	return o, nil
}

func (k *PublicKey) UnmarshalJSON(d []byte) error {
	var jPk jsonPublicKey

	err := json.Unmarshal(d, &jPk)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON Public Key")
	}

	b := bytes.NewBufferString(jPk.PEM)
	pk, err := crypto.ReadPublicKey(b)
	if err != nil {
		return errors.Wrap(err, "failed to read key")
	}

	k.key = pk
	k.hash = jPk.Hash

	return nil
}

func (k *PublicKey) Publish(s *cachedshell.Shell) (string, error) {
	if k.hash != "" {
		return k.hash, nil
	}

	b := bytes.NewBuffer(nil)

	err := k.key.Write(b)
	if err != nil {
		return "", errors.Wrap(err, "failed to write key to buffer")
	}

	h, err := s.Add(b)
	if err != nil {
		return "", errors.Wrap(err, "failed to add key buffer")
	}

	k.hash = h

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

	k.key = pk
	k.hash = h

	return nil
}

func (k *PublicKey) Equals(o *PublicKey) bool {
	k1 := k.key
	k2 := o.key
	return (k1.Curve == k2.Curve && k1.X.Cmp(k2.X) == 0 && k1.Y.Cmp(k2.Y) == 0)
}
