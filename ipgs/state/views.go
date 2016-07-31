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
	"github.com/pkg/errors"
	"goji.io/pat"

	"goji.io"
	"golang.org/x/net/context"
)

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

type viewPlayer struct {
	ID        string
	Timestamp IPGSTime
	Name      string
	Flags     map[string]int
	Nodes     []string
}

func (p *Player) viewPlayer() *viewPlayer {
	return &viewPlayer{
		ID:        p.ID(),
		Timestamp: IPGSTime{p.Timestamp},
		Name:      p.Name,
		Flags:     p.Flags,
		Nodes:     p.Nodes,
	}
}

func MakePlayersGetHandler(
	st *State,
	mx *sync.Mutex,
) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		defer mx.Unlock()

		players := []*viewPlayer{st.Owner.viewPlayer()}

		for _, p := range st.Players {
			players = append(players, p.viewPlayer())
		}

		WriteJSON(w, players, http.StatusOK)
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

		player := st.PlayerForID(playerID)

		if player == nil {
			WriteError(w, errors.Errorf("no player with id '%s'", playerID), http.StatusNotFound)
			return
		}

		WriteJSON(w, player.viewPlayer(), http.StatusOK)
	}
}

type playersPOSTformat struct {
	Nodes []string
}

func MakePlayersPostHandler(
	st *State,
	mx *sync.Mutex,
	nodeDir string,
	s *cachedshell.Shell,
	unpin bool,
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
			remoteSt, err := FindStateForNode(pn, s)
			if err != nil {
				WriteError(
					w,
					errors.Wrapf(err, "could not load IPGS state for node %s", pn),
					http.StatusNotFound,
				)
				return
			}

			changed := st.AddPlayer(remoteSt.Owner)
			if changed {
				st.LastUpdated = time.Now()
			}
		}

		err = st.Commit(nodeDir, s, unpin)
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not commit updated state"),
				http.StatusInternalServerError,
			)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}
