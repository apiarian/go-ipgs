package state

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/apiarian/go-ipgs/crypto"
)

func TestGameLifecycle(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	p := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), "player-p-pub-key-hash"),
		NewPrivateKey(priv),
	)

	now := time.Now()

	g, err := CreateGame(p, 5*time.Hour, "test game")
	fatalIfErr(t, "failed to create a game", err)

	c := g.Challenge()
	if c == nil {
		t.Fatal("game has no challenge")
	}

	if g.head != c {
		t.Fatal("the game's head is not the challenge")
	}

	tdiff := c.Timeout().Sub(now) - 5*time.Hour
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the game's timeout is not within 1 second of 5 hours from now")
	}

	tdiff = c.Timestamp().Sub(now)
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the game's timestamp is not within 1 second of now")
	}

	if c.Comment() != "test game" {
		t.Fatal("the comment is incorrect")
	}

	if !c.Challenger().Key().Equals(p.Key()) {
		t.Fatal("the challenger public key does not match player's")
	}

	if c.Type() != CommitTypeChallenge {
		t.Fatal("the challenge committ type is incorrect")
	}

	if c.ID() != fmt.Sprintf("player-p-pub-key-hash|%s", c.Timestamp().UTC().Format(time.RFC3339Nano)) {
		t.Fatal("the challenge ID is incorrect")
	}

	if g.ID() != c.ID() {
		t.Fatal("the game ID is not the challenge ID")
	}

	if c.Parent() != nil {
		t.Fatal("challenge parent is not nil")
	}

	if c.Hash() != "" {
		t.Fatal("challenge hash is not an empty string")
	}

	if len(c.Signature()) == 0 {
		t.Fatal("challenge has not been signed yet")
	}

	err = c.Verify()
	fatalIfErr(t, "failed to verify challenge", err)

	d, err := c.SignatureData()
	fatalIfErr(t, "failed to get challenge signature data", err)

	dPrime := []byte(fmt.Sprintf(
		"%s|%s|%s|none",
		c.ID(),
		c.Timeout().UTC().Format(time.RFC3339Nano),
		c.Comment(),
	))

	if !bytes.Equal(d, dPrime) {
		t.Fatal("challenge signature data does not meet expectations")
	}

	s := c.Signature()
	for i, _ := range s {
		s[i] = 0
	}

	if bytes.Equal(s, c.Signature()) {
		t.Fatal("succeeded in zeroing out the challenge signature")
	}

	// this would normally be done by the publish method
	c.hash = "challenge-hash"
	if c.Hash() != "challenge-hash" {
		t.Fatal("the challenge still has no hash")
	}

	now = time.Now()

	err = g.Accept(p, 5*time.Hour, "test acceptance")
	fatalIfErr(t, "failed to accept game", err)

	a := g.Acceptance()
	if a == nil {
		t.Fatal("game has no acceptance")
	}

	if g.head != a {
		t.Fatal("the game's head is not the acceptnce")
	}

	tdiff = a.Timeout().Sub(now) - 5*time.Hour
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the acceptance's timeout is not within 1 second of 5 hours from now")
	}

	tdiff = a.Timestamp().Sub(now)
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the acceptance's timestamp is not within 1 second of now")
	}

	if a.Comment() != "test acceptance" {
		t.Fatal("the acceptance's comment is incorrect")
	}

	if !a.Accepter().Key().Equals(p.Key()) {
		t.Fatal("the accepter public key does not match the player's")
	}

	if a.Type() != CommitTypeChallengeAcceptance {
		t.Fatal("the acceptance commit type is incorrect")
	}

	if a.ID() != fmt.Sprintf("player-p-pub-key-hash|%s|player-p-pub-key-hash", a.Challenge().Timestamp().UTC().Format(time.RFC3339Nano)) {
		t.Fatal("the acceptance ID is incorrect")
	}

	if g.ID() != a.ID() {
		t.Fatal("the game id is not the acceptance id")
	}

	if a.Parent() != c {
		t.Fatal("the acceptance parent is not the challenge")
	}

	if a.Challenge() != c {
		t.Fatal("the acceptance challenge is not the challenge")
	}

	if a.Hash() != "" {
		t.Fatal("the acceptance hash is not an empty string")
	}

	if len(a.Signature()) == 0 {
		t.Fatal("the acceptance has not been signed yet")
	}

	err = a.Verify()
	fatalIfErr(t, "failed to verify acceptance", err)

	d, err = a.SignatureData()
	fatalIfErr(t, "failed to get acceptance signature data", err)

	dPrime = []byte(fmt.Sprintf(
		"%s|%s|%s|%s",
		a.ID(),
		a.Timeout().UTC().Format(time.RFC3339Nano),
		a.Comment(),
		c.Hash(),
	))

	if !bytes.Equal(d, dPrime) {
		t.Fatal("acceptance signature data does not meet expectations")
	}

	s = a.Signature()
	for i, _ := range s {
		s[i] = 0
	}

	if bytes.Equal(s, a.Signature()) {
		t.Fatal("succeeded in zeroing out acceptance signature")
	}

	// this would normally be done by the publish method
	a.hash = "acceptance-hash"
	if a.Hash() != "acceptance-hash" {
		t.Fatal("the acceptance still has no hash")
	}

	now = time.Now()

	err = g.Confirm(p, 5*time.Hour, "test confirmation")
	fatalIfErr(t, "failed to confirm game", err)

	o := g.Confirmation()
	if o == nil {
		t.Fatal("the game has no confirmation")
	}

	if g.head != o {
		t.Fatal("game has no confirmation")
	}

	tdiff = o.Timeout().Sub(now) - 5*time.Hour
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the confirmation's timeout is not within 1 second of 5 hours from now")
	}

	tdiff = o.Timestamp().Sub(now)
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("The confirmations's timestamp is not within 1 second of now")
	}

	if o.Comment() != "test confirmation" {
		t.Fatal("the confirmation's comment is incorrect")
	}

	if !o.Confirmer().Key().Equals(p.Key()) {
		t.Fatal("the confirmer's public key does not match the player's")
	}

	if o.Type() != CommitTypeChallengeConfirm {
		t.Fatal("the confirmation commit type is incorrect")
	}

	if o.ID() != fmt.Sprintf("player-p-pub-key-hash|%s|player-p-pub-key-hash", o.Acceptance().Challenge().Timestamp().UTC().Format(time.RFC3339Nano)) {
		t.Fatal("the confirmation ID is incorrect")
	}

	if g.ID() != a.ID() {
		t.Fatal("the game ID is no longer the acceptance ID")
	}

	if o.Parent() != a {
		t.Fatal("the confirmation partent is not the acceptance")
	}

	if o.Acceptance() != a {
		t.Fatal("the confirmation acceptance is not the acceptance")
	}

	if o.Hash() != "" {
		t.Fatal("the confirmation hash is not an empty string")
	}

	if len(o.Signature()) == 0 {
		t.Fatal("the confirmation has not been signed yet")
	}

	err = o.Verify()
	fatalIfErr(t, "failed to verify confirmation", err)

	d, err = o.SignatureData()
	fatalIfErr(t, "failed to get confirmation signature data", err)

	dPrime = []byte(fmt.Sprintf(
		"%s|%s|%s|%s",
		o.ID(),
		o.Timeout().UTC().Format(time.RFC3339Nano),
		o.Comment(),
		a.Hash(),
	))

	if !bytes.Equal(d, dPrime) {
		t.Fatal("confirmation signature data does not meet expectations")
	}

	s = o.Signature()
	for i, _ := range s {
		s[i] = 0
	}

	if bytes.Equal(s, o.Signature()) {
		t.Fatal("succeeded in zeroing out confirmation signature")
	}

	// this would normally be done by the publish method
	o.hash = "confirmation-hash"
	if o.Hash() != "confirmation-hash" {
		t.Fatal("the confirmation still has no hash")
	}

	now = time.Now()

	err = g.Step(p, []byte("step-1"))
	fatalIfErr(t, "failed to make the first step", err)

	if len(g.Steps()) != 1 {
		t.Fatal("there is not exactly one game step")
	}

	gs1 := g.Steps()[0]

	if gs1 == nil {
		t.Fatal("game has no first step")
	}

	if g.head != gs1 {
		t.Fatal("the game head is not the first step")
	}

	tdiff = gs1.Timestamp().Sub(now)
	if -1*time.Second > tdiff || tdiff > 1*time.Second {
		t.Fatal("the first step's timestamp is not within 1 second of now")
	}

	if !bytes.Equal(gs1.Data(), []byte("step-1")) {
		t.Fatal("the first step's data is incorrect")
	}

	d = gs1.Data()
	for i, _ := range d {
		d[i] = 0
	}

	if bytes.Equal(d, gs1.Data()) {
		t.Fatal("succeeded in zeroing out step 1's data")
	}

	if !gs1.Player().Key().Equals(p.Key()) {
		t.Fatal("step player's public key does not match the actual player's")
	}

	if gs1.Type() != CommitTypeGameStep {
		t.Fatal("the step's commit type is incorrect")
	}

	if gs1.ID() != a.ID() {
		t.Fatal("the step's ID is not the same as the acceptance ID")
	}

	if gs1.ID() != g.ID() {
		t.Fatal("the game's id is not the same as the game step's ID")
	}

	if gs1.Parent() != o {
		t.Fatal("the step's parent is not the confirmation")
	}

	if gs1.Hash() != "" {
		t.Fatal("the step's hash is not an empty string")
	}

	if len(gs1.Signature()) == 0 {
		t.Fatal("the step has not been signed yet")
	}

	err = gs1.Verify()
	fatalIfErr(t, "failed to verify first step", err)

	d, err = gs1.SignatureData()
	fatalIfErr(t, "failed to get step's signature data", err)

	buf := bytes.NewBufferString(fmt.Sprintf(
		"%s|%s|",
		gs1.ID(),
		gs1.Timestamp().UTC().Format(time.RFC3339Nano),
	))
	buf.Write(gs1.Data())
	buf.WriteString(fmt.Sprintf("|%s", o.Hash()))
	dPrime = buf.Bytes()

	if !bytes.Equal(d, dPrime) {
		t.Fatal("game step signature data does not meet expectations")
	}

	s = gs1.Signature()
	for i, _ := range s {
		s[i] = 0
	}

	if bytes.Equal(s, gs1.Signature()) {
		t.Fatal("succeeded in zeroing out game step's signature")
	}

	// this would normally be done by the publish method
	gs1.hash = "game-step-1-hash"
	if gs1.Hash() != "game-step-1-hash" {
		t.Fatal("the game step's hash is wrong")
	}

	err = g.Step(p, []byte("step-2"))
	fatalIfErr(t, "failed to make second step", err)

	if len(g.Steps()) != 2 {
		t.Fatal("there is not exactly two game steps")
	}

	gs2 := g.Steps()[1]

	if gs2 == nil {
		t.Fatal("game has no second step")
	}

	if g.head != gs2 {
		t.Fatal("the game's head is not the second step")
	}

	if gs1.Timestamp().Equal(gs2.Timestamp()) {
		t.Fatal("the first and second steps seem to have the same timestamp")
	}

	if !gs1.Timestamp().Before(gs2.Timestamp()) {
		t.Fatal("the second timestamp seems to preceed the first")
	}

	if gs2.Parent() != gs1 {
		t.Fatal("the second step's parent is not the first step")
	}

	if gs2.ID() != a.ID() {
		t.Fatal("the second step's ID is not the same as the acceptance ID")
	}

	if gs2.ID() != g.ID() {
		t.Fatal("the second step's ID is not the same as the game's ID")
	}
}

