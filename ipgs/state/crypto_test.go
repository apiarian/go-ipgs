package state

import (
	"encoding/json"
	"testing"

	"github.com/apiarian/go-ipgs/crypto"
)

func TestCryptoJSON(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key", err)

	k := NewPublicKey(priv.GetPublicKey(), "some-hash")

	t.Logf("key: %+v\n", k)

	j, err := json.Marshal(k)
	fatalIfErr(t, "marshal key", err)

	t.Logf("marshaled: %s\n", string(j))

	var l *PublicKey
	err = json.Unmarshal(j, &l)
	fatalIfErr(t, "unmarshal key", err)

	t.Logf("unmarshaled: %+v\n", l)

	if !l.Equals(k) {
		t.Fatal("the new key is not the same as the old key")
	}

	if l.Hash() == k.Hash() {
		t.Fatalf("the hash was inexplicably restored from the JSON. magic?")
	}
}

func TestCryptoEquality(t *testing.T) {
	priv1, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key 1", err)

	priv2, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key 2", err)

	k1 := NewPublicKey(priv1.GetPublicKey(), "a")

	k1_prime := NewPublicKey(priv1.GetPublicKey(), "x")

	k2 := NewPublicKey(priv2.GetPublicKey(), "b")

	if k1.Equals(k2) {
		t.Fatalf("k1 and k2 are strangely equal")
	}

	if !k1.Equals(k1_prime) {
		t.Fatalf("k1 and k1_prime are not equal")
	}

	if k2.Equals(k1) {
		t.Fatalf("k2 and k1 should not be equal either")
	}

	if k2.Equals(k1_prime) {
		t.Fatalf("k2 and k1_prime should not be equal either either")
	}
}

func TestCryptoPublishGet(t *testing.T) {
	s, err := newShellForNode(0)
	fatalIfErr(t, "failed to get shell for node 0", err)

	sPrime, err := newShellForNode(1)
	fatalIfErr(t, "failed to get shell for node 1", err)

	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	k := NewPublicKey(priv.GetPublicKey(), "")

	t.Logf("key: %+v\n", k)

	h, err := k.Publish(s)
	fatalIfErr(t, "failed to publish key", err)

	t.Log("hash:", h)

	if h == "" {
		t.Fatal("the hash should not be an empty string")
	}

	if k.Hash() != h {
		t.Fatal("key hash has not been set correctly")
	}

	obj, err := s.ObjectGet(h)
	fatalIfErr(t, "failed to get key object", err)

	t.Logf("key object: %+v\n", obj)

	l := NewPublicKey(nil, "")
	err = l.Get(h, sPrime)
	fatalIfErr(t, "failed to get key from a different node", err)

	t.Logf("loaded: %+v\n", l)

	if l.Hash() != k.Hash() {
		t.Fatal("the hash of the loaded key is not the same as the original")
	}

	if !k.Equals(l) {
		t.Fatal("the loaded key does not match the original")
	}

	h2, err := l.Publish(sPrime)
	fatalIfErr(t, "failed to republish the loaded key", err)

	if h2 != h {
		t.Fatal("the hash of the republished key is not the same as the original")
	}
}
