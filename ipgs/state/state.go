package state

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/pkg/errors"
)

const (
	StateLinkName        = "interplanetary-game-system"
	IdentityLinkName     = "identity.pem"
	PlayersLinkName      = "players"
	LastUpdatedFileName  = "last-updated"
	StateDirectoryName   = "state"
	PlayersDirectoryName = "players"
	PrivateKeyFileName   = "private.pem"
)

type State struct {
	LastUpdated time.Time
	Owner       *Player
	Players     []*Player
}

func NewState() *State {
	return &State{}
}

func (st *State) LastUpdatedString() string {
	return st.LastUpdated.UTC().Format(time.RFC3339Nano)
}

func (st *State) ParseLastUpdated(s string) error {
	t, err := time.ParseInLocation(time.RFC3339Nano, s, nil)
	if err != nil {
		return errors.Wrapf(err, "could not parse string '%s'", s)
	}

	st.LastUpdated = t

	return nil
}

func (st *State) Write(nodeDir string) error {
	tmp := filepath.Join(nodeDir, "state-tmp")

	err := os.RemoveAll(tmp)
	if err != nil {
		return errors.Wrap(err, "failed to remove temporary state directory")
	}

	err = os.Mkdir(tmp, 0700)
	if err != nil {
		return errors.Wrap(err, "failed to create temporary state directory")
	}

	err = ioutil.WriteFile(
		filepath.Join(tmp, LastUpdatedFileName),
		[]byte(st.LastUpdatedString()),
		0600,
	)
	if err != nil {
		return errors.Wrap(err, "failed to write temporary last-updated file")
	}

	pls := filepath.Join(tmp, PlayersDirectoryName)
	err = os.Mkdir(pls, 0700)
	if err != nil {
		return errors.Wrap(err, "failed to create players directory")
	}

	f, err := os.Create(
		filepath.Join(
			pls,
			"player-000.json",
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create owner file")
	}
	defer f.Close()

	err = st.Owner.Write(f)
	if err != nil {
		return errors.Wrap(err, "failed to write owner to file")
	}

	for i, p := range st.Players {
		f, err := os.Create(
			filepath.Join(
				pls,
				fmt.Sprintf("player-%03d.json", i+1),
			),
		)
		if err != nil {
			return errors.Wrap(err, "failed to create player file")
		}
		defer f.Close()

		err = p.Write(f)
		if err != nil {
			return errors.Wrap(err, "failed to write player to file")
		}
	}

	dir := filepath.Join(nodeDir, StateDirectoryName)
	err = os.RemoveAll(dir)
	if err != nil {
		return errors.Wrap(err, "failed to remove current state directory")
	}

	err = os.Rename(tmp, dir)
	if err != nil {
		return errors.Wrap(err, "failed to move temporary state directory to the permanent location")
	}

	return nil
}

func (st *State) Read(nodeDir string) error {
	dir := filepath.Join(nodeDir, StateDirectoryName)

	pkFile, err := os.Open(filepath.Join(nodeDir, PrivateKeyFileName))
	if err != nil {
		return errors.Wrap(err, "failed to open private key file")
	}
	defer pkFile.Close()

	pk, err := crypto.ReadPrivateKey(pkFile)
	if err != nil {
		return errors.Wrap(err, "failed to read private key from file")
	}
	ownerPrvKey := NewPrivateKey(pk)
	ownerPubKey := NewPublicKey(pk.GetPublicKey(), "")

	d, err := ioutil.ReadFile(filepath.Join(dir, LastUpdatedFileName))
	if err != nil {
		return errors.Wrap(err, "failed to read last-updated file")
	}

	err = st.ParseLastUpdated(string(d))
	if err != nil {
		return errors.Wrap(err, "failed to parse last-updated contents")
	}

	plDir := filepath.Join(dir, PlayersDirectoryName)
	pls, err := ioutil.ReadDir(plDir)
	if err != nil {
		return errors.Wrap(err, "failed to read players directory")
	}

	for _, pfInfo := range pls {
		plF, err := os.Open(filepath.Join(plDir, pfInfo.Name()))
		if err != nil {
			return errors.Wrap(err, "failed to open player file")
		}
		defer plF.Close()

		p := NewPlayer(nil, nil)
		err = p.Read(plF)
		if err != nil {
			return errors.Wrap(err, "failed to read player from file")
		}

		if p.Key().Equals(ownerPubKey) {
			if st.Owner != nil {
				return errors.Wrap(err, "found more than one player that could be the owner")
			}

			p.privateKey = ownerPrvKey

			st.Owner = p
		} else {
			st.Players = append(st.Players, p)
		}
	}
	if st.Owner == nil {
		return errors.New("did not find a player object for the state's owner")
	}

	return nil
}

func (st *State) Publish(s *cachedshell.Shell) (string, error) {
	h, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create state object")
	}

	h, err = s.PatchData(h, true, st.LastUpdatedString())
	if err != nil {
		return "", errors.Wrap(err, "failed to add last-updated to state")
	}

	ownerHash, err := st.Owner.Publish(s, st.Owner)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish owner")
	}

	ownerKeyHash, err := st.Owner.Key().Publish(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish owner's key")
	}

	h, err = s.PatchLink(h, IdentityLinkName, ownerKeyHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add identity link to state")
	}

	pHash, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create players object")
	}

	pHash, err = s.PatchLink(pHash, ownerKeyHash, ownerHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add owner link to players")
	}

	for _, p := range st.Players {
		pH, err := p.Publish(s, st.Owner)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish player")
		}

		pkH, err := p.Key().Publish(s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish player's key")
		}

		pHash, err = s.PatchLink(pHash, pkH, pH, false)
		if err != nil {
			return "", errors.Wrap(err, "failed to add player link to players")
		}
	}

	h, err = s.PatchLink(h, PlayersLinkName, pHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add players link to state")
	}

	return h, nil
}

