package state

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/pkg/errors"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
)

// Player describes the state of an IPGS player
type Player struct {
	// PublicKeyHash is the IPFS hash of the player's IPGS identity file
	PublicKeyHash string

	// PreviousVersionHash is the IPFS hash of the previous version of this player's
	// state information. This can be used to follow committed changes to the
	// state over time.
	PreviousVersionHash string

	// LastUpdated is the timestamp of the last change to this player's state
	Timestamp IPGSTime

	// Name is the human-friendly name of the player
	Name string

	// Nodes is a list of IPFS node IDs
	Nodes []string

	// Flags is a map of string keys corresponding to various behaviors a player
	// may exhibit (usually negative things, like not signing games or toxic
	// comments) and their count.
	Flags map[string]int
}

// ipfsPlayer is the ipfs object data form of the player, the PublicKeyHash and
// PreviousVersionHash strings are stored as ipfs object links instead of data
type ipfsPlayer struct {
	Timestamp IPGSTime
	Name      string
	Nodes     []string
	Flags     map[string]int
}

func (p *Player) ipfsPlayer() *ipfsPlayer {
	return &ipfsPlayer{
		Timestamp: p.Timestamp,
		Name:      p.Name,
		Nodes:     p.Nodes,
		Flags:     p.Flags,
	}
}

func (p *Player) CreateIPFSObject(s *cachedshell.Shell, identHash string) (string, error) {
	j, err := json.Marshal(p.ipfsPlayer())
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal player to JSON")
	}

	pHash, err := s.NewObject("")
	if err != nil {
		return "", errors.Wrap(err, "failed to create player object")
	}

	pHash, err = s.PatchData(pHash, true, string(j))
	if err != nil {
		return "", errors.Wrap(err, "failed putd data in player object")
	}

	pHash, err = s.PatchLink(pHash, "author-public-key", identHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add author-public-key link to player")
	}

	pHash, err = s.PatchLink(pHash, "player-public-key", p.PublicKeyHash, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add player-public-key link to player")
	}

	if p.PreviousVersionHash != "" {
		pHash, err = s.PatchLink(pHash, "previous-version", p.PreviousVersionHash, false)
		if err != nil {
			return "", errors.Wrap(err, "failed to add previous-version link to player")
		}
	}

	return pHash, nil
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

func MakePlayersGetOneHandler(
	st *State,
	mx *sync.Mutex,
) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		defer mx.Unlock()

		playerID := pat.Param(ctx, "id")

		player, ok := st.Players[playerID]

		if !ok {
			WriteError(w, errors.Errorf("no player with id '%s'", playerID), http.StatusNotFound)
			return
		}

		WriteJSON(w, player, http.StatusOK)
	}
}

func MakePlayersPostHandler(
	nodeDir string,
	cfg config.Config,
	s *cachedshell.Shell,
	st *State,
	mx *sync.Mutex,
) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		defer mx.Unlock()

		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "failed to read POST body"),
				http.StatusInternalServerError,
			)
			return
		}
		if err = r.Body.Close(); err != nil {
			WriteError(
				w,
				errors.Wrap(err, "failed to close POST body"),
				http.StatusInternalServerError,
			)
			return
		}

		var postedPlayers playersPOSTformat
		err = json.Unmarshal(body, &postedPlayers)
		if err != nil || len(postedPlayers.Nodes) == 0 {
			WriteError(
				w,
				errors.Wrap(
					err,
					`expected data format: {"Nodes":["node-id-1","node-id-2"]}`,
				),
				http.StatusBadRequest,
			)
			return
		}

		for _, pn := range postedPlayers.Nodes {
			ipnsHash, err := s.Resolve(fmt.Sprintf("/ipns/%s", pn))
			if err != nil {
				WriteError(
					w,
					errors.Wrapf(err, "could not resolve %s", pn),
					http.StatusNotFound,
				)
				return
			}

			stHash, err := s.ResolvePath(fmt.Sprintf("%s/interplanetary-game-system", ipnsHash))
			if err != nil {
				WriteError(
					w,
					errors.Wrapf(err, "could not find IPFS object under node %s", pn),
					http.StatusNotFound,
				)
				return
			}

			remoteSt, err := LoadFromHash(stHash, s)
			if err != nil {
				WriteError(
					w,
					errors.Wrapf(err, "could not load IPGS object for node %s", pn),
					http.StatusInternalServerError,
				)
				return
			}

			p, ok := remoteSt.Players[remoteSt.IdentityHash]
			if !ok {
				WriteError(
					w,
					errors.Wrapf(err, "could not find player's object for node %s", pn),
					http.StatusInternalServerError,
				)
				return
			}

			st.Players[p.PublicKeyHash] = p
			st.LastUpdated = IPGSTime{time.Now()}
		}

		err = st.Publish(nodeDir, cfg, s)
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not publish updated state"),
				http.StatusInternalServerError,
			)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}
