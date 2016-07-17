package state

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/pkg/errors"
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

func (p *Player) CreateIPFSObject(s *cachedshell.CachedShell, identHash string) (string, error) {
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

var PlayersUrlRe = regexp.MustCompile(`^/players/([0-9A-Za-z]*)[/]?$`)

type playersPOSTformat struct {
	Nodes []string
}

func MakePlayersHandlerFunc(
	nodeDir string,
	c config.Config,
	s *cachedshell.CachedShell,
	st *State,
	mx *sync.Mutex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		defer mx.Unlock()

		sm := PlayersUrlRe.FindStringSubmatch(r.URL.Path)
		if len(sm) == 0 {
			http.Error(
				w,
				"available endpoints: /players/, /players/:player-id",
				http.StatusNotFound,
			)
			return
		}

		playerID := sm[1]

		pl := st.Players

		if playerID == "" {
			switch r.Method {
			case http.MethodGet:
				j, err := json.MarshalIndent(pl, "", "\t")
				if err != nil {
					log.Println("failed to marshal players to JSON:", err)
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write(j)
				w.Write([]byte("\n"))

			case http.MethodPost:
				body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
				if err != nil {
					log.Println("failed to read /players/ POST body:", err)
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				if err = r.Body.Close(); err != nil {
					log.Println("failed to close /players/ POST body:", err)
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}

				var postedPlayers playersPOSTformat
				err = json.Unmarshal(body, &postedPlayers)
				if err != nil || len(postedPlayers.Nodes) == 0 {
					http.Error(
						w,
						"invalid data format; sample format:\n",
						http.StatusBadRequest,
					)
					err = json.NewEncoder(w).Encode(
						playersPOSTformat{
							Nodes: []string{
								pl[st.IdentityHash].Nodes[0],
								"other-node-ids",
							},
						},
					)
					if err != nil {
						log.Println("failed to marshal sample /players/ POST struct: %s", err)
					}
					w.Write([]byte("\n"))
				}

				for _, pn := range postedPlayers.Nodes {
					ipnsHash, err := s.Resolve(fmt.Sprintf("/ipns/%s", pn))
					if err != nil {
						log.Println("failed to resolve provided node IPNS:", err)
						http.Error(
							w,
							fmt.Sprintf("could not resolve node %s", pn),
							http.StatusNotFound,
						)
						return
					}

					stHash, err := s.ResolvePath(fmt.Sprintf("%s/interplanetary-game-system", ipnsHash))
					if err != nil {
						log.Println("failed to find ipgs object on node:", err)
						http.Error(
							w,
							fmt.Sprintf("could not find interplanetary-game-system object for node %s", pn),
							http.StatusNotFound,
						)
						return
					}

					remoteSt, err := LoadFromHash(stHash, s)
					if err != nil {
						log.Println("failed to load ipgs object on node:", err)
						http.Error(
							w,
							fmt.Sprintf("could not load IPGS state for node %s", pn),
							http.StatusInternalServerError,
						)
						return
					}

					p, ok := remoteSt.Players[remoteSt.IdentityHash]
					if !ok {
						log.Println("could not find the playser's object at their node")
						http.Error(
							w,
							fmt.Sprintf("could not find player's object for node %s", pn),
							http.StatusInternalServerError,
						)
						return
					}

					st.Players[p.PublicKeyHash] = p
					st.LastUpdated = IPGSTime{time.Now()}
				}

				err = st.Publish(nodeDir, c, s)
				if err != nil {
					log.Println("failed to publish updated state", err)
					http.Error(
						w,
						"could not publish updated state",
						http.StatusInternalServerError,
					)
					return
				}

				w.WriteHeader(http.StatusCreated)

				return
			default:
				log.Printf("got request to %s on /players/", r.Method)
				http.Error(
					w,
					"available methods: GET, POST",
					http.StatusNotImplemented,
				)
				return
			}
		} else {
			fmt.Fprintf(w, "looking for player %s in %+v", playerID, pl)
		}
	}
}
