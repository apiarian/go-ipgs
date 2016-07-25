// Package crypto implements the ECDSA P256 SHA256 algorithm. Some day we might
// be brave enough to implement EdDSA Curve25519 signatures (
// https://tools.ietf.org/html/draft-josefsson-eddsa-ed25519-02 ) but not
// today. The ECDSA implementation is largely based on the
// https://github.com/gtank/cryptopasta code. The reading and writing is mostly
// hacked together from the pem, elliptic, and ecdsa documentations, so these
// parts may not be entirely interoperable with other readers and writers.
package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"

	"github.com/pkg/errors"
)

// PublicKey embeds the ecdsa.PublicKey type with an extra Name and Comment
type PublicKey struct {
	*ecdsa.PublicKey
	Name    string
	Comment string
}

// PrivateKey embeds the ecdsa.PrivateKey type with an extra Name and Comment
type PrivateKey struct {
	*ecdsa.PrivateKey
	Name    string
	Comment string
}

// GetPublicKey returns the PublicKey from the PrivateKey. This should be used
// instead of pulling the PublicKey field directly out of the PrivateKey. That
// would pull an *ecdsa.PublicKey out, instead of a *crypto.PublicKey. I would
// liked to have just called this method PublicKey(), but that causes strange
// recursive function references, so we need to use GetPublicKey() instead.
func (k *PrivateKey) GetPublicKey() *PublicKey {
	return &PublicKey{
		PublicKey: &k.PublicKey,
		Name:      k.Name,
		Comment:   k.Comment,
	}
}

const (
	// PrivateKeyPEMType is the type recorded in the pem preamble for our private keys
	PrivateKeyPEMType = "ECDSA P256 PRIVATE KEY"
	// PublicKeyPEMType is the type recorded in the pem preamble for our public keys
	PublicKeyPEMType = "ECDSA P256 PUBLIC KEY"
	// CurveNameHeader is the key for the Curve-Name pem header
	CurveNameHeader = "Curve-Name"
	// NameHeader is the key for the Name pem header
	NameHeader = "Name"
	// CommentHeader is the key for the Comment pem header
	CommentHeader = "Comment"
)

// NewPrivateKey creates a random ECDSA P256 private key (which includes a public key)
func NewPrivateKey() (*PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create key")
	}

	return &PrivateKey{
		PrivateKey: k,
		Name:       "",
		Comment:    "",
	}, nil
}

// WritePrivateKey encodes the private key into the output Writer. The pem
// encoding is used, storing the private key D value in one block and the
// public key in a second. The public key is written using the WritePublicKey
// function. The pem headers include the curve name for future-proofing.
func (k *PrivateKey) Write(out io.Writer) error {
	priv := &pem.Block{
		Type: PrivateKeyPEMType,
		Headers: map[string]string{
			CurveNameHeader: k.Curve.Params().Name,
			NameHeader:      k.Name,
			CommentHeader:   k.Comment,
		},
		Bytes: k.D.Bytes(),
	}

	err := pem.Encode(out, priv)
	if err != nil {
		return errors.Wrap(err, "failed to encode private key")
	}

	return k.GetPublicKey().Write(out)
}

// WritePublicKey encodes the public key to the output Writer. The pem encoding
// is used, storing the X and Y values encoded by the elliptic.Marshal function
// and the appropriate curve. The pem headers include the curve name for
// future-proofing. If you are writing a public key separately from a
// crypto.PrivateKey, use the privateKey.GetPublicKey() method instead of
// pulling PublicKey field directly from the privateKey.
func (k *PublicKey) Write(out io.Writer) error {
	pub := &pem.Block{
		Type: PublicKeyPEMType,
		Headers: map[string]string{
			CurveNameHeader: k.Curve.Params().Name,
			NameHeader:      k.Name,
			CommentHeader:   k.Comment,
		},
		Bytes: elliptic.Marshal(k.Curve, k.X, k.Y),
	}

	err := pem.Encode(out, pub)
	if err != nil {
		return errors.Wrap(err, "failed to encode public key")
	}

	return nil
}