func (g *Game) mockPublish() {
	if g.head == nil {
		return
	}

	if g.head.Hash() != "" {
		return
	}

	ch, ok := g.head.(*Challenge)
	if ok {
		ch.hash = base64.StdEncoding.EncodeToString(ch.Signature())
		return
	}

	ca, ok := g.head.(*ChallengeAcceptance)
	if ok {
		ca.hash = base64.StdEncoding.EncodeToString(ca.Signature())
		return
	}

	cc, ok := g.head.(*ChallengeConfirmation)
	if ok {
		cc.hash = base64.StdEncoding.EncodeToString(cc.Signature())
		return
	}

	gs, ok := g.head.(*GameStep)
	if ok {
		gs.hash = base64.StdEncoding.EncodeToString(gs.Signature())
		return
	}
}

func TestGameMerge(t *testing.T) {
	var pls []*Player
	for i := 0; i < 3; i++ {
		priv, err := crypto.NewPrivateKey()
		fatalIfErr(t, "failed to create private key", err)

		pls = append(pls, NewPlayer(
			NewPublicKey(priv.GetPublicKey(), fmt.Sprintf("player-%d-public-key", i)),
			NewPrivateKey(priv),
		))
	}

	timeout := 5 * time.Hour

	g, err := CreateGame(pls[0], timeout, "test game")
	fatalIfErr(t, "failed to create a game", err)
	g.mockPublish()

	t.Logf("g.Challenge() = %+v\n", g.Challenge())

	b := &bytes.Buffer{}
	err = g.Write(b)
	fatalIfErr(t, "failed to write the game", err)
	b2 := &bytes.Buffer{}
	err = g.Write(b2)
	fatalIfErr(t, "failed to write the game again", err)

	o, err := ReadGame(b, pls)
	fatalIfErr(t, "failed to read the game", err)

	t.Logf("o.Challenge() = %+v\n", o.Challenge())

	if g.Challenge().Hash() != o.Challenge().Hash() {
		t.Fatal("the two challenges are not the same")
	}

	err = o.Accept(pls[1], timeout, "lets go")
	fatalIfErr(t, "failed to accept the game", err)
	o.mockPublish()

	// we should be able to merge games once we have a common challenge

	err = g.Merge(o)
	fatalIfErr(t, "failed to merge the accepted game", err)

	if g.Acceptance().Hash() != o.Acceptance().Hash() {
		t.Fatal("the two acceptances are not the same")
	}

	if g.Acceptance() == o.Acceptance() {
		t.Fatal("the two acceptances are the same object, not clones")
	}

	o2, err := ReadGame(b2, pls)
	fatalIfErr(t, "failed to read the game again", err)

	t.Logf("o2.Challenge() = %+v\n", o2.Challenge())

	if g.Challenge().Hash() != o2.Challenge().Hash() {
		t.Fatal("the other challenge is not the same as the original")
	}

	err = o2.Accept(pls[2], timeout, "lets go too")
	fatalIfErr(t, "failed to accept the game as another player", err)
	o2.mockPublish()

	// we should not be able to merge games that would end up with multiple
	// acceptances

	h := g.head
	err = g.Merge(o2)
	t.Logf("o2 merge err = %+v\n", err)
	if err == nil {
		t.Fatal("managed to merge an alternate accepted game")
	}
	if h != g.head {
		t.Fatal("the game head commit changed despite running into a merge error")
	}

	err = g.Confirm(pls[0], timeout, "make it so")
	fatalIfErr(t, "failed to confirm the game", err)
	g.mockPublish()

	// we should be able to merge something that has already been merged without
	// trouble

	h = g.head
	err = g.Merge(o)
	fatalIfErr(t, "failed to merge an already merged game", err)
	if h != g.head {
		t.Fatal("merging an already merged game should have been a noop")
	}

	err = g.Step(pls[0], []byte("move 1"))
	fatalIfErr(t, "failed to make the first move", err)
	g.mockPublish()

	// we should be able to merge multiple steps forward

	err = o.Merge(g)
	fatalIfErr(t, "failed to merge a couple of steps forward", err)

	if g.head.Hash() != o.head.Hash() {
		t.Fatal("the merged games don't have the same head")
	}

	err = o.Step(pls[1], []byte("move 2"))
	fatalIfErr(t, "failed to make the second move", err)
	o.mockPublish()

	b.Reset()
	err = o.Write(b)
	fatalIfErr(t, "failed to write the other game", err)
	o3, err := ReadGame(b, pls)
	fatalIfErr(t, "failed to read the other game", err)

	// we should be able to merge the game with another game step

	err = g.Merge(o3)
	fatalIfErr(t, "failed to merge the other game after it was loaded again", err)

	if g.head.Hash() != o.head.Hash() {
		t.Fatal("the merged games do not have the same head after two moves")
	}

	x, err := CreateGame(pls[0], timeout, "totally different game")
	fatalIfErr(t, "failed to create a totally separate game", err)
	x.mockPublish()

	err = g.Merge(x)
	t.Logf("separate merge err = %+v\n", err)
	if err == nil {
		t.Fatal("somehow merged totally separate games")
	}
}

