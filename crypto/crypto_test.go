package crypto

import (
	"bytes"
	"crypto/elliptic"
	"testing"
)

func fatalIfErr(t *testing.T, msg string, err error) {
	if err != nil {
		t.Fatalf("%s: %+v\n", msg, err)
	}
}

func TestSignAndVerify(t *testing.T) {
	// since we are using random keys, lets run this test a bunch of times to
	// make sure that things are generally good
	for n := 0; n < 1000; n++ {
		d := []byte("test 123")

		k, err := NewPrivateKey()
		fatalIfErr(t, "failed to create new key", err)

		s, err := Sign(d, k)
		fatalIfErr(t, "failed to sign data", err)

		if !Verify(d, s, k.GetPublicKey()) {
			t.Fatal("signature was not verified")
		}

		dPrime := make([]byte, len(d))
		copy(dPrime, d)
		dPrime[0] ^= 0xff

		if Verify(dPrime, s, k.GetPublicKey()) {
			t.Fatal("altered data was verified")
		}

		sPrime := make([]byte, len(s))
		copy(sPrime, s)
		sPrime[0] ^= 0xff

		if Verify(d, sPrime, k.GetPublicKey()) {
			t.Fatal("altered signature was verified")
		}

		var i int
		var kPrime *PrivateKey
		for {
			kPrime, err = NewPrivateKey()
			fatalIfErr(t, "failed to create another key", err)

			pk1 := k.PublicKey
			pk2 := kPrime.PublicKey

			if (pk1.X.Cmp(pk2.X) != 0) || (pk1.Y.Cmp(pk2.Y) != 0) {
				break
			}

			i++
			if i > 100 {
				t.Fatalf("tried 100 new private keys and they were all the same as the original")
			}
		}

		if Verify(d, s, kPrime.GetPublicKey()) {
			t.Fatal("a different public key verified the data and signature")
		}
	}
}

type Keylike interface {
	Params() *elliptic.CurveParams
}

func checkKeyEquality(t *testing.T, k1, k2 Keylike) {
	priv1, ok1 := k1.(*PrivateKey)
	priv2, ok2 := k2.(*PrivateKey)

	if ok1 && ok2 {
		t.Log("detected two private keys")

		if priv1.Curve != priv2.Curve {
			t.Fatal("the two private keys have different curves")
		}

		if priv1.X.Cmp(priv2.X) != 0 {
			t.Fatal("the two private keys have different X values")
		}

		if priv1.Y.Cmp(priv2.Y) != 0 {
			t.Fatal("the two private keys have different Y values")
		}

		if priv1.D.Cmp(priv2.D) != 0 {
			t.Fatal("the two private keys have different D values")
		}

		if priv1.Name != priv2.Name {
			t.Fatal("the two private keys have different names")
		}

		if priv1.Comment != priv2.Comment {
			t.Fatal("the two private keys have different comments")
		}

		return
	}

	pub1, ok1 := k1.(*PublicKey)
	pub2, ok2 := k2.(*PublicKey)

	if ok1 && ok2 {
		t.Log("detected at two public keys")

		if pub1.Curve != pub2.Curve {
			t.Fatal("the two public keys have different curves")
		}

		if pub1.X.Cmp(pub2.X) != 0 {
			t.Fatal("the two public keys have different X values")
		}

		if pub1.Y.Cmp(pub2.Y) != 0 {
			t.Fatal("the two public keys have different Y values")
		}

		if pub1.Name != pub2.Name {
			t.Fatal("the two public keys have different names")
		}

		if pub1.Comment != pub2.Comment {
			t.Fatal("the two public keys have different comments")
		}

		return
	}

	t.Fatal("did not detect a pair of matched private or public keys")
}

func TestReadWrite(t *testing.T) {
	// since we are using random keys, lets run this test a bunch of times to
	// make sure that things are generally good
	for n := 0; n < 1000; n++ {
		k, err := NewPrivateKey()
		fatalIfErr(t, "failed to create new key", err)

		k.Name = "tester"
		k.Comment = "hello world"

		var b bytes.Buffer
		err = k.Write(&b)
		fatalIfErr(t, "failed to write bytes", err)

		t.Logf("written private key:\n%s", b.String())

		kPrime, err := ReadPrivateKey(&b)
		fatalIfErr(t, "failed to read key", err)

		checkKeyEquality(t, k, kPrime)

		var bPub bytes.Buffer
		err = k.GetPublicKey().Write(&bPub)
		fatalIfErr(t, "failed to write bytes", err)

		t.Logf("written public key:\n%s", bPub.String())

		kPub, err := ReadPublicKey(&bPub)
		fatalIfErr(t, "failed to read public key", err)

		checkKeyEquality(t, k.GetPublicKey(), kPub)

		if kPub.Name != k.Name {
			t.Fatal("name did not carry over from PrivateKey to PublicKey")
		}

		if kPub.Comment != k.Comment {
			t.Fatal("Comment did not carry over from PrivateKey to PublicKey")
		}

		b.Reset()
		err = k.Write(&b)
		fatalIfErr(t, "failed to write private key again", err)

		kPub2, err := ReadPublicKey(&b)
		fatalIfErr(t, "failed to read public key from private/public buffer", err)

		checkKeyEquality(t, k.GetPublicKey(), kPub2)

		bPub.Reset()
		err = k.GetPublicKey().Write(&bPub)
		fatalIfErr(t, "failed to write public key again", err)

		kNone, err := ReadPrivateKey(&bPub)
		if err == nil {
			t.Fatal("was able to read a private key from a public-only buffer")
		}

		if kNone != nil {
			t.Fatal("a key was returned when none should have been found")
		}

		if err.Error() != "did not find a private key in the input" {
			t.Fatalf("wrong error returned: %+v\n", err)
		}
	}
}
