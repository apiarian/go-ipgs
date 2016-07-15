package crypto

import (
	"crypto/ecdsa"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	// since we are using random keys, lets run this test a bunch of times to
	// make sure that things are generally good
	for n := 0; n < 1000; n++ {
		d := []byte("test 123")

		k, err := NewSigningKey()
		if err != nil {
			t.Fatalf("failed to create new key: %+v\n", err)
		}

		s, err := Sign(d, k)
		if err != nil {
			t.Fatalf("failed to sign data: %+v\n", err)
		}

		if !Verify(d, s, &k.PublicKey) {
			t.Fatal("signature was not verified")
		}

		dPrime := make([]byte, len(d))
		copy(dPrime, d)
		dPrime[0] ^= 0xff

		if Verify(dPrime, s, &k.PublicKey) {
			t.Fatal("altered data was verified")
		}

		sPrime := make([]byte, len(s))
		copy(sPrime, s)
		sPrime[0] ^= 0xff

		if Verify(d, sPrime, &k.PublicKey) {
			t.Fatal("altered signature was verified")
		}

		var i int
		var kPrime *ecdsa.PrivateKey
		for {
			kPrime, err = NewSigningKey()
			if err != nil {
				t.Fatalf("failed to create another key: %+v\n", err)
			}

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

		if Verify(d, s, &kPrime.PublicKey) {
			t.Fatal("a different public key verified the data and signature")
		}
	}
}
