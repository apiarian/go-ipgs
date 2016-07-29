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

	"github.com/apiarian/go-ipgs/crypto"
	"github.com/apiarian/go-ipgs/util"
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
		Count:     3,
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

	o := NewPlayer()
	o.Timestamp = time.Now()
	o.Name = "owner"
	o.Flags["something"] = 1
	o.Flags["shiny"] = 1000
	o.Key = &PublicKey{pk.GetPublicKey(), ""}
	o.PrivateKey = &PrivateKey{pk}
	o.Nodes = append(o.Nodes, "node1")
	s.Owner = o

	for i := 0; i < 4; i++ {
		pk, err := crypto.NewPrivateKey()
		fatalIfErr(t, "failed to create provate key for other player", err)

		p := NewPlayer()
		p.Timestamp = time.Now()
		p.Name = fmt.Sprintf("player%v", i)
		p.Flags["cool-factor"] = i
		p.Key = &PublicKey{pk.GetPublicKey(), ""}
		p.Nodes = append(o.Nodes, fmt.Sprintf("player-node-%v", i))

		s.Players = append(s.Players, p)
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

	os.RemoveAll(nodeDir)
}
