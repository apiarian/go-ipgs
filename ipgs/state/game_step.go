package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/apiarian/go-ipgs/cachedshell"
)

type GameStep struct {
	player    *Player
	data      []byte
	parent    Commit
	timestamp time.Time
	signature []byte
	hash      string
}

type fileGameStep struct {
	PlayerID   string
	Data       []byte
	ParentHash string
	Timestamp  IPGSTime
	Signature  []byte
	Hash       string
}

type ipfsGameStep struct {
	Data []byte
}

func NewGameStep() *GameStep {
	return &GameStep{}
}

func (g *GameStep) Player() *Player {
	return g.player
}

func (g *GameStep) Data() []byte {
	dat := make([]byte, len(g.data))
	copy(dat, g.data)

	return dat
}

func (g *GameStep) Type() string {
	return CommitTypeGameStep
}

func (g *GameStep) Timestamp() time.Time {
	return g.timestamp
}

func (g *GameStep) ID() string {
	return g.Parent().ID()
}

func (g *GameStep) Committer() *Player {
	return g.Player()
}

func (g *GameStep) Parent() Commit {
	return g.parent
}

func (g *GameStep) Hash() string {
	return g.hash
}

func (g *GameStep) Signature() []byte {
	sig := make([]byte, len(g.signature))
	copy(sig, g.signature)

	return sig
}

func (g *GameStep) SignatureData() ([]byte, error) {
	if g.Parent().Hash() == "" {
		return nil, errors.New("the parent does not have a hash")
	}

	b := bytes.NewBufferString(fmt.Sprintf(
		"%s|%s|",
		g.ID(),
		g.Timestamp().UTC().Format(time.RFC3339Nano),
	))

	_, err := b.Write(g.Data())
	if err != nil {
		return nil, errors.Wrap(err, "failed to write game data to buffer")
	}

	_, err = b.WriteString(fmt.Sprintf(
		"|%s",
		g.Parent().Hash(),
	))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write parent hash to buffer")
	}

	return b.Bytes(), nil
}

func (g *GameStep) Sign() error {
	if len(g.signature) == 0 {
		sig, err := signCommit(g)
		if err != nil {
			return errors.Wrap(err, "failed to sign game step")
		}

		g.signature = sig
	}

	return nil
}

func (g *GameStep) Verify() error {
	err := verifyCommit(g)
	if err != nil {
		return errors.Wrap(err, "failed to verify the game step")
	}

	return nil
}

func (g *GameStep) IpfsJsonData() ([]byte, error) {
	d, err := json.Marshal(
		&ipfsGameStep{
			Data: g.data,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal game step data to JSON")
	}

	return d, nil
}

func getIpfsGameStep(h string, s *cachedshell.Shell) (*ipfsGameStep, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get game step data object")
	}

	var c ipfsGameStep
	err = json.Unmarshal([]byte(obj.Data), &c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal game step data JSON")
	}

	return &c, nil
}

func (g *GameStep) Publish(s *cachedshell.Shell) (string, error) {
	if g.hash == "" {
		h, err := publishCommit(g, s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish game step")
		}

		g.hash = h
	}

	return g.hash, nil
}

func (g *GameStep) clone() Commit {
	dat := make([]byte, len(g.data))
	copy(dat, g.data)

	sig := make([]byte, len(g.signature))
	copy(sig, g.signature)

	return &GameStep{
		player:    g.player,
		data:      dat,
		parent:    g.parent.clone(),
		timestamp: g.timestamp,
		signature: sig,
		hash:      g.hash,
	}
}
