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
	ChallengesLinkName   = "challenges"
	GamesLinkName        = "games"
	LastUpdatedFileName  = "last-updated"
	StateDirectoryName   = "state"
	PlayersDirectoryName = "players"
	GamesDirectoryName   = "games"
	PrivateKeyFileName   = "private.pem"
)

type State struct {
	LastUpdated time.Time
	Owner       *Player
	Players     []*Player
	games       map[string]*Game
}

func NewState() *State {
	return &State{
		games: make(map[string]*Game),
	}
}

func (st *State) LastUpdatedString() string {
	return st.LastUpdated.UTC().Format(time.RFC3339Nano)
}

func (st *State) ParseLastUpdated(s string) error {
	t, err := time.Parse(time.RFC3339Nano, s)
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

	gms := filepath.Join(tmp, GamesDirectoryName)
	err = os.Mkdir(gms, 0700)
	if err != nil {
		return errors.Wrap(err, "failed to create games directory")
	}

	for k, g := range st.games {
		f, err := os.Create(
			filepath.Join(
				gms,
				fmt.Sprintf("%s.json", k),
			),
		)
		if err != nil {
			return errors.Wrap(err, "failed to create game file")
		}
		defer f.Close()

		err = g.Write(f)
		if err != nil {
			return errors.Wrap(err, "failed to write game to file")
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

	playerLib := []*Player{st.Owner}
	for _, p := range st.Players {
		playerLib = append(playerLib, p)
	}

	gmDir := filepath.Join(dir, GamesDirectoryName)
	gms, err := ioutil.ReadDir(gmDir)
	if err != nil {
		return errors.Wrap(err, "failed to read games directory")
	}

	for _, gfInfo := range gms {
		gmF, err := os.Open(filepath.Join(gmDir, gfInfo.Name()))
		if err != nil {
			return errors.Wrap(err, "failed to open game file")
		}
		defer gmF.Close()

		g, err := ReadGame(gmF, playerLib)
		if err != nil {
			return errors.Wrap(err, "failed to read game from file")
		}

		i := g.ID()

		if i == "" {
			return errors.Errorf("game loaded from %s has an empty ID", gfInfo.Name())
		}

		if _, ok := st.games[i]; ok {
			return errors.Errorf("game with id %s already exists", i)
		}

		st.games[i] = g
	}

	return nil
}

func (st *State) Publish(s *cachedshell.Shell) (string, error) {
	h, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create state object")
	}

	emptyObjectH := h

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

	pHash := emptyObjectH
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

	chHash := emptyObjectH
	gHash := emptyObjectH

	for _, g := range st.games {
		gH, err := g.Publish(s)
		if err != nil {
			return "", errors.Wrap(err, "failed to publish game")
		}

		if g.Acceptance() != nil {
			gHash, err = s.PatchLink(gHash, g.ID(), gH, false)
			if err != nil {
				return "", errors.Wrap(err, "failed to add game to games object")
			}
		} else if g.Challenge() != nil {
			chHash, err = s.PatchLink(chHash, g.ID(), gH, false)
			if err != nil {
				return "", errors.Wrap(err, "failed to add game to challenges object")
			}
		} else {
			return "", errors.New("found a game without a challenge or acceptance")
		}
	}

	if chHash != emptyObjectH {
		h, err = s.PatchLink(h, ChallengesLinkName, chHash, false)
		if err != nil {
			return "", errors.Wrap(err, "failed to add challenges link to state")
		}
	}

	if gHash != emptyObjectH {
		h, err = s.PatchLink(h, GamesLinkName, gHash, false)
		if err != nil {
			return "", errors.Wrap(err, "failed to add games link to state")
		}
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

	for _, l := range obj.Links {
		switch l.Name {

		case ChallengesLinkName, GamesLinkName:
			gObj, err := s.ObjectGet(l.Hash)
			if err != nil {
				return errors.Wrap(err, "failed to get games or challenges object")
			}

			for _, gl := range gObj.Links {
				g, err := GetGame(gl.Hash, s, players)
				if err != nil {
					return errors.Wrap(err, "failed to get game")
				}

				i := g.ID()

				if i == "" {
					return errors.Errorf("game at %s has an empty ID", gl.Hash)
				}

				if _, ok := st.games[i]; ok {
					return errors.Errorf("game with id %s already exists", i)
				}

				st.games[i] = g
			}

		}
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

func (s *State) PlayerForID(id string) *Player {
	if id == "" {
		return nil
	}

	if s.Owner.ID() == id {
		return s.Owner
	}

	for _, p := range s.Players {
		if p.ID() == id {
			return p
		}
	}

	return nil
}

func (s *State) AddPlayer(p *Player) (changed bool) {
	existing := s.PlayerForID(p.ID())
	if existing != nil {
		return false
	}

	s.Players = append(s.Players, p)
	return true
}

func (s *State) Combine(o *State) (bool, error) {
	var changed bool

	ourP := s.PlayerForID(o.Owner.ID())
	if ourP == nil {
		return changed, errors.New("failed to find the other state's owner in our player list")
	}

	changed, err := ourP.Update(o.Owner)
	if err != nil {
		return changed, errors.Wrap(err, "failed to update our version of the player")
	}

	if changed {
		s.LastUpdated = time.Now()
	}

	return changed, nil
}

func (st *State) Game(id string) *Game {
	return st.games[id]
}

func (st *State) CreateGame(exp time.Duration, c string) (string, error) {
	g, err := CreateGame(
		st.Owner,
		exp,
		c,
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to create game")
	}

	i := g.ID()

	_, ok := st.games[i]
	if ok {
		return "", errors.Errorf("a game with id %s already exists", i)
	}

	st.games[i] = g

	return i, nil
}

func (st *State) AcceptGame(id string, exp time.Duration, c string) (string, error) {
	g := st.Game(id)
	if g == nil {
		return "", errors.New("game does not exist")
	}

	err := g.Accept(
		st.Owner,
		exp,
		c,
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to accept game")
	}

	i := g.ID()

	if i == id {
		return "", errors.New("accepted game has the same as the challenge")
	}

	_, ok := st.games[i]
	if ok {
		return "", errors.Errorf("this game id %s has already been accepted", i)
	}

	delete(st.games, id)
	st.games[i] = g

	return i, nil
}

func (st *State) ConfirmGame(id string, exp time.Duration, c string) error {
	g := st.Game(id)
	if g == nil {
		return errors.New("game does not exist")
	}

	err := g.Confirm(
		st.Owner,
		exp,
		c,
	)
	if err != nil {
		return errors.Wrap(err, "failed to confirm game")
	}

	i := g.ID()
	if i != id {
		return errors.Errorf("confirmed game has a different id: %s", i)
	}

	return nil
}

func (st *State) StepGame(id string, data []byte) error {
	g := st.Game(id)
	if g == nil {
		return errors.New("game does not exist")
	}

	err := g.Step(st.Owner, data)
	if err != nil {
		return errors.Wrap(err, "failed to step game")
	}

	i := g.ID()
	if i != id {
		return errors.Errorf("stepped game has a different id: %s", i)
	}

	return nil
}
