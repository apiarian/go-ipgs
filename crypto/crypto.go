// The crypto package implements the ECDSA P256 SHA256 algorithm. Some day we
// might be brave enough to implement EdDSA Curve25519 signatures
// ( https://tools.ietf.org/html/draft-josefsson-eddsa-ed25519-02 ) but not
// today. The ECDSA implementation is largely based on the
// github.com/gtank/cryptopasta code
package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/pkg/errors"
)

// NewSigningKey creates a random ECDSA P56 private key (which includes a public key)
func NewSigningKey() (*ecdsa.PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create key")
	}

	return k, nil
}

// Sign signs some arbitrary data with an ECDSA private keys, such as one
// created by NewSigningKey() . The signature can later be checked with the
// Verify function.
func Sign(d []byte, k *ecdsa.PrivateKey) ([]byte, error) {
	h := sha256.Sum256(d)

	r, s, err := ecdsa.Sign(rand.Reader, k, h[:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign the digest")
	}

	size := k.Curve.Params().P.BitLen() / 8
	rb, sb := r.Bytes(), s.Bytes()
	sig := make([]byte, size*2)
	// this copy format is used to take care of leading zeros
	copy(sig[size-len(rb):], rb)
	copy(sig[size*2-len(sb):], sb)

	return sig, nil
}

// Verify checks the arbitrary data and signature against the public key. This
// is a reciprocal of the Sign function.
func Verify(d, sig []byte, k *ecdsa.PublicKey) bool {
	h := sha256.Sum256(d)

	size := k.Curve.Params().P.BitLen() / 8

	r, s := new(big.Int), new(big.Int)

	r.SetBytes(sig[:size])
	s.SetBytes(sig[size:])

	return ecdsa.Verify(k, h[:], r, s)
}
