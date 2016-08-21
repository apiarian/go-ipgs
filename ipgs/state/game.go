package state

import (
	"encoding/json"
	"io"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/pkg/errors"
)

type Game struct {
	head Commit
}

func NewGame() *Game {
	return &Game{}
}

func (g *Game) findCommit(f func(Commit) bool) Commit {
	p := g.head

	for {
		if p == nil {
			return nil
		}

		if f(p) {
			return p
		}

		p = p.Parent()
	}
}

func (g *Game) Challenge() *Challenge {
	ch := g.findCommit(func(c Commit) bool {
		_, ok := c.(*Challenge)
		return ok
	})

	if ch == nil {
		return nil
	}

	return ch.(*Challenge)
}

func (g *Game) Acceptance() *ChallengeAcceptance {
	ch := g.findCommit(func(c Commit) bool {
		_, ok := c.(*ChallengeAcceptance)
		return ok
	})

	if ch == nil {
		return nil
	}

	return ch.(*ChallengeAcceptance)
}

func (g *Game) Confirmation() *ChallengeConfirmation {
	ch := g.findCommit(func(c Commit) bool {
		_, ok := c.(*ChallengeConfirmation)
		return ok
	})

	if ch == nil {
		return nil
	}

	return ch.(*ChallengeConfirmation)
}

func (g *Game) Steps() []*GameStep {
	var s []*GameStep

	p := g.head

	for {
		gs, ok := p.(*GameStep)
		if !ok {
			break
		}

		s = append(s, gs)

		p = p.Parent()
	}

	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}

	return s
}

func (g *Game) ID() string {
	return g.head.ID()
}

func (g *Game) Timestamp() time.Time {
	return g.head.Timestamp()
}

func (g *Game) Timeout() time.Time {
	_, ok := g.head.(*GameStep)
	if ok {
		// TODO: calculate game timeout based on the game timing rules
		return time.Now().Add(time.Hour * 24 * 365)
	}

	o := g.Confirmation()
	if o != nil {
		return o.Timeout()
	}

	a := g.Acceptance()
	if a != nil {
		return a.Timeout()
	}

	c := g.Challenge()
	if c != nil {
		return c.Timeout()
	}

	return time.Time{}
}

func (g *Game) Players() []*Player {
	pls := make(map[string]*Player)

	h := g.head

	for {
		if h == nil {
			break
		}

		p := h.Committer()
		pID := p.ID()
		if pID != "" {
			pls[pID] = p
		}

		h = h.Parent()
	}

	pp := make([]*Player, len(pls))
	for _, p := range pls {
		pp = append(pp, p)
	}

	return pp
}

func (g *Game) Merge(o *Game) error {
	if g.head.Hash() == "" {
		return errors.New("the base game does not have a hash for its head")
	}

	if o.head.Hash() == "" {
		return errors.New("the head game does not have a hash for its head")
	}

	originalHead := g.head

	h := g.head
	for {
		if h == nil {
			break
		}

		if h.Hash() == o.head.Hash() {
			// the head of the merge-head is in the base's history, so already merged
			return nil
		}

		h = h.Parent()
	}

	var future []Commit
	var foundOurHeadInHistory bool
	h = o.head
	for {
		if h == nil {
			break
		}

		if h.Hash() == g.head.Hash() {
			foundOurHeadInHistory = true
			break
		}

		future = append(future, h)

		h = h.Parent()
	}

	if foundOurHeadInHistory {
		var err error
	FUTURE:
		for i := len(future) - 1; i >= 0; i-- {
			c := future[i]
			switch c.(type) {
			case *Challenge:
				err = errors.New("found a Challenge in the forked history")
				break FUTURE

			case *ChallengeAcceptance:
				err = errors.New("found a Challenge Acceptance in the forked history")
				break FUTURE

			case *ChallengeConfirmation:
				a, ok := g.head.(*ChallengeAcceptance)
				if !ok {
					err = errors.New("found a Callenge Confirmation but base head is not a Challenge Acceptance")
					break FUTURE
				}

				x := c.(*ChallengeConfirmation)

				sig := make([]byte, len(x.signature))
				copy(sig, x.signature)

				y := &ChallengeConfirmation{
					timeout:    x.timeout,
					comment:    x.comment,
					acceptance: a,
					confirmer:  x.confirmer,
					timestamp:  x.timestamp,
					signature:  sig,
					hash:       x.hash,
				}
				err = y.Verify()
				if err != nil {
					err = errors.Wrap(err, "failed to verify cloned challenge confirmation")
					break FUTURE
				}

				g.head = y
				err = g.validate()
				if err != nil {
					err = errors.Wrap(err, "failed to merge confirmation")
					break FUTURE
				}

			case *GameStep:
				t := g.head.Type()
				if t != CommitTypeChallengeAcceptance && t != CommitTypeGameStep {
					err = errors.New("found a Game Step but the base head is not a Challenge Acceptance or Game Step")
					break FUTURE
				}

				x := c.(*GameStep)

				dat := make([]byte, len(x.data))
				copy(dat, x.data)

				sig := make([]byte, len(x.signature))
				copy(sig, x.signature)

				y := &GameStep{
					player:    x.player,
					data:      dat,
					parent:    g.head,
					timestamp: x.timestamp,
					signature: sig,
					hash:      x.hash,
				}
				err = y.Verify()
				if err != nil {
					err = errors.Wrap(err, "failed to verify cloned game step")
				}

				g.head = y
				err = g.validate()
				if err != nil {
					err = errors.Wrap(err, "failed to merge game step")
					break FUTURE
				}
			}
		}

		if err != nil {
			g.head = originalHead
			return errors.Wrap(err, "failed to merge other game simply")
		}
	} else {
		// TODO: support non-fast-forward merges...
		return errors.New("non-fast-forward merges are not supported")
	}

	return nil
}

