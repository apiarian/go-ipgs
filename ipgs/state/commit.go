package state

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/pkg/errors"
)

const (
	CommitterPublicKeyLinkName    = "committer-public-key"
	DataLinkName                  = "data"
	ParentLinkName                = "parent"
	CommitTypeChallenge           = "challenge-offer"
	CommitTypeChallengeAcceptance = "challenge-accept"
	CommitTypeChallengeConfirm    = "challenge-confirm"
	CommitTypeGameStep            = "game-step"
)

type Commit interface {
	Type() string
	Timestamp() time.Time
	ID() string
	Committer() *Player
	Parent() Commit
	Hash() string
	Signature() []byte
	SignatureData() ([]byte, error)
	Sign() error
	Verify() error
	IpfsJsonData() ([]byte, error)
	Publish(*cachedshell.Shell) (string, error)
}

func signCommit(c Commit) ([]byte, error) {
	if c.Committer().PrivateKey() == nil {
		return nil, errors.New("signer's private key is not available")
	}

	d, err := c.SignatureData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get data for signature")
	}

	sig, err := crypto.Sign(
		d,
		c.Committer().PrivateKey().Key(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign commit data")
	}

	return sig, nil
}

func verifyCommit(c Commit) error {
	if c.Committer() == nil {
		return errors.New("commit does not seem to have a committer")
	}

	if c.Committer().Key() == nil {
		return errors.New("signer's public key is not available")
	}

	d, err := c.SignatureData()
	if err != nil {
		return errors.Wrap(err, "failed to get data for verification")
	}

	ok := crypto.Verify(
		d,
		c.Signature(),
		c.Committer().Key().Key(),
	)

	if !ok {
		return errors.New("signature is not ok")
	}

	return nil
}

type ipfsCommit struct {
	Timestamp  IPGSTime
	CommitType string
	Signature  string
}

func publishCommit(c Commit, s *cachedshell.Shell) (string, error) {
	if c.Parent() != nil && c.Parent().Hash() == "" {
		_, err := c.Parent().Publish(s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish parent commit")
		}
	}

	if len(c.Signature()) == 0 {
		err := c.Sign()
		if err != nil {
			return "", errors.Wrap(err, "failed to sign commit")
		}
	}

	d, err := c.IpfsJsonData()
	if err != nil {
		return "", errors.Wrap(err, "failed to get IPFS JSON data for commit")
	}

	dh, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create empty commit data object")
	}

	dh, err = s.PatchData(dh, true, string(d))
	if err != nil {
		return "", errors.Wrap(err, "failed to add data to commit data object")
	}

	committerKeyHash, err := c.Committer().Key().Publish(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish committer public key")
	}

	sig := &pem.Block{
		Type:  crypto.SignaturePEMType,
		Bytes: c.Signature(),
	}
	sigBuf := bytes.Buffer{}
	err = pem.Encode(&sigBuf, sig)
	if err != nil {
		return "", errors.Wrap(err, "failed to encode commmit signature")
	}

	x, err := json.Marshal(
		&ipfsCommit{
			Timestamp:  IPGSTime{c.Timestamp()},
			CommitType: c.Type(),
			Signature:  sigBuf.String(),
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal commit metadata to JSON")
	}

	ch, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create empty commit object")
	}

	ch, err = s.PatchData(ch, true, string(x))
	if err != nil {
		return "", errors.Wrap(err, "failed to add commit metadata to object")
	}

	ch, err = s.PatchLink(ch, CommitterPublicKeyLinkName, committerKeyHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add commiter key link to commit object")
	}

	ch, err = s.PatchLink(ch, DataLinkName, dh, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add data link to commit object")
	}

	if c.Parent() != nil {
		ch, err = s.PatchLink(ch, ParentLinkName, c.Parent().Hash(), false)
		if err != nil {
			return "", errors.Wrap(err, "failed to add parent link to commit object")
		}
	}

	return ch, nil
}

type rawCommit struct {
	Hash          string
	Timestamp     time.Time
	Type          string
	Signature     []byte
	CommitterHash string
	DataHash      string
	ParentHash    string
}

func getRawCommit(h string, s *cachedshell.Shell) (*rawCommit, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get commit object")
	}

	rc := &rawCommit{Hash: h}

	var ic ipfsCommit
	err = json.Unmarshal([]byte(obj.Data), &ic)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal commit metadata JSON")
	}

	rc.Timestamp = ic.Timestamp.Time
	rc.Type = ic.CommitType

	sig := []byte(ic.Signature)
	for {
		var blk *pem.Block
		blk, sig = pem.Decode(sig)

		if blk == nil {
			break
		}

		switch blk.Type {
		case crypto.SignaturePEMType:
			rc.Signature = blk.Bytes
		}
	}

	for _, l := range obj.Links {
		switch l.Name {
		case CommitterPublicKeyLinkName:
			rc.CommitterHash = l.Hash

		case DataLinkName:
			rc.DataHash = l.Hash

		case ParentLinkName:
			rc.ParentHash = l.Hash
		}
	}

	return rc, nil
}
