package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/pkg/errors"
)

type Challenge struct {
	timeout    time.Time
	comment    string
	challenger *Player
	timestamp  time.Time
	signature  []byte
	hash       string
}

type fileChallenge struct {
	Timeout      IPGSTime
	Comment      string
	ChallengerID string
	Timestamp    IPGSTime
	Signature    []byte
	Hash         string
}

type ipfsChallenge struct {
	Timeout IPGSTime
	Comment string
}

func NewChallenge() *Challenge {
	return &Challenge{}
}

func (c *Challenge) Timeout() time.Time {
	return c.timeout
}

func (c *Challenge) Challenger() *Player {
	return c.challenger
}

func (c *Challenge) Comment() string {
	return c.comment
}

func (c *Challenge) Type() string {
	return CommitTypeChallenge
}

func (c *Challenge) Timestamp() time.Time {
	return c.timestamp
}

func (c *Challenge) ID() string {
	return fmt.Sprintf(
		"%s|%s",
		c.Challenger().ID(),
		c.Timestamp().UTC().Format(time.RFC3339Nano),
	)
}

func (c *Challenge) Committer() *Player {
	return c.Challenger()
}

func (c *Challenge) Parent() Commit {
	return nil
}

func (c *Challenge) Hash() string {
	return c.hash
}

func (c *Challenge) Signature() []byte {
	sig := make([]byte, len(c.signature))
	copy(sig, c.signature)

	return sig
}

func (c *Challenge) SignatureData() ([]byte, error) {
	return []byte(fmt.Sprintf(
		"%s|%s|%s|%s",
		c.ID(),
		c.Timeout().UTC().Format(time.RFC3339Nano),
		c.Comment(),
		"none",
	)), nil
}

func (c *Challenge) Sign() error {
	if len(c.signature) == 0 {
		sig, err := signCommit(c)
		if err != nil {
			return errors.Wrap(err, "failed to sign challenge")
		}

		c.signature = sig
	}

	return nil
}

func (c *Challenge) Verify() error {
	err := verifyCommit(c)
	if err != nil {
		return errors.Wrap(err, "failed to verify the challenge")
	}

	return nil
}

func (c *Challenge) IpfsJsonData() ([]byte, error) {
	d, err := json.Marshal(
		&ipfsChallenge{
			Timeout: IPGSTime{c.Timeout()},
			Comment: c.Comment(),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal challenge data to JSON")
	}

	return d, nil
}

func getIpfsChallenge(h string, s *cachedshell.Shell) (*ipfsChallenge, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get challenge data object")
	}

	var c ipfsChallenge
	err = json.Unmarshal([]byte(obj.Data), &c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal challenge data JSON")
	}

	return &c, nil
}

func (c *Challenge) Publish(s *cachedshell.Shell) (string, error) {
	if c.hash == "" {
		h, err := publishCommit(c, s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish challenge")
		}

		c.hash = h
	}

	return c.hash, nil
}

func (c *Challenge) clone() Commit {
	sig := make([]byte, len(c.signature))
	copy(sig, c.signature)

	return &Challenge{
		timeout:    c.timeout,
		comment:    c.comment,
		challenger: c.challenger,
		timestamp:  c.timestamp,
		signature:  sig,
		hash:       c.hash,
	}
}