func CreateGame(
	challenger *Player,
	exp time.Duration,
	c string,
) (*Game, error) {

	if challenger.ID() == "" {
		return nil, errors.New("challenger has an empty id")
	}

	if challenger.PrivateKey() == nil {
		return nil, errors.New("missing challenger private key")
	}

	now := time.Now()

	ch := NewChallenge()
	ch.timeout = now.Add(exp)
	ch.challenger = challenger
	ch.comment = c
	ch.timestamp = now

	err := ch.Sign()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create challenge")
	}

	return &Game{head: ch}, nil
}

func (g *Game) Accept(
	accepter *Player,
	exp time.Duration,
	c string,
) error {

	if g.Acceptance() != nil {
		return errors.New("game has already been accepted")
	}

	if g.Challenge() == nil {
		return errors.New("challenge has not been created yet")
	}

	if accepter.ID() == "" {
		return errors.New("accepter has an empty id")
	}

	if accepter.PrivateKey() == nil {
		return errors.New("missing accepter private key")
	}

	now := time.Now()

	ca := NewChallengeAcceptance()
	ca.timeout = now.Add(exp)
	ca.challenge = g.Challenge()
	ca.accepter = accepter
	ca.comment = c
	ca.timestamp = now

	err := ca.Sign()
	if err != nil {
		return errors.Wrap(err, "failed to create challenge acceptance")
	}

	g.head = ca

	return nil
}

func (g *Game) Confirm(
	confirmer *Player,
	exp time.Duration,
	c string,
) error {

	if g.Confirmation() != nil {
		return errors.New("game has already been confirmed")
	}

	if g.Acceptance() == nil {
		return errors.New("challange has not been accepted yet")
	}

	if confirmer.ID() == "" {
		return errors.New("confirmer has an empty id")
	}

	if confirmer.PrivateKey() == nil {
		return errors.New("missing confirmer private key")
	}

	if g.Challenge().Challenger() != confirmer {
		return errors.New("only the challenger may confirm a challenge")
	}

	now := time.Now()

	cc := NewChallengeConfirmation()
	cc.timeout = now.Add(exp)
	cc.acceptance = g.Acceptance()
	cc.confirmer = confirmer
	cc.comment = c
	cc.timestamp = now

	err := cc.Sign()
	if err != nil {
		return errors.Wrap(err, "failed to create challenge confirmation")
	}

	g.head = cc

	return nil
}

func (g *Game) Step(
	player *Player,
	data []byte,
) error {

	if g.Confirmation() == nil {
		return errors.New("hame has not been confirmed yet")
	}

	if player.ID() == "" {
		return errors.New("player has an empty id")
	}

	if player.PrivateKey() == nil {
		return errors.New("missing player private key")
	}

	d := make([]byte, len(data))
	n := copy(d, data)
	if n != len(data) {
		return errors.New("failed to copy all of the data to a new array")
	}

	gs := NewGameStep()
	gs.player = player
	gs.data = d
	gs.parent = g.head
	gs.timestamp = time.Now()

	err := gs.Sign()
	if err != nil {
		return errors.Wrap(err, "failed to create game step")
	}

	g.head = gs

	return nil
}

