package state

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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

func GetRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		WriteError(
			w,
			errors.Wrap(err, "failed to read POST body"),
			http.StatusInternalServerError,
		)
		return nil, false
	}

	if err = r.Body.Close(); err != nil {
		WriteError(
			w,
			errors.Wrap(err, "failed to close POST body"),
			http.StatusInternalServerError,
		)
		return nil, false
	}

	return body, true
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

func MakePlayersGetHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		players := []*viewPlayer{st.Owner.viewPlayer()}

		for _, p := range st.Players {
			players = append(players, p.viewPlayer())
		}

		WriteJSON(w, players, http.StatusOK)
	}
}

func findPlayerForId(ctx context.Context, w http.ResponseWriter, r *http.Request, st *State) *Player {
	playerID := pat.Param(ctx, "id")

	player := st.PlayerForID(playerID)

	if player == nil {
		WriteError(w, errors.Errorf("no player with id '%s'", playerID), http.StatusNotFound)
	}

	return player
}

func MakePlayersGetOneHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		player := findPlayerForId(ctx, w, r, st)

		if player == nil {
			return
		}

		WriteJSON(w, player.viewPlayer(), http.StatusOK)
	}
}

type playersPATCHformat struct {
	Name string
}

func MakePlayersPatchHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		player := findPlayerForId(ctx, w, r, st)
		if player == nil {
			return
		}

		body, ok := GetRequestBody(w, r)
		if !ok {
			return
		}

		var patchContent playersPATCHformat
		err := json.Unmarshal(body, &patchContent)
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, `expected data format: {"Name":"new-name"}`),
				http.StatusBadRequest,
			)
			return
		}

		var changed bool

		if patchContent.Name != "" && player.Name != patchContent.Name {
			if player != st.Owner {
				WriteError(
					w,
					errors.New("can only rename the owner"),
					http.StatusForbidden,
				)
				return
			}

			player.Name = patchContent.Name
			player.Timestamp = time.Now()
			changed = true
		}

		if changed {
			err := b.Checkin()
			if err != nil {
				WriteError(
					w,
					errors.Wrap(err, "failed to checkin changed state"),
					http.StatusInternalServerError,
				)
				return
			}
		}
	}
}

type playersPOSTformat struct {
	Nodes []string
}

func MakePlayersPostHandler(b *Broker, s *cachedshell.Shell) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		body, ok := GetRequestBody(w, r)
		if !ok {
			return
		}

		var postedPlayers playersPOSTformat
		err := json.Unmarshal(body, &postedPlayers)
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

		var changed bool

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

			c := st.AddPlayer(remoteSt.Owner)

			if c {
				changed = true
			}
		}

		if changed {
			err := b.Checkin()
			if err != nil {
				WriteError(
					w,
					errors.Wrap(err, "could not checkin updated state"),
					http.StatusInternalServerError,
				)
				return
			}
		}

		w.WriteHeader(http.StatusCreated)
	}
}

type viewChallenge struct {
	ID           string
	Timestamp    IPGSTime
	ChallengerID string
	Timeout      IPGSTime
	Comment      string
}

func (g *Game) viewChallenge() *viewChallenge {
	c := g.Challenge()
	if c == nil {
		return nil
	}

	return &viewChallenge{
		ID:           c.ID(),
		Timestamp:    IPGSTime{c.Timestamp()},
		ChallengerID: c.Challenger().ID(),
		Timeout:      IPGSTime{c.Timeout()},
		Comment:      c.Comment(),
	}
}

func MakeChallengesGetHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		var challenges []*viewChallenge

		for _, g := range st.Challenges() {
			challenges = append(challenges, g.viewChallenge())
		}

		WriteJSON(w, challenges, http.StatusOK)
	}
}

func findChallengeForID(ctx context.Context, w http.ResponseWriter, r *http.Request, st *State) *Game {
	gameID := pat.Param(ctx, "id")

	game := st.Game(gameID)
	if game == nil || game.Challenge() == nil {
		WriteError(w, errors.Errorf("no challenge with id '%s'", gameID), http.StatusNotFound)
		return nil
	}

	return game
}

func MakeChallengesGetOneHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		game := findChallengeForID(ctx, w, r, st)

		if game == nil {
			return
		}

		WriteJSON(w, game.viewChallenge(), http.StatusOK)
	}
}

type challengesPOSTformat struct {
	TimeoutMinutes int
	Comment        string
}

func MakeChallengesPostHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		body, ok := GetRequestBody(w, r)
		if !ok {
			return
		}

		var postedChallenge challengesPOSTformat
		err := json.Unmarshal(body, &postedChallenge)
		if err != nil || postedChallenge.TimeoutMinutes == 0 {
			WriteError(
				w,
				errors.Wrap(
					err,
					`expected data format: {"TimeoutMinutes": 60, "Comment": "friendly game"}`,
				),
				http.StatusBadRequest,
			)
			return
		}

		_, err = st.CreateGame(
			time.Duration(postedChallenge.TimeoutMinutes)*time.Minute,
			postedChallenge.Comment,
		)
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not create challenge"),
				http.StatusInternalServerError,
			)
			return
		}

		err = b.Checkin()
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not checkin updated state"),
				http.StatusInternalServerError,
			)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

type acceptPOSTformat struct {
	TimeoutMinutes int
	Comment        string
}

func MakeChallengesAcceptHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		body, ok := GetRequestBody(w, r)
		if !ok {
			return
		}

		game := findChallengeForID(ctx, w, r, st)

		if game == nil {
			return
		}

		var postedAcceptance acceptPOSTformat
		err := json.Unmarshal(body, &postedAcceptance)
		if err != nil || postedAcceptance.TimeoutMinutes == 0 {
			WriteError(
				w,
				errors.Wrap(
					err,
					`expected data format: {"TimeoutMinutes": 60, "Comment": "lets go!"}`,
				),
				http.StatusBadRequest,
			)
			return
		}

		_, err = st.AcceptGame(
			game.ID(),
			time.Duration(postedAcceptance.TimeoutMinutes)*time.Minute,
			postedAcceptance.Comment,
		)
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not accept challenge"),
				http.StatusInternalServerError,
			)
			return
		}

		err = b.Checkin()
		if err != nil {
			WriteError(
				w,
				errors.Wrap(err, "could not checking updated state"),
				http.StatusInternalServerError,
			)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

type viewGame struct {
	ID                  string
	Timestamp           IPGSTime
	ChallengerID        string
	AccepterID          string
	Timeout             IPGSTime
	ChallengeComment    string
	AcceptanceComment   string
	ConfirmationComment string
	Confirmed           bool
}

func (g *Game) viewGame() *viewGame {
	a := g.Acceptance()
	if a == nil {
		return nil
	}

	c := g.Challenge()
	if c == nil {
		return nil
	}

	vg := &viewGame{
		ID:                g.ID(),
		Timestamp:         IPGSTime{g.head.Timestamp()},
		ChallengerID:      c.Challenger().ID(),
		AccepterID:        a.Accepter().ID(),
		Timeout:           IPGSTime{g.Timeout()},
		ChallengeComment:  c.Comment(),
		AcceptanceComment: a.Comment(),
	}

	o := g.Confirmation()
	if o != nil {
		vg.ConfirmationComment = o.Comment()
		vg.Confirmed = true
	}

	return vg
}

func MakeGamesGetHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		var games []*viewGame

		for _, g := range st.Games() {
			games = append(games, g.viewGame())
		}

		WriteJSON(w, games, http.StatusOK)
	}
}

func MakeGamesGetOneHandler(b *Broker) goji.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		st := b.Checkout()
		defer b.Return()

		gameID := pat.Param(ctx, "id")

		game := st.Game(gameID)
		if game == nil || game.Acceptance() == nil {
			WriteError(w, errors.Errorf("no game with id '%s'", gameID), http.StatusNotFound)
			return
		}

		WriteJSON(w, game.viewGame(), http.StatusOK)
	}
}
