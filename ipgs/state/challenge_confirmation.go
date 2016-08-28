package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/apiarian/go-ipgs/cachedshell"
)

type ChallengeConfirmation struct {
	timeout    time.Time
	comment    string
	acceptance *ChallengeAcceptance
	confirmer  *Player
	timestamp  time.Time
	signature  []byte
	hash       string
}

type fileChallengeConfirmation struct {
	Timeout        IPGSTime
	Comment        string
	AcceptanceHash string
	ConfirmerID    string
	Timestamp      IPGSTime
	Signature      []byte
	Hash           string
}

type ipfsChallengeConfirmation struct {
	Timeout IPGSTime
	Comment string
}

func NewChallengeConfirmation() *ChallengeConfirmation {
	return &ChallengeConfirmation{}
}

func (c *ChallengeConfirmation) Timeout() time.Time {
	return c.timeout
}

func (c *ChallengeConfirmation) Acceptance() *ChallengeAcceptance {
	return c.acceptance
}

func (c *ChallengeConfirmation) Confirmer() *Player {
	return c.confirmer
}

func (c *ChallengeConfirmation) Comment() string {
	return c.comment
}

func (c *ChallengeConfirmation) Type() string {
	return CommitTypeChallengeConfirm
}

func (c *ChallengeConfirmation) Timestamp() time.Time {
	return c.timestamp
}

func (c *ChallengeConfirmation) ID() string {
	return c.acceptance.ID()
}

func (c *ChallengeConfirmation) Committer() *Player {
	return c.Confirmer()
}

func (c *ChallengeConfirmation) Parent() Commit {
	return c.Acceptance()
}

func (c *ChallengeConfirmation) Hash() string {
	return c.hash
}

func (c *ChallengeConfirmation) Signature() []byte {
	sig := make([]byte, len(c.signature))
	copy(sig, c.signature)

	return sig
}

func (c *ChallengeConfirmation) SignatureData() ([]byte, error) {
	if c.Acceptance().hash == "" {
		return nil, errors.New("the parent challenge does not have a hash")
	}

	return []byte(fmt.Sprintf(
		"%s|%s|%s|%s",
		c.ID(),
		c.Timeout().UTC().Format(time.RFC3339Nano),
		c.Comment(),
		c.Acceptance().hash,
	)), nil
}

func (c *ChallengeConfirmation) Sign() error {
	if len(c.signature) == 0 {
		sig, err := signCommit(c)
		if err != nil {
			return errors.Wrap(err, "failed to sign challenge confirmation")
		}

		c.signature = sig
	}

	return nil
}

func (c *ChallengeConfirmation) Verify() error {
	err := verifyCommit(c)
	if err != nil {
		return errors.Wrap(err, "failed to verify the challenge confirmation")
	}

	return nil
}

func (c *ChallengeConfirmation) IpfsJsonData() ([]byte, error) {
	d, err := json.Marshal(
		&ipfsChallengeConfirmation{
			Timeout: IPGSTime{c.Timeout()},
			Comment: c.Comment(),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal challenge confirmation data to JSON")
	}

	return d, nil
}

func getIpfsChallengeConfirmation(h string, s *cachedshell.Shell) (*ipfsChallengeConfirmation, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get challenge confirmation data object")
	}

	var c ipfsChallengeConfirmation
	err = json.Unmarshal([]byte(obj.Data), &c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal challenge confirmation data JSON")
	}

	return &c, nil
}

func (c *ChallengeConfirmation) Publish(s *cachedshell.Shell) (string, error) {
	if c.hash == "" {
		h, err := publishCommit(c, s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish challenge confirmation")
		}

		c.hash = h
	}

	return c.hash, nil
}

func (c *ChallengeConfirmation) clone() Commit {
	sig := make([]byte, len(c.signature))
	copy(sig, c.signature)

	return &ChallengeConfirmation{
		timeout:    c.timeout,
		comment:    c.comment,
		acceptance: c.acceptance.clone().(*ChallengeAcceptance),
		confirmer:  c.confirmer,
		timestamp:  c.timestamp,
		signature:  sig,
		hash:       c.hash,
	}
}
