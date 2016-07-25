package state

import (
	"encoding/json"
	"testing"

	"github.com/apiarian/go-ipgs/crypto"
)

func TestCryptoJSON(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key", err)

	k := &PublicKey{priv.GetPublicKey(), "some-hash"}

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

	if l.Hash == k.Hash {
		t.Fatalf("the hash was inexplicably restored from the JSON. magic?")
	}
}

func TestCryptoEquality(t *testing.T) {
	priv1, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key 1", err)

	priv2, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key 2", err)

	k1 := &PublicKey{priv1.GetPublicKey(), "a"}

	k1_prime := &PublicKey{priv1.GetPublicKey(), "x"}

	k2 := &PublicKey{priv2.GetPublicKey(), "b"}

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
