package state

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/apiarian/go-ipgs/util"
	"github.com/pkg/errors"
	"github.com/whyrusleeping/iptb/util"
)

func fatalIfErr(t *testing.T, msg string, err error) {
	if err != nil {
		t.Fatalf("%s: %+v\n", msg, err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	ipfsDir, err := ioutil.TempDir("", "ipgs-test-state-iptb-root")
	util.FatalIfErr("failed to create temporary ipfs directory", err)
	log.Println("temporary ipfs directory:", ipfsDir)

	err = os.Setenv("IPTB_ROOT", ipfsDir)
	util.FatalIfErr("failed to set IPTB_ROOT to temporary ipfsdir", err)

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	ps := 15000 + (rnd.Int()%500)*10
	log.Println("iptb port start:", ps)

	cfg := &iptbutil.InitCfg{
		Count:     2,
		Force:     true,
		Bootstrap: "star",
		PortStart: ps,
		Mdns:      false,
		Utp:       false,
		Override:  "",
		NodeType:  "",
	}
	err = iptbutil.IpfsInit(cfg)
	util.FatalIfErr("failed to initialize iptb", err)

	nodes, err := iptbutil.LoadNodes()
	util.FatalIfErr("failed load nodes", err)
	defer iptbutil.IpfsKillAll(nodes)

	err = iptbutil.IpfsStart(nodes, true)
	if err != nil {
		for i, n := range nodes {
			killerr := n.Kill()
			if killerr != nil {
				log.Println("failed to kill node", i, ":", killerr)
			} else {
				log.Println("killed node", i)
			}
		}
		util.FatalIfErr("failed to start nodes", err)
	}

	r := m.Run()

	err = iptbutil.IpfsKillAll(nodes)
	if err != nil {
		log.Print("error killing nodes:", err)
	}

	os.RemoveAll(ipfsDir)

	os.Exit(r)
}

func newShellForNode(n int) (*cachedshell.Shell, error) {
	node, err := iptbutil.LoadNodeN(n)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node")
	}

	addr, err := node.APIAddr()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node API address")
	}

	s := cachedshell.NewShell(addr, cache.NewCache())
	if !s.IsUp() {
		return nil, errors.New("ipfs node does not seem to be up")
	}

	return s, nil
}

func TestStateReadWrite(t *testing.T) {
	nodeDir, err := ioutil.TempDir("", "ipgs-test-state-read-write")
	fatalIfErr(t, "create temporary nodeDir", err)
	t.Log("temporary directory:", nodeDir)

	s := NewState()

	s.LastUpdated = time.Now()

	pk, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	pkFile, err := os.Create(filepath.Join(nodeDir, PrivateKeyFileName))
	fatalIfErr(t, "failed to create private key file", err)
	defer pkFile.Close()

	err = pk.Write(pkFile)
	fatalIfErr(t, "failed to write private key to file", err)

	o := NewPlayer(
		NewPublicKey(pk.GetPublicKey(), "owner-hash"),
		NewPrivateKey(pk),
	)
	o.Timestamp = time.Now()
	o.Name = "owner"
	o.Flags["something"] = 1
	o.Flags["shiny"] = 1000
	o.Nodes = append(o.Nodes, "node1")
	s.Owner = o

	for i := 0; i < 4; i++ {
		pk, err := crypto.NewPrivateKey()
		fatalIfErr(t, "failed to create provate key for other player", err)

		p := NewPlayer(
			NewPublicKey(pk.GetPublicKey(), fmt.Sprintf("player-hash-%d", i)),
			nil,
		)
		p.Timestamp = time.Now()
		p.Name = fmt.Sprintf("player%v", i)
		p.Flags["cool-factor"] = i
		p.Nodes = append(o.Nodes, fmt.Sprintf("player-node-%v", i))

		s.Players = append(s.Players, p)
	}

	i1, err := s.CreateGame(5*time.Hour, "test game")
	fatalIfErr(t, "failed to create test game", err)

	i2, err := s.CreateGame(5*time.Hour, "test game 2")
	fatalIfErr(t, "failed to create a second test game", err)

	s.games[i2].head.(*Challenge).hash = "pretend-challenge-hash"

	i2, err = s.AcceptGame(i2, 5*time.Hour, "test acceptance")
	fatalIfErr(t, "failed to accept the second game", err)

	if len(s.games) != 3 {
		t.Fatal("don't have 3 games")
	}

	err = s.Write(nodeDir)
	fatalIfErr(t, "failed to write state to directory", err)

	l := NewState()
	err = l.Read(nodeDir)
	fatalIfErr(t, "failed to read state from directory", err)

	if !s.LastUpdated.Equal(l.LastUpdated) {
		t.Fatal("loaded timestamp does not match original")
	}

	if s.Owner.Name != l.Owner.Name {
		t.Fatal("loaded owner name does not match original")
	}

	// we expect the player order to be preserved
	for i, p := range s.Players {
		if l.Players[i].Name != p.Name {
			t.Fatal("loaded player does not match original")
		}
	}

	if len(l.games) != 3 {
		t.Fatal("did not load 3 games")
	}

	if l.Game(i1).ID() != s.Game(i1).ID() {
		t.Fatal("the first game ids do not match")
	}

	if l.Game(i2).ID() != s.Game(i2).ID() {
		t.Fatal("the second game ids do not match")
	}

	os.RemoveAll(nodeDir)
}

