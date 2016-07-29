package state

import (
	"bytes"
	"testing"
	"time"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/whyrusleeping/iptb/util"
)

func TestPlayerReadWrite(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "make private key", err)

	p := &Player{
		Timestamp: time.Now(),
		Name:      "test player",
		Flags: map[string]int{
			"something": 1,
			"other":     2,
		},
		Key:        &PublicKey{priv.GetPublicKey(), ""},
		PrivateKey: &PrivateKey{priv},
		Nodes:      []string{"node1", "node2"},
	}

	t.Logf("player: %+v\n", p)

	b := bytes.NewBuffer(nil)

	err = p.Write(b)
	fatalIfErr(t, "write player to buffer", err)

	t.Logf("written: %s\n", b.String())

	l := &Player{}
	err = l.Read(b)
	fatalIfErr(t, "read player from buffer", err)

	t.Logf("loaded: %+v\n", l)

	if !p.Timestamp.Equal(l.Timestamp) {
		t.Fatal("player timestamps do not match")
	}

	if p.Name != l.Name {
		t.Fatal("player names do not match")
	}

	for k, v1 := range p.Flags {
		if l.Flags[k] != v1 {
			t.Fatalf("player flags do not match")
		}
	}

	if !p.Key.Equals(l.Key) {
		t.Fatal("player keys do not match")
	}

	for i, v1 := range p.Nodes {
		if l.Nodes[i] != v1 {
			t.Fatal("player node lists do not match")
		}
	}

	if l.PrivateKey != nil {
		t.Fatal("loaded player somehow has a private key")
	}
}

func TestPlayerPublishGet(t *testing.T) {
	node, err := iptbutil.LoadNodeN(0)
	fatalIfErr(t, "failed to load node 0", err)

	addr, err := node.APIAddr()
	fatalIfErr(t, "failed to get node api address", err)
	t.Log("node API address:", addr)

	c := cache.NewCache()
	s := cachedshell.NewShell(addr, c)
	nodeid, err := s.ID()
	fatalIfErr(t, "failed to connect to ipfs node", err)
	t.Logf("node id: %+v\n", nodeid)

	t.Fatal("bump")
}