func (g *Game) validate() error {
	if g.Challenge() != nil && g.Confirmation() != nil {
		if g.Challenge().Challenger().ID() != g.Confirmation().Confirmer().ID() {
			return errors.New("the game was not confirmed by the challenger")
		}
	}

	return nil
}

func (g *Game) Publish(s *cachedshell.Shell) (string, error) {
	h, err := g.head.Publish(s)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish game head")
	}

	return h, nil
}

func GetGame(h string, s *cachedshell.Shell, players []*Player) (*Game, error) {
	var rcs []*rawCommit

	hPrime := h
	for {
		rc, err := getRawCommit(hPrime, s)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get raw commit")
		}

		rcs = append(rcs, rc)

		hPrime = rc.ParentHash

		if hPrime == "" {
			break
		}
	}

	for i, j := 0, len(rcs)-1; i < j; i, j = i+1, j-1 {
		rcs[i], rcs[j] = rcs[j], rcs[i]
	}

	ps := make(map[string]*Player)
	for _, p := range players {
		i := p.ID()
		if i != "" {
			ps[i] = p
		}
	}

	g := NewGame()

	for i, rc := range rcs {
		switch i {
		case 0:
			if rc.Type != CommitTypeChallenge {
				return nil, errors.Errorf("first commit was not a challenge: %+v", rc)
			}

			ic, err := getIpfsChallenge(rc.DataHash, s)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get challenge data")
			}

			c := NewChallenge()
			c.timeout = ic.Timeout.Time
			c.challenger = ps[rc.CommitterHash]
			c.comment = ic.Comment
			c.timestamp = rc.Timestamp
			c.signature = rc.Signature
			c.hash = rc.Hash

			err = c.Verify()
			if err != nil {
				return nil, errors.Wrap(err, "failed to load challenge")
			}

			g.head = c

		case 1:
			if rc.Type != CommitTypeChallengeAcceptance {
				return nil, errors.Errorf("second commit was not a challenge acceptance: %+v", rc)
			}

			ia, err := getIpfsChallengeAcceptance(rc.DataHash, s)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get challenge acceptance data")
			}

			a := NewChallengeAcceptance()
			a.timeout = ia.Timeout.Time
			a.challenge = g.Challenge()
			a.accepter = ps[rc.CommitterHash]
			a.comment = ia.Comment
			a.timestamp = rc.Timestamp
			a.signature = rc.Signature
			a.hash = rc.Hash

			err = a.Verify()
			if err != nil {
				return nil, errors.Wrap(err, "failed to load challenge acceptance")
			}

			g.head = a

		case 2:
			if rc.Type != CommitTypeChallengeConfirm {
				return nil, errors.Errorf("third commit was not a challenge confirmation: %+v", rc)
			}

			ic, err := getIpfsChallengeConfirmation(rc.DataHash, s)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get challenge confirmation data")
			}

			c := NewChallengeConfirmation()
			c.timeout = ic.Timeout.Time
			c.acceptance = g.Acceptance()
			c.confirmer = ps[rc.CommitterHash]
			c.comment = ic.Comment
			c.timestamp = rc.Timestamp
			c.signature = rc.Signature
			c.hash = rc.Hash

			err = c.Verify()
			if err != nil {
				return nil, errors.Wrap(err, "failed to load challenge confirmation")
			}

			g.head = c

		default:
			if rc.Type != CommitTypeGameStep {
				return nil, errors.Errorf("unexpected commit type found: %+v", rc)
			}

			ig, err := getIpfsGameStep(rc.DataHash, s)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get game step data")
			}

			gs := NewGameStep()
			gs.player = ps[rc.CommitterHash]
			gs.data = ig.Data
			gs.parent = g.head
			gs.timestamp = rc.Timestamp
			gs.signature = rc.Signature
			gs.hash = rc.Hash

			err = gs.Verify()
			if err != nil {
				return nil, errors.Wrap(err, "failed to load game step")
			}

			g.head = gs
		}
	}

	err := g.validate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load game")
	}

	return g, nil
}

type fileGame struct {
	Challenge    *fileChallenge
	Acceptance   *fileChallengeAcceptance
	Confirmation *fileChallengeConfirmation
	Steps        []*fileGameStep
}

