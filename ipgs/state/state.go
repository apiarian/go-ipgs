package state

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/pkg/errors"
)

const (
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

// Publish safely saves the state to the node directory and to IPFS. It writes
// the new state the nodeDir/state-tmp directory, then removes the old
// nodeDir/state and replaces it with nodeDir/state-tmp.  Finally it builds a
// new state IPFS object and publishes it into the node's IPNS object at the
// /ipns/[node]/interplanetary-game-system link.
//func (st *State) Publish(
//	nodeDir string,
//	cfg config.Config,
//	s *cachedshell.Shell,
//) error {
//	curObjHash, err := s.Resolve("")
//	if err != nil {
//		// TODO: change this to a more sensible error identification process
//		if !strings.HasSuffix(err.Error(), "Could not resolve name.") {
//			return errors.Wrap(err, "failed to resolve nodes IPNS")
//		}
//
//		curObjHash, err = s.NewObject("")
//		if err != nil {
//			return errors.Wrap(err, "failed to create new IPNS base object")
//		}
//	}
//
//	newObjHash, err := s.Patch(
//		curObjHash,
//		"rm-link",
//		"interplanetary-game-system",
//	)
//	if err != nil {
//		if !strings.HasSuffix(err.Error(), "not found") {
//			return errors.Wrap(err, "failed to remove old interplanetary-game-system link")
//		}
//		// the interplanetary-game-system link didn't exist anyway, so we'll use
//		// the existing object for our purposes
//		newObjHash = curObjHash
//	}
//
//	newObjHash, err = s.PatchLink(
//		newObjHash,
//		"interplanetary-game-system",
//		stHash,
//		false,
//	)
//	if err != nil {
//		return errors.Wrap(err, "failed to add state link to the base")
//	}
//
//	err = s.Pin(newObjHash)
//	if err != nil {
//		return errors.Wrap(err, "failed to pin the new IPNS base object")
//	}
//
//	err = s.Publish("", newObjHash)
//	if err != nil {
//		return errors.Wrap(err, "failed to publish new IPNS base object")
//	}
//
//	log.Printf("published new IPNS base: /ipfs/%s", newObjHash)
//
//	if cfg.IPGS.UnpinIPNS {
//		err = s.Unpin(curObjHash)
//		if err != nil {
//			log.Printf("failed to unpin old IPNS base object %s: %s", curObjHash, err)
//		}
//	}
//
//	return nil
//}
