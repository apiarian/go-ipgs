package state

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/pkg/errors"
	"goji.io"
	"golang.org/x/net/context"
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

func (p *Player) Key() *PublicKey {
	return p.publicKey
}

func (p *Player) PrivateKey() *PrivateKey {
	return p.privateKey
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

type playersPOSTformat struct {
	Nodes []string
}

func WriteJSON(w http.ResponseWriter, d interface{}, c int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	w.WriteHeader(c)

	err := json.NewEncoder(w).Encode(d)

	if err != nil {
		log.Println("failed to marshal %+v into JSON")

		w.WriteHeader(http.StatusInternalServerError)

		w.Write([]byte(`"internal server error"`))
		return
	}
}

func WriteError(w http.ResponseWriter, e error, c int) {
	log.Printf("returning error(%v) to user: %+v\n", c, e)

	WriteJSON(
		w,
		struct {
			Error, Details string
		}{
			e.Error(), fmt.Sprintf("%+v", e),
		},
		c,
	)
}

func MakePlayersGetHandler(
	st *State,
	mx *sync.Mutex,
) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		defer mx.Unlock()

		WriteJSON(w, st.Players, http.StatusOK)
	}
}

// func MakePlayersGetOneHandler(
// 	st *State,
// 	mx *sync.Mutex,
// ) goji.HandlerFunc {
// 	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
// 		mx.Lock()
// 		defer mx.Unlock()
//
// 		playerID := pat.Param(ctx, "id")
//
// 		player, ok := st.Players[playerID]
//
// 		if !ok {
// 			WriteError(w, errors.Errorf("no player with id '%s'", playerID), http.StatusNotFound)
// 			return
// 		}
//
// 		WriteJSON(w, player, http.StatusOK)
// 	}
// }

// func MakePlayersPostHandler(
// 	nodeDir string,
// 	cfg config.Config,
// 	s *cachedshell.Shell,
// 	st *State,
// 	mx *sync.Mutex,
// ) goji.HandlerFunc {
// 	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
// 		mx.Lock()
// 		defer mx.Unlock()
//
// 		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
// 		if err != nil {
// 			WriteError(
// 				w,
// 				errors.Wrap(err, "failed to read POST body"),
// 				http.StatusInternalServerError,
// 			)
// 			return
// 		}
// 		if err = r.Body.Close(); err != nil {
// 			WriteError(
// 				w,
// 				errors.Wrap(err, "failed to close POST body"),
// 				http.StatusInternalServerError,
// 			)
// 			return
// 		}
//
// 		var postedPlayers playersPOSTformat
// 		err = json.Unmarshal(body, &postedPlayers)
// 		if err != nil || len(postedPlayers.Nodes) == 0 {
// 			WriteError(
// 				w,
// 				errors.Wrap(
// 					err,
// 					`expected data format: {"Nodes":["node-id-1","node-id-2"]}`,
// 				),
// 				http.StatusBadRequest,
// 			)
// 			return
// 		}
//
// 		for _, pn := range postedPlayers.Nodes {
// 			stHash, err := util.FindIpgsHash(pn, s)
// 			if err != nil {
// 				WriteError(
// 					w,
// 					errors.Wrapf(err, "could not find IPGS object for node %s", pn),
// 					http.StatusNotFound,
// 				)
// 				return
// 			}
//
// 			remoteSt, err := LoadFromHash(stHash, s)
// 			if err != nil {
// 				WriteError(
// 					w,
// 					errors.Wrapf(err, "could not load IPGS object for node %s", pn),
// 					http.StatusInternalServerError,
// 				)
// 				return
// 			}
//
// 			p, ok := remoteSt.Players[remoteSt.IdentityHash]
// 			if !ok {
// 				WriteError(
// 					w,
// 					errors.Wrapf(err, "could not find player's object for node %s", pn),
// 					http.StatusInternalServerError,
// 				)
// 				return
// 			}
//
// 			st.Players[p.PublicKeyHash] = p
// 			st.LastUpdated = IPGSTime{time.Now()}
// 		}
//
// 		err = st.Publish(nodeDir, cfg, s)
// 		if err != nil {
// 			WriteError(
// 				w,
// 				errors.Wrap(err, "could not publish updated state"),
// 				http.StatusInternalServerError,
// 			)
// 			return
// 		}
//
// 		w.WriteHeader(http.StatusCreated)
// 	}
// }