func TestGamePlayerPermissions(t *testing.T) {
	var pls []*Player
	for i := 0; i < 3; i++ {
		priv, err := crypto.NewPrivateKey()
		fatalIfErr(t, "failed to create private key", err)

		pls = append(pls, NewPlayer(
			NewPublicKey(priv.GetPublicKey(), fmt.Sprintf("player-%d-public-key", i)),
			NewPrivateKey(priv),
		))
	}

	g, err := CreateGame(pls[0], 5*time.Hour, "test game")
	fatalIfErr(t, "failed to create a game", err)
	g.mockPublish()

	err = g.Accept(pls[1], 5*time.Hour, "lets go")
	fatalIfErr(t, "failed to accept the game", err)
	g.mockPublish()

	err = g.Confirm(pls[2], 5*time.Hour, "butting in")
	if err == nil {
		t.Fatal("succeeded in confirming a game that we don't own")
	}

	err = g.Confirm(pls[1], 5*time.Hour, "self-confirming")
	if err == nil {
		t.Fatal("succeded in confirming a game that we just accepted")
	}

	err = g.Confirm(pls[0], 5*time.Hour, "real confirmation")
	fatalIfErr(t, "failed to confirm the game", err)
	g.mockPublish()

	err = g.Step(pls[2], []byte("barging-in"))
	fatalIfErr(t, "failed to barge into a game", err)
}