func (g *Game) Write(out io.Writer) error {
	fg := &fileGame{}

	ch := g.Challenge()
	if ch != nil {
		fg.Challenge = &fileChallenge{
			Timeout:      IPGSTime{ch.Timeout()},
			Comment:      ch.Comment(),
			ChallengerID: ch.Challenger().ID(),
			Timestamp:    IPGSTime{ch.Timestamp()},
			Signature:    ch.Signature(),
			Hash:         ch.Hash(),
		}
	}

	ca := g.Acceptance()
	if ca != nil {
		fg.Acceptance = &fileChallengeAcceptance{
			Timeout:       IPGSTime{ca.Timeout()},
			Comment:       ca.Comment(),
			ChallengeHash: ch.Hash(),
			AccepterID:    ca.Accepter().ID(),
			Timestamp:     IPGSTime{ca.Timestamp()},
			Signature:     ca.Signature(),
			Hash:          ca.Hash(),
		}
	}

	cc := g.Confirmation()
	if cc != nil {
		fg.Confirmation = &fileChallengeConfirmation{
			Timeout:        IPGSTime{cc.Timeout()},
			Comment:        cc.Comment(),
			AcceptanceHash: ca.Hash(),
			ConfirmerID:    cc.Confirmer().ID(),
			Timestamp:      IPGSTime{cc.Timestamp()},
			Signature:      cc.Signature(),
			Hash:           cc.Hash(),
		}
	}

	gss := g.Steps()
	for i, gs := range gss {
		pHash := cc.Hash()
		if i > 0 {
			pHash = gss[i-1].Hash()
		}

		fg.Steps = append(
			fg.Steps,
			&fileGameStep{
				PlayerID:   gs.Player().ID(),
				Data:       gs.Data(),
				ParentHash: pHash,
				Timestamp:  IPGSTime{gs.Timestamp()},
				Signature:  gs.Signature(),
				Hash:       gs.Hash(),
			},
		)
	}

	err := json.NewEncoder(out).Encode(fg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal game to output")
	}

	return nil
}

func ReadGame(in io.Reader, players []*Player) (*Game, error) {
	var fg fileGame
	err := json.NewDecoder(in).Decode(&fg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal game from input")
	}

	ps := make(map[string]*Player)
	for _, p := range players {
		i := p.ID()
		if i != "" {
			ps[i] = p
		}
	}

	g := NewGame()

	if fg.Challenge != nil {
		c := NewChallenge()
		c.timeout = fg.Challenge.Timeout.Time
		c.challenger = ps[fg.Challenge.ChallengerID]
		c.comment = fg.Challenge.Comment
		c.timestamp = fg.Challenge.Timestamp.Time
		c.signature = fg.Challenge.Signature
		c.hash = fg.Challenge.Hash

		err := c.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "failed to load challenge")
		}

		g.head = c
	}

	if fg.Acceptance != nil {
		a := NewChallengeAcceptance()
		a.timeout = fg.Acceptance.Timeout.Time

		c := g.Challenge()
		if c.Hash() != fg.Acceptance.ChallengeHash {
			return nil, errors.New("challenge acceptance does not link back to the game's challenge")
		}

		a.challenge = c
		a.accepter = ps[fg.Acceptance.AccepterID]
		a.comment = fg.Acceptance.Comment
		a.timestamp = fg.Acceptance.Timestamp.Time
		a.signature = fg.Acceptance.Signature
		a.hash = fg.Acceptance.Hash

		err := a.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "failed to load challenge acceptance")
		}

		g.head = a
	}

	if fg.Confirmation != nil {
		c := NewChallengeConfirmation()
		c.timeout = fg.Confirmation.Timeout.Time

		a := g.Acceptance()
		if a.Hash() != fg.Confirmation.AcceptanceHash {
			return nil, errors.New("challenge confirmation does not link back to the game's acceptance")
		}

		c.acceptance = a
		c.confirmer = ps[fg.Confirmation.ConfirmerID]
		c.comment = fg.Confirmation.Comment
		c.timestamp = fg.Confirmation.Timestamp.Time
		c.signature = fg.Confirmation.Signature
		c.hash = fg.Confirmation.Hash

		err := c.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "failed to load challenge confirmation")
		}

		g.head = c
	}

	for _, fgs := range fg.Steps {
		gs := NewGameStep()
		gs.player = ps[fgs.PlayerID]
		gs.data = fgs.Data

		p := g.head
		if p.Hash() != fgs.ParentHash {
			return nil, errors.New("game step does not link back to the game's current head")
		}

		gs.parent = p
		gs.timestamp = fgs.Timestamp.Time
		gs.signature = fgs.Signature
		gs.hash = fgs.Hash

		err := gs.Verify()
		if err != nil {
			return nil, errors.Wrap(err, "failed to load game step")
		}

		g.head = gs
	}

	err = g.validate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load game")
	}

	return g, nil
}