// ReadPrivateKey looks for the private and public key components of the
// ecdsa.PrivateKey in the Reader's bytes. If both are found, the pem blocks
// are decoded. The data is expected to have been written by the
// WritePrivateKey function.
func ReadPrivateKey(in io.Reader) (*PrivateKey, error) {
	b, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read all of the input bytes")
	}

	k := &PrivateKey{
		PrivateKey: &ecdsa.PrivateKey{},
		Name:       "",
		Comment:    "",
	}
	var foundPriv bool
	var foundPub bool

	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)

		if blk == nil {
			break
		}

		switch blk.Type {
		case PrivateKeyPEMType:
			if blk.Headers[CurveNameHeader] != elliptic.P256().Params().Name {
				return nil, errors.Errorf("unknown curve name: %s", blk.Headers[CurveNameHeader])
			}

			d := big.NewInt(0).SetBytes(blk.Bytes)

			k.Curve = elliptic.P256()
			k.D = d

			k.Name = blk.Headers[NameHeader]
			k.Comment = blk.Headers[CommentHeader]

			foundPriv = true

		case PublicKeyPEMType:
			pubKey, err := extractPublicKeyFromPEMBlock(blk)
			if err != nil {
				return nil, err
			}

			k.PublicKey = *pubKey

			foundPub = true
		}
	}

	if !foundPriv {
		return nil, errors.New("did not find a private key in the input")
	}

	if !foundPub {
		return nil, errors.New("did not find a public key in the input")
	}

	return k, nil
}

// ReadPublicKey looks for the public key components of the ecdsa.PublicKey in
// the Reader's bytes. The pem block is decoded. THe data is expected to have
// been written by the WritePublicKey or WritePrivateKey function.
func ReadPublicKey(in io.Reader) (*PublicKey, error) {
	b, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read all of the input bytes")
	}

	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)

		if blk == nil {
			break
		}

		switch blk.Type {
		case PublicKeyPEMType:
			pubKey, err := extractPublicKeyFromPEMBlock(blk)
			if err != nil {
				return nil, err
			}

			k := &PublicKey{
				PublicKey: pubKey,
				Name:      blk.Headers[NameHeader],
				Comment:   blk.Headers[CommentHeader],
			}

			return k, nil
		}
	}

	return nil, errors.New("did not find public key in the input")
}

func extractPublicKeyFromPEMBlock(b *pem.Block) (*ecdsa.PublicKey, error) {
	if b.Headers[CurveNameHeader] != elliptic.P256().Params().Name {
		return nil, errors.Errorf("unknown curve name: %s", b.Headers[CurveNameHeader])
	}

	k := &ecdsa.PublicKey{}

	x, y := elliptic.Unmarshal(elliptic.P256(), b.Bytes)

	k.Curve = elliptic.P256()
	k.X = x
	k.Y = y

	return k, nil
}

// Sign signs some arbitrary data with an ECDSA private keys, such as one
// created by NewSigningKey() . The signature can later be checked with the
// Verify function.
func Sign(d []byte, k *PrivateKey) ([]byte, error) {
	h := sha256.Sum256(d)

	r, s, err := ecdsa.Sign(rand.Reader, k.PrivateKey, h[:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign the digest")
	}

	size := k.Curve.Params().P.BitLen() / 8
	rb, sb := r.Bytes(), s.Bytes()
	sig := make([]byte, size*2)
	// this copy format is used to take care of leading zeros
	copy(sig[size-len(rb):size], rb)
	copy(sig[size*2-len(sb):], sb)

	return sig, nil
}

// Verify checks the arbitrary data and signature against the public key. This
// is a reciprocal of the Sign function.
func Verify(d, sig []byte, k *PublicKey) bool {
	h := sha256.Sum256(d)

	size := k.Curve.Params().P.BitLen() / 8

	r, s := new(big.Int), new(big.Int)

	r.SetBytes(sig[:size])
	s.SetBytes(sig[size:])

	return ecdsa.Verify(k.PublicKey, h[:], r, s)
}