func TestStatePublishGet(t *testing.T) {
	s, err := newShellForNode(0)
	fatalIfErr(t, "failed to get a shell for node 0", err)

	sPrime, err := newShellForNode(1)
	fatalIfErr(t, "failed to get a shell for node 1", err)

	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	st := NewState()
	st.LastUpdated = time.Now()

	o := NewPlayer(NewPublicKey(priv.GetPublicKey(), ""), NewPrivateKey(priv))
	o.Timestamp = time.Now()
	o.Name = "owner"
	o.Flags["something"] = 1
	o.Nodes = append(o.Nodes, "node1")
	st.Owner = o

	for i := 0; i < 4; i++ {
		pk, err := crypto.NewPrivateKey()
		fatalIfErr(t, "failed to create private key for player", err)

		p := NewPlayer(NewPublicKey(pk.GetPublicKey(), ""), nil)
		p.Timestamp = time.Now()
		p.Name = fmt.Sprintf("player%v", i)
		p.Flags["player-flag"] = i
		p.Nodes = append(p.Nodes, fmt.Sprintf("node-%v", i))

		st.Players = append(st.Players, p)
	}

	t.Logf("state: %+v\n", st)

	h, err := st.Publish(s)
	fatalIfErr(t, "failed to publish state", err)

	i1, err := st.CreateGame(5*time.Hour, "test game 1")
	fatalIfErr(t, "failed to create test game 1", err)

	i2, err := st.CreateGame(5*time.Hour, "test game 2")
	fatalIfErr(t, "failed to create test game 2", err)

	h, err = st.Publish(s)
	fatalIfErr(t, "failed to publish state with a pair of challenges", err)

	i2, err = st.AcceptGame(i2, 5*time.Hour, "accept 2")
	fatalIfErr(t, "failed to accept the second test game", err)

	h, err = st.Publish(s)
	fatalIfErr(t, "failed to publish state with a challenge and acceptance", err)

	t.Log("hash:", h)

	if h == "" {
		t.Fatal("the hash should not be an empty string")
	}

	obj, err := s.ObjectGet(h)
	fatalIfErr(t, "failed to get state object", err)

	t.Logf("state object: %+v\n", obj)

	l := NewState()
	err = l.Get(h, sPrime)
	fatalIfErr(t, "failed to get state from different node", err)

	t.Logf("loaded: %+v\n", l)

	if !st.LastUpdated.Equal(l.LastUpdated) {
		t.Fatal("loaded timestamp does not match original")
	}

	if st.Owner.Name != l.Owner.Name {
		t.Fatal("loaded owner name does not match original")
	}

	// we do not expect player order to be preserved across IPFS
	for _, p := range st.Players {
		var found int
		for _, pl := range l.Players {
			if p.Name == pl.Name {
				found++
			}
		}
		if found != 1 {
			t.Fatal("did not find exactly one match for the player")
		}
	}

	if len(l.games) != 2 {
		t.Fatal("did not load 2 games")
	}

	if l.Game(i1).ID() != st.Game(i1).ID() {
		t.Fatal("the first game ids do not match")
	}

	if l.Game(i2).ID() != st.Game(i2).ID() {
		t.Fatal("the second game ids to not match")
	}
}