func checkGameEquivalence(t *testing.T, g1, g2 *Game) {
	if g1 == nil && g2 != nil {
		t.Fatal("g1 is nil but g2 is not")
	}

	if g1 != nil && g2 == nil {
		t.Fatal("g1 is not nil but g2 is")
	}

	c1 := g1.Challenge()
	c2 := g2.Challenge()

	if c1 == nil && c2 != nil {
		t.Fatal("g1 has no challenge but g2 does")
	}
	if c1 != nil && c2 == nil {
		t.Fatal("g1 has a challenge but g2 does not")
	}

	if c1 != nil && c2 != nil {
		if !c1.Timeout().Equal(c2.Timeout()) {
			t.Fatal("challenge timeouts are not equal")
		}

		if c1.Comment() != c2.Comment() {
			t.Fatal("challenge comments are not equal")
		}

		if !c1.Challenger().Key().Equals(c2.Challenger().Key()) {
			t.Fatal("challenge challengers don't have the same public key")
		}

		if !c1.Timestamp().Equal(c2.Timestamp()) {
			t.Fatal("challenge timestamps are not equal")
		}

		if !bytes.Equal(c1.Signature(), c2.Signature()) {
			t.Fatal("challenge signatures are not equal")
		}

		if c1.Hash() != c2.Hash() {
			t.Fatal("challenge hashes are not equal")
		}
	}

	ca1 := g1.Acceptance()
	ca2 := g2.Acceptance()

	if ca1 == nil && ca2 != nil {
		t.Fatal("g1 has no acceptance but g2 does")
	}
	if ca1 != nil && ca2 == nil {
		t.Fatal("g1 has an acceptance but g2 does not")
	}

	if ca1 != nil && ca2 != nil {
		if !ca1.Timeout().Equal(ca2.Timeout()) {
			t.Fatal("acceptance timeouts are not equal")
		}

		if ca1.Comment() != ca2.Comment() {
			t.Fatal("acceptance comments are not equal")
		}

		if !bytes.Equal(ca1.Challenge().Signature(), ca2.Challenge().Signature()) {
			t.Fatal("acceptance challenge signatures are not equal")
		}

		if !ca1.Accepter().Key().Equals(ca2.Accepter().Key()) {
			t.Fatal("acceptance accepters don't have the same public key")
		}

		if !ca1.Timestamp().Equal(ca2.Timestamp()) {
			t.Fatal("acceptance timestamps are not equal")
		}

		if !bytes.Equal(ca1.Signature(), ca2.Signature()) {
			t.Fatal("acceptance signatures are not equal")
		}

		if ca1.Hash() != ca2.Hash() {
			t.Fatal("acceptance hashes are not equal")
		}
	}

	cc1 := g1.Confirmation()
	cc2 := g2.Confirmation()

	if cc1 == nil && cc2 != nil {
		t.Fatal("g1 has no confirmation but g2 does")
	}
	if cc1 != nil && cc2 == nil {
		t.Fatal("g1 has a confirmation but g2 does not")
	}

	if cc1 != nil && cc2 != nil {
		if !cc1.Timeout().Equal(cc2.Timeout()) {
			t.Fatal("confirmation timeouts are not equal")
		}

		if cc1.Comment() != cc2.Comment() {
			t.Fatal("confirmation comments are not equal")
		}

		if !bytes.Equal(cc1.Acceptance().Signature(), cc2.Acceptance().Signature()) {
			t.Fatal("confirmation acceptance signatures are not equal")
		}

		if !cc1.Confirmer().Key().Equals(cc2.Confirmer().Key()) {
			t.Fatal("confirmation confirmers don't have the same public key")
		}

		if !cc1.Timestamp().Equal(cc2.Timestamp()) {
			t.Fatal("confirmation timestamps are not equal")
		}

		if !bytes.Equal(cc1.Signature(), cc2.Signature()) {
			t.Fatal("confirmation signatures are not equal")
		}

		if cc1.Hash() != cc2.Hash() {
			t.Fatal("confirmation hashes are not equal")
		}
	}

	gss1 := g1.Steps()
	gss2 := g2.Steps()

	if len(gss1) != len(gss2) {
		t.Fatal("g1 and g2 do not have the same number of steps")
	}

	for i, gs1 := range gss1 {
		gs2 := gss2[i]

		if !gs1.Player().Key().Equals(gs2.Player().Key()) {
			t.Fatalf("game step %d player public keys are not equal", i)
		}

		if !bytes.Equal(gs1.Data(), gs2.Data()) {
			t.Fatalf("game step %d datae are not equal", i)
		}

		if !bytes.Equal(gs1.Parent().Signature(), gs2.Parent().Signature()) {
			t.Fatalf("game step %d parent commit signatures are not equal", i)
		}

		if !gs1.Timestamp().Equal(gs2.Timestamp()) {
			t.Fatalf("game step %d timestamps are not equal", i)
		}

		if !bytes.Equal(gs1.Signature(), gs2.Signature()) {
			t.Fatalf("game step %d signatures are not equal", i)
		}

		if gs1.Hash() != gs2.Hash() {
			t.Fatalf("game step %d hashes are not equal", i)
		}
	}
}

