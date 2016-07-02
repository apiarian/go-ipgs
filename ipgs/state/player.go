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

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
)

// Player describes the state of an IPGS player
type Player struct {
	// PublicKeyHash is the IPFS hash of the player's IPGS identity.asc file
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
		return "", fmt.Errorf("failed to marshal player to JSON: %s", err)
	}

	pHash, err := s.NewObject("")
	if err != nil {
		return "", fmt.Errorf("failed to create player object: %s", err)
	}

	pHash, err = s.PatchData(pHash, true, string(j))
	if err != nil {
		return "", fmt.Errorf("failed putd data in player object: %s", err)
	}

	pHash, err = s.PatchLink(pHash, "author-public-key", identHash, false)
	if err != nil {
		return "", fmt.Errorf("failed to add author-public-key link to player: %s", err)
	}

	pHash, err = s.PatchLink(pHash, "player-public-key", p.PublicKeyHash, false)
	if err != nil {
		return "", fmt.Errorf("failed to add player-public-key link to player: %s", err)
	}

	if p.PreviousVersionHash != "" {
		pHash, err = s.PatchLink(pHash, "previous-version", p.PreviousVersionHash, false)
		if err != nil {
			return "", fmt.Errorf("failed to add previous-version link to player: %s", err)
		}
	}

	return pHash, nil
}

var PlayersUrlRe = regexp.MustCompile(`^/players/([0-9A-Za-z]*)[/]?$`)

type playersPOSTformat struct {
	Nodes []string
}

func MakePlayersHandlerFunc(
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
								pl[0].Nodes[0],
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
					log.Println("going to look for player data at node", pn)
				}

				http.Error(w, "players POST still not fully implemented", http.StatusNotImplemented)
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
