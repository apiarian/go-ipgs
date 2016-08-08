package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/apiarian/go-ipgs/cachedshell"
)

type ChallengeAcceptance struct {
	timeout   time.Time
	comment   string
	challenge *Challenge
	accepter  *Player
	timestamp time.Time
	signature []byte
	hash      string
}

type fileChallengeAcceptance struct {
	Timeout       IPGSTime
	Comment       string
	ChallengeHash string
	AccepterID    string
	Timestamp     IPGSTime
	Signature     []byte
	Hash          string
}

type ipfsChallengeAcceptance struct {
	Timeout IPGSTime
	Comment string
}

func NewChallengeAcceptance() *ChallengeAcceptance {
	return &ChallengeAcceptance{}
}

func (c *ChallengeAcceptance) Timeout() time.Time {
	return c.timeout
}

func (c *ChallengeAcceptance) Challenge() *Challenge {
	return c.challenge
}

func (c *ChallengeAcceptance) Accepter() *Player {
	return c.accepter
}

func (c *ChallengeAcceptance) Comment() string {
	return c.comment
}

func (c *ChallengeAcceptance) Type() string {
	return CommitTypeChallengeAcceptance
}

func (c *ChallengeAcceptance) Timestamp() time.Time {
	return c.timestamp
}

func (c *ChallengeAcceptance) ID() string {
	return fmt.Sprintf(
		"%s|%s",
		c.challenge.ID(),
		c.accepter.ID(),
	)
}

func (c *ChallengeAcceptance) Committer() *Player {
	return c.Accepter()
}

func (c *ChallengeAcceptance) Parent() Commit {
	return c.Challenge()
}

func (c *ChallengeAcceptance) Hash() string {
	return c.hash
}

func (c *ChallengeAcceptance) Signature() []byte {
	sig := make([]byte, len(c.signature))
	copy(sig, c.signature)

	return sig
}

func (c *ChallengeAcceptance) SignatureData() ([]byte, error) {
	if c.Challenge().hash == "" {
		return nil, errors.New("the parent challenge does not have a hash")
	}

	return []byte(fmt.Sprintf(
		"%s|%s|%s|%s",
		c.ID(),
		c.Timeout().UTC().Format(time.RFC3339Nano),
		c.Comment(),
		c.Challenge().hash,
	)), nil
}

func (c *ChallengeAcceptance) Sign() error {
	if len(c.signature) == 0 {
		sig, err := signCommit(c)
		if err != nil {
			return errors.Wrap(err, "failed to sign challenge acceptance")
		}

		c.signature = sig
	}

	return nil
}

func (c *ChallengeAcceptance) Verify() error {
	err := verifyCommit(c)
	if err != nil {
		return errors.Wrap(err, "failed to verify the challenge acceptance")
	}

	return nil
}

func (c *ChallengeAcceptance) IpfsJsonData() ([]byte, error) {
	d, err := json.Marshal(
		&ipfsChallengeAcceptance{
			Timeout: IPGSTime{c.Timeout()},
			Comment: c.Comment(),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal challenge data to JSON")
	}

	return d, nil
}

func getIpfsChallengeAcceptance(h string, s *cachedshell.Shell) (*ipfsChallengeAcceptance, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get challenge acceptance data object")
	}

	var c ipfsChallengeAcceptance
	err = json.Unmarshal([]byte(obj.Data), &c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal challenge acceptance data JSON")
	}

	return &c, nil
}

func (c *ChallengeAcceptance) Publish(s *cachedshell.Shell) (string, error) {
	if c.hash == "" {
		h, err := publishCommit(c, s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish challenge acceptance")
		}

		c.hash = h
	}

	return c.hash, nil
}