func TestStateCommitFind(t *testing.T) {
	sh0, err := newShellForNode(0)
	fatalIfErr(t, "failed to get a shell for node 0", err)

	initIPNS, err := sh0.ResolveFresh("")
	fatalIfErr(t, "failed to resolve initial IPNS", err)
	t.Log("initial IPNS:", initIPNS)

	shInfo, err := sh0.ID()
	fatalIfErr(t, "failed to get id for node 0", err)
	sh0ID := shInfo.ID

	sh0Prime, err := newShellForNode(0)
	fatalIfErr(t, "failed to get a secondary shell for node 0", err)

	sh1, err := newShellForNode(1)
	fatalIfErr(t, "failed to get a shell for node 1", err)

	nodeDir, err := ioutil.TempDir("", "ipgs-test-state-commit-find")
	fatalIfErr(t, "create temporary nodeDir", err)
	t.Log("temporary directory:", nodeDir)

	s := NewState()

	s.LastUpdated = time.Now()

	pk, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	pkFile, err := os.Create(filepath.Join(nodeDir, PrivateKeyFileName))
	fatalIfErr(t, "failed to create private key file", err)
	defer pkFile.Close()

	err = pk.Write(pkFile)
	fatalIfErr(t, "failed to write private key to file", err)
	pkFile.Close()

	o := NewPlayer(
		NewPublicKey(pk.GetPublicKey(), ""),
		NewPrivateKey(pk),
	)
	o.Timestamp = time.Now()
	o.Name = "owner"
	o.Nodes = append(o.Nodes, sh0ID)
	s.Owner = o

	t.Logf("initial state: %+v\n", s)

	err = s.Commit(nodeDir, sh0, true)
	fatalIfErr(t, "failed to commit state", err)

	h1, err := sh0.ResolveFresh("")
	fatalIfErr(t, "failed to resolve the node's IPNS", err)
	h2, err := sh0Prime.ResolveFresh("")
	fatalIfErr(t, "failed to resolve the node's IPNS from second API", err)
	h3, err := sh1.ResolveFresh(sh0ID)
	fatalIfErr(t, "failed to resolve the node's IPNS from another node", err)

	t.Log("h1:", h1, "h2", h2, "h3", h3)

	if h1 == initIPNS {
		t.Fatal("the initial IPNS has not actually been updated")
	}
	if h1 != h2 {
		t.Fatal("the two resolved IPNS hashes are different")
	}

	ipnsObj, err := sh0.ObjectGet(h1)
	fatalIfErr(t, "failed to get IPNS object", err)

	t.Logf("IPNS object: %+v\n", ipnsObj)

	sRem, err := FindStateForNode(sh0ID, sh1)
	fatalIfErr(t, "failed to load state from a different node", err)

	t.Logf("remote state: %+v\n", sRem)

	if !sRem.LastUpdated.Equal(s.LastUpdated) {
		t.Fatal("last updated on the remote node does not match the original state")
	}

	if sRem.Owner.PrivateKey() != nil {
		t.Fatal("somehow found a private key for the remote state")
	}

	if !sRem.Owner.Key().Equals(s.Owner.Key()) {
		t.Fatal("the owner public keys do not match")
	}

	if sRem.Owner.Name != s.Owner.Name {
		t.Fatal("the owner names do not match")
	}

	// this should be loading from IPFS
	sPrime, err := FindLatestState(nodeDir, sh0Prime, true)
	fatalIfErr(t, "failed to find the latest state", err)

	t.Logf("loaded state (IPFS): %+v\n", sPrime)

	if !sPrime.LastUpdated.Equal(s.LastUpdated) {
		t.Fatal("last updated on the reloaded state does not match original state")
	}

	if sPrime.Owner.PrivateKey().Key().D.Cmp(s.Owner.PrivateKey().Key().D) != 0 {
		t.Fatal("loaded private key does not match the original state")
	}

	if sPrime.Owner.Key().Hash() == "" {
		t.Fatal("the loaded owner public key should have a hash")
	}

	luBumped := s.LastUpdated.Add(5 * time.Minute)
	err = ioutil.WriteFile(
		filepath.Join(nodeDir, StateDirectoryName, LastUpdatedFileName),
		[]byte(luBumped.UTC().Format(time.RFC3339Nano)),
		0600,
	)
	fatalIfErr(t, "failed to bump the filesystem last updated time", err)

	sPrime, err = FindLatestState(nodeDir, sh0Prime, true)
	fatalIfErr(t, "failed to find latest state", err)

	t.Logf("loaded state (filesystem): %+v\n", sPrime)

	if !sPrime.LastUpdated.Equal(luBumped) {
		t.Fatal("last updated on the reloaded state does not match bumped time")
	}

	if sPrime.Owner.PrivateKey().Key().D.Cmp(s.Owner.PrivateKey().Key().D) != 0 {
		t.Fatal("loaded private key does not match the original state")
	}

	if sPrime.Owner.Key().Hash() == "" {
		t.Fatal("the loaded owner public key should have a hash")
	}

	os.RemoveAll(nodeDir)
}