func TestGameReadWrite(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "create private key", err)

	p := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), "player-p-hash"),
		NewPrivateKey(priv),
	)

	t.Logf("player p: %+v", p)

	pPrime := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), "player-p-hash"),
		nil,
	)

	t.Logf("player p': %+v", pPrime)

	g, err := CreateGame(p, 5*time.Hour, "simple game")
	fatalIfErr(t, "failed to create game", err)

	// pretend that we published the challenge. don't do this anywhere else
	g.head.(*Challenge).hash = "challenge-hash"

	t.Logf("new game: %+v head: %+v", g, g.head)

	err = g.Accept(p, 5*time.Hour, "lets go")
	fatalIfErr(t, "failed to accept game", err)

	// pretend we published the acceptance. don't do this anywhere else
	g.head.(*ChallengeAcceptance).hash = "acceptance-hash"

	t.Logf("accepted game: %+v head: %+v", g, g.head)

	err = g.Confirm(p, 5*time.Hour, "ok lets go")
	fatalIfErr(t, "failed to confirm game", err)

	// pretend we published the confirmation. don't do this anywhere else
	g.head.(*ChallengeConfirmation).hash = "confirmation-hash"

	t.Logf("confirmed game: %+v head: %+v", g, g.head)

	err = g.Step(p, []byte("step 1"))
	fatalIfErr(t, "failed to make step 1", err)

	// pretend we published the step. don't do this anywhere else
	g.head.(*GameStep).hash = "step-1-hash"

	t.Logf("one step: %+v head: %+v", g, g.head)

	err = g.Step(p, []byte("step 2"))
	fatalIfErr(t, "failed to make step 2", err)

	// pretend we published the step. don't do this anywhere else
	g.head.(*GameStep).hash = "step-2-hash"

	t.Logf("second step: %+v head: %+v", g, g.head)

	b := &bytes.Buffer{}
	err = g.Write(b)
	fatalIfErr(t, "failed to write game to buffer", err)

	t.Logf("written game: %s", b.String())

	// using the non-private-key-carrying player
	l, err := ReadGame(b, []*Player{pPrime})
	fatalIfErr(t, "failed to read game from buffer", err)

	t.Logf("loaded game: %+v head: %+v", l, l.head)

	checkGameEquivalence(t, g, l)
}

