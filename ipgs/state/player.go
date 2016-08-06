package state

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/pkg/errors"
)

const (
	AuthorPublicKeyLinkName = "author-public-key"
	PlayerPublicKeyLinkName = "player-public-key"
)

type Player struct {
	Timestamp  time.Time
	Name       string
	Flags      map[string]int
	Nodes      []string
	publicKey  *PublicKey
	privateKey *PrivateKey
}

func NewPlayer(pub *PublicKey, priv *PrivateKey) *Player {
	return &Player{
		Flags:      make(map[string]int),
		publicKey:  pub,
		privateKey: priv,
	}
}

func (p *Player) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.ID())
}

func (p *Player) Key() *PublicKey {
	return p.publicKey
}

func (p *Player) PrivateKey() *PrivateKey {
	return p.privateKey
}

func (p *Player) Update(o *Player) (bool, error) {
	var changed bool

	if !p.Key().Equals(o.Key()) {
		return changed, errors.New("player keys do not match")
	}

	if p.Name != o.Name {
		p.Name = o.Name
		changed = true
	}

	difNodes := len(p.Nodes) != len(o.Nodes)
	if !difNodes {
		for _, oN := range o.Nodes {
			var found bool
			for _, pN := range p.Nodes {
				if oN == pN {
					found = true
					break
				}
			}

			if !found {
				difNodes = true
				break
			}
		}
	}

	if difNodes {
		p.Nodes = o.Nodes
		changed = true
	}

	if changed {
		p.Timestamp = time.Now()
	}

	return changed, nil
}

func (p *Player) addPrivateKey(k *PrivateKey) error {
	if p.privateKey != nil {
		return errors.New("cannot replace existing private key")
	}

	if p.publicKey == nil {
		return errors.New("cannot add a private key without a public key in place")
	}

	pub := NewPublicKey(k.Key().GetPublicKey(), "")
	if !p.publicKey.Equals(pub) {
		return errors.New("existing public key does not match provided private key")
	}

	p.privateKey = k

	return nil
}

type filePlayer struct {
	Timestamp IPGSTime
	Name      string
	Flags     map[string]int
	Key       *PublicKey
	Nodes     []string
}

func (p *Player) filePlayer() *filePlayer {
	return &filePlayer{
		Timestamp: IPGSTime{p.Timestamp},
		Name:      p.Name,
		Flags:     p.Flags,
		Key:       p.Key(),
		Nodes:     p.Nodes,
	}
}

func (p *Player) fromFilePlayer(fp *filePlayer) {
	p.Timestamp = fp.Timestamp.Time
	p.Name = fp.Name
	p.Flags = fp.Flags
	p.Nodes = fp.Nodes
	p.publicKey = fp.Key
}

func (p *Player) Write(out io.Writer) error {
	fp := p.filePlayer()
	err := json.NewEncoder(out).Encode(fp)
	if err != nil {
		return errors.Wrap(err, "failed to encode player")
	}

	return nil
}

func (p *Player) Read(in io.Reader) error {
	var fp *filePlayer
	err := json.NewDecoder(in).Decode(&fp)
	if err != nil {
		return errors.Wrap(err, "failed to decode player")
	}

	p.fromFilePlayer(fp)

	return nil
}

type ipfsPlayer struct {
	Timestamp IPGSTime
	Name      string
	Flags     map[string]int
	Nodes     []string
}

func (p *Player) ipfsPlayer() *ipfsPlayer {
	return &ipfsPlayer{
		Timestamp: IPGSTime{p.Timestamp},
		Name:      p.Name,
		Flags:     p.Flags,
		Nodes:     p.Nodes,
	}
}

func (p *Player) fromIpfsPlayer(ip *ipfsPlayer) {
	p.Timestamp = ip.Timestamp.Time
	p.Name = ip.Name
	p.Flags = ip.Flags
	p.Nodes = ip.Nodes
}

func (p *Player) Publish(s *cachedshell.Shell, author *Player) (string, error) {
	j, err := json.Marshal(p.ipfsPlayer())
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal player to JSON")
	}

	h, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create empty player object")
	}

	h, err = s.PatchData(h, true, string(j))
	if err != nil {
		return "", errors.Wrap(err, "failed to add data to player object")
	}

	authorKeyHash, err := author.Key().Publish(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish author public key")
	}

	playerKeyHash, err := p.Key().Publish(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish player public key")
	}

	h, err = s.PatchLink(h, AuthorPublicKeyLinkName, authorKeyHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add author key link to player object")
	}

	h, err = s.PatchLink(h, PlayerPublicKeyLinkName, playerKeyHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add player key link to player object")
	}

	return h, nil
}

func (p *Player) Get(h string, s *cachedshell.Shell) (string, error) {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return "", errors.Wrap(err, "failed to get player object")
	}

	var ip *ipfsPlayer
	err = json.Unmarshal([]byte(obj.Data), &ip)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal player JSON")
	}

	p.fromIpfsPlayer(ip)

	var authorKeyHash string

	for _, l := range obj.Links {
		switch l.Name {

		case AuthorPublicKeyLinkName:
			authorKeyHash = l.Hash

		case PlayerPublicKeyLinkName:
			k := NewPublicKey(nil, "")
			err := k.Get(l.Hash, s)
			if err != nil {
				return "", errors.Wrap(err, "failed to get public key")
			}
			p.publicKey = k

		}
	}

	return authorKeyHash, nil
}

func (p *Player) ID() string {
	if p == nil || p.Key() == nil {
		return ""
	}

	return p.Key().Hash()
}