func (st *State) mockPublish() {
	for _, g := range st.games {
		g.mockPublish()
	}
}

func TestStateIntractions(t *testing.T) {
	var pPriv, pPub []*Player
	for i := 0; i < 3; i++ {
		priv, err := crypto.NewPrivateKey()
		fatalIfErr(t, fmt.Sprintf("failed to create private key %v", i), err)

		pPriv = append(pPriv, NewPlayer(
			NewPublicKey(priv.GetPublicKey(), fmt.Sprintf("player-%d-public-key", i)),
			NewPrivateKey(priv),
		))
		pPriv[i].Name = fmt.Sprintf("player-%d", i)

		pPub = append(pPub, NewPlayer(
			NewPublicKey(priv.GetPublicKey(), fmt.Sprintf("player-%d-public-key", i)),
			nil,
		))
		pPub[i].Name = fmt.Sprintf("player-%d", i)
	}

	var st []*State
	for i := 0; i < 3; i++ {
		s := NewState()
		s.LastUpdated = time.Now()
		s.Owner = pPriv[i]
		s.mockPublish()
		st = append(st, s)
	}

	// 0 and 1 know about each-other
	// 1 and 2 know about each-other
	// 0 and 2 do not know about each-other

	st[0].AddPlayer(pPub[1])
	st[1].AddPlayer(pPub[0])
	st[1].AddPlayer(pPub[2])
	st[2].AddPlayer(pPub[1])

	t.Logf("st[0] known players: %+v\n", st[0].Players)
	t.Logf("st[1] known players: %+v\n", st[1].Players)

	ch, err := st[0].Combine(st[1])
	fatalIfErr(t, "failed to combine base state 0 and state 1", err)
	if ch {
		t.Fatal("there should have been no changes yet")
	}

	t.Logf("st[0] known players: %+v\n", st[0].Players)
	t.Logf("st[1] known players: %+v\n", st[1].Players)

	_, err = st[0].Combine(st[2])
	if err == nil {
		t.Fatal("should not be able to combine state when we don't know the owner")
	}

	timeout := 5 * time.Hour

	chID, err := st[0].CreateGame(timeout, "lets go")
	fatalIfErr(t, "failed to create first game", err)

	if len(st[0].Challenges()) != 1 {
		t.Fatal("state 0 does not seem to have one challenge")
	}

	st[0].mockPublish()

	ch, err = st[1].Combine(st[0])
	fatalIfErr(t, "failed to combine state 1 with a new challenge from state 0", err)
	if !ch {
		t.Fatal("adding a new challenge to state 1 should be a change")
	}

	t.Logf("st[1] known players: %+v\n", st[1].Players)

	if len(st[1].Challenges()) != 1 {
		t.Fatal("state 1 does not seem to know about one challenge")
	}

	gID, err := st[1].AcceptGame(chID, timeout, "challenge accepted")
	fatalIfErr(t, "failed to accept challenge at state 1", err)

	t.Logf("st[1].games = %+v\n", st[1].games)
	t.Logf("st[1].Challenges() = %+v\n", st[1].Challenges())
	t.Logf("st[1].Games() = %+v\n", st[1].Games())

	if len(st[1].Challenges()) != 1 {
		t.Fatal("state 1 should still list the accepted game as a challenge")
	}

	if len(st[1].Games()) != 1 {
		t.Fatal("state 1 does not seem to have the accepted game listed")
	}

	st[1].mockPublish()

	ch, err = st[0].Combine(st[1])
	fatalIfErr(t, "failed to combine state 0 with the accepted challenge from state 1", err)
	if !ch {
		t.Fatal("adding a challenge acceptance should be a change")
	}

	t.Logf("st[0] known players: %+v\n", st[0].Players)
	t.Logf("st[1] known players: %+v\n", st[1].Players)

	t.Logf("st[0].games = %+v\n", st[0].games)
	t.Logf("st[0].Challenges() = %+v\n", st[0].Challenges())
	t.Logf("st[0].Games() = %+v\n", st[0].Games())

	if len(st[0].Challenges()) != 1 {
		t.Fatal("state should still list the accepted game as a challenge")
	}

	if len(st[0].Games()) != 1 {
		t.Fatal("state 0 does not seem to noticed that the game has been accepted")
	}

	err = st[0].ConfirmGame(gID, timeout, "ok sure lets go")
	fatalIfErr(t, "failed to confirm the game at state 0", err)

	if len(st[0].Challenges()) != 0 {
		t.Fatal("state 0 should now not list the challenge as such")
	}

	if len(st[0].games) != 1 {
		t.Fatal("the game should have been removed from the available challenges")
	}

	st[0].mockPublish()

	err = st[0].StepGame(gID, []byte("step 1"))
	fatalIfErr(t, "failed to add the initial step to the game", err)

	st[0].mockPublish()

	t.Logf("st[0] game players: %+v\n", st[0].games[gID].Players())
	h := st[0].games[gID].head
	for {
		if h == nil {
			break
		}

		t.Log("st[0] commit type", h.Type())
		t.Log("st[0] committer", h.Committer())

		h = h.Parent()
	}

	ch, err = st[1].Combine(st[0])
	fatalIfErr(t, "failed to combine state 1 with the newly running game from 0", err)
	if !ch {
		t.Fatal("confirming and making step should count as a change")
	}

	if len(st[1].Challenges()) != 0 {
		t.Fatal("state 1 should no not publish the challenge")
	}

	if len(st[1].Games()) != 1 {
		t.Fatal("state 1 should now only list a single game")
	}

	if len(st[1].games) != 1 {
		t.Fatal("state 1 should only have a single game in its internal storage")
	}

	ch, err = st[2].Combine(st[1])
	fatalIfErr(t, "failed to combine state 2 with the new game from state 1", err)
	if ch {
		t.Fatal("we shouldn't have made any changes yet because we don't know how to deal with unknown players")
	}

	if len(st[2].games) != 0 {
		t.Fatal("state 2 should not have any games yet since it doesn't know player 0")
	}
}