func TestGamePublishGet(t *testing.T) {
	s, err := newShellForNode(0)
	fatalIfErr(t, "failed to get a shell for node 0", err)

	sPrime, err := newShellForNode(1)
	fatalIfErr(t, "failed to get a shell for node 1", err)

	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	p := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), ""),
		NewPrivateKey(priv),
	)

	_, err = p.Publish(s, p)
	fatalIfErr(t, "failed to publish player", err)

	pPrime := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), p.Key().Hash()),
		nil,
	)

	g, err := CreateGame(p, 5*time.Hour, "test game")
	fatalIfErr(t, "failed to create game", err)

	h, err := g.Publish(s)
	fatalIfErr(t, "failed to publish game", err)

	t.Log("game hash:", h)

	l, err := GetGame(h, sPrime, []*Player{pPrime})
	fatalIfErr(t, "failed to load the game at node 1", err)

	checkGameEquivalence(t, g, l)

	err = g.Accept(p, 5*time.Hour, "lets go")
	fatalIfErr(t, "failed to accept the game", err)

	h, err = g.Publish(s)
	fatalIfErr(t, "failed to publish accepted game", err)

	t.Log("accepted game hash:", h)

	l, err = GetGame(h, sPrime, []*Player{pPrime})
	fatalIfErr(t, "failed to load accepted game at node 1", err)

	checkGameEquivalence(t, g, l)

	err = g.Confirm(p, 5*time.Hour, "ok")
	fatalIfErr(t, "failed to confirm game", err)

	h, err = g.Publish(s)
	fatalIfErr(t, "failed to publish confirmed game", err)

	t.Log("confirmed game hash:", h)

	l, err = GetGame(h, sPrime, []*Player{pPrime})
	fatalIfErr(t, "failed to load confirmed game at node 1", err)

	checkGameEquivalence(t, g, l)

	err = g.Step(p, []byte("step 1"))
	fatalIfErr(t, "failed to step game", err)

	h, err = g.Publish(s)
	fatalIfErr(t, "failed to publish stepped game", err)

	t.Log("stepped game hash:", h)

	l, err = GetGame(h, sPrime, []*Player{pPrime})
	fatalIfErr(t, "failed to load stepped game at node 1", err)

	checkGameEquivalence(t, g, l)

	err = g.Step(p, []byte("step 2"))
	fatalIfErr(t, "failed to step game again", err)

	h, err = g.Publish(s)
	fatalIfErr(t, "failed to publish stepped game again", err)

	t.Log("double stepped game hash:", h)

	l, err = GetGame(h, sPrime, []*Player{pPrime})
	fatalIfErr(t, "failed to load double stepped game at node 1", err)

	checkGameEquivalence(t, g, l)

	l, err = GetGame(h, s, []*Player{p})
	fatalIfErr(t, "failed to load game at node 0 (the originator)", err)

	checkGameEquivalence(t, g, l)
}