func (st *State) Get(h string, s *cachedshell.Shell) error {
	obj, err := s.ObjectGet(h)
	if err != nil {
		return errors.Wrap(err, "failed to get state object")
	}

	err = st.ParseLastUpdated(obj.Data)
	if err != nil {
		return errors.Wrap(err, "failed to parse object data")
	}

	var identityHash string
	for _, l := range obj.Links {
		switch l.Name {

		case IdentityLinkName:
			identityHash = l.Hash

		}
	}

	pls, err := s.ObjectGet(fmt.Sprintf("%s/%s", h, PlayersLinkName))
	if err != nil {
		return errors.Wrap(err, "failed to get players object")
	}

	players := make([]*Player, len(pls.Links))
	for i, l := range pls.Links {
		p := NewPlayer(nil, nil)
		_, err = p.Get(l.Hash, s)
		if err != nil {
			return errors.Wrap(err, "failed to load get player from hash")
		}
		players[i] = p
	}

	for _, p := range players {
		if p.Key().Hash() == identityHash {
			if st.Owner != nil {
				return errors.Wrap(err, "found more than one player that could be the owner")
			}
			st.Owner = p
		} else {
			st.Players = append(st.Players, p)
		}
	}
	if st.Owner == nil {
		return errors.New("did not find a player object for the state's owner")
	}

	return nil
}

func (st *State) Commit(nodeDir string, s *cachedshell.Shell, unpin bool) error {
	err := st.Write(nodeDir)
	if err != nil {
		return errors.Wrap(err, "failed to write state to filesystem")
	}

	h, err := st.Publish(s)
	if err != nil {
		return errors.Wrap(err, "failed to publish state to IPFS")
	}

	log.Println("created state object at", h)

	cur, err := s.ResolveFresh("")
	if err != nil {
		if !strings.HasSuffix(err.Error(), "Could not resolve name.") {
			return errors.Wrap(err, "failed to resolve node's IPNS")
		}

		cur, err = s.NewObject("")
		if err != nil {
			return errors.Wrap(err, "failed to create IPNS base object")
		}
	}

	new, err := s.Patch(cur, "rm-link", StateLinkName)
	if err != nil {
		if !strings.HasSuffix(err.Error(), "not found") {
			return errors.Wrap(err, "failed to remove old state link")
		}

		new = cur
	}

	new, err = s.PatchLink(new, StateLinkName, h, false)
	if err != nil {
		return errors.Wrap(err, "failed to add state link to IPNS base")
	}

	err = s.Pin(new)
	if err != nil {
		return errors.Wrap(err, "failed to pin new IPNS base")
	}

	err = s.Publish("", new)
	if err != nil {
		return errors.Wrap(err, "failed to publish new IPNS base")
	}

	log.Println("published hash", new)

	if unpin && (new != cur) {
		err = s.Unpin(cur)
		if err != nil {
			log.Printf("failed to unpin old IPNS base %s: %+v\n", cur, err)
		}
	}

	log.Println("updated IPNS to", new)

	return nil
}

func FindStateForNode(nodeID string, s *cachedshell.Shell) (*State, error) {
	var search string
	if nodeID != "" {
		search = fmt.Sprintf("/ipns/%s", nodeID)
	}

	h, err := s.Resolve(search)
	if err != nil {
		return nil, errors.Wrapf(err, "could not resolve '%s'", search)
	}

	sh, err := s.ResolvePath(fmt.Sprintf("%s/%s", h, StateLinkName))
	if err != nil {
		return nil, errors.Wrapf(err, "no IPGS object under node '%s'", search)
	}

	st := NewState()
	err = st.Get(sh, s)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state from %s", sh)
	}

	return st, nil
}

func FindLatestState(nodeDir string, s *cachedshell.Shell, unpin bool) (*State, error) {
	fsSt := NewState()
	err := fsSt.Read(nodeDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read state from file system")
	}

	ipfsSt, err := FindStateForNode("", s)
	if err != nil {
		log.Printf("failed to find state in IPFS: %+v\n", err)
	}

	var st *State

	if ipfsSt == nil || fsSt.LastUpdated.After(ipfsSt.LastUpdated) {
		log.Println("filesystem state is more fresh than the IPFS one")

		st = fsSt
	} else {
		log.Println("IPFS state is at least as fresh as the filesystem one")

		st = ipfsSt

		err := st.Owner.addPrivateKey(fsSt.Owner.PrivateKey())
		if err != nil {
			return nil, errors.Wrap(err, "failed to add private key to owner")
		}
	}

	err = st.Commit(nodeDir, s, unpin)
	if err != nil {
		return nil, errors.Wrap(err, "failed to commit latest state")
	}

	return st, nil
}
