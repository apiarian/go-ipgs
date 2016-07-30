package state

import (
	"bytes"
	"testing"
	"time"

	"github.com/apiarian/go-ipgs/crypto"
)

func TestPlayerReadWrite(t *testing.T) {
	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "make private key", err)

	p := NewPlayer(
		NewPublicKey(priv.GetPublicKey(), ""),
		NewPrivateKey(priv),
	)
	p.Timestamp = time.Now()
	p.Name = "test player"
	p.Flags["something"] = 1
	p.Flags["other"] = 2
	p.Nodes = append(p.Nodes, "node1", "node2")

	t.Logf("player: %+v\n", p)

	b := bytes.NewBuffer(nil)

	err = p.Write(b)
	fatalIfErr(t, "write player to buffer", err)

	t.Logf("written: %s\n", b.String())

	l := NewPlayer(nil, nil)
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

	if !p.Key().Equals(l.Key()) {
		t.Fatal("player keys do not match")
	}

	for i, v1 := range p.Nodes {
		if l.Nodes[i] != v1 {
			t.Fatal("player node lists do not match")
		}
	}

	if l.PrivateKey() != nil {
		t.Fatal("loaded player somehow has a private key")
	}
}

func TestPlayerPublishGet(t *testing.T) {
	s, err := newShellForNode(0)
	fatalIfErr(t, "failed to get a shell for node 0", err)

	sPrime, err := newShellForNode(1)
	fatalIfErr(t, "failed to get a shell for node 1", err)

	priv, err := crypto.NewPrivateKey()
	fatalIfErr(t, "failed to create private key", err)

	p := NewPlayer(NewPublicKey(priv.GetPublicKey(), ""), NewPrivateKey(priv))
	p.Timestamp = time.Now()
	p.Name = "test player"
	p.Flags["something"] = 1
	p.Nodes = append(p.Nodes, "node1")

	t.Logf("player: %+v\n", p)

	h, err := p.Publish(s, p)
	fatalIfErr(t, "failed to publish player", err)

	t.Log("hash:", h)

	if h == "" {
		t.Fatal("the hash should not be an empty string")
	}

	obj, err := s.ObjectGet(h)
	fatalIfErr(t, "failed to get player object", err)

	t.Logf("player object: %+v\n", obj)

	l := NewPlayer(nil, nil)
	ah, err := l.Get(h, sPrime)
	fatalIfErr(t, "failed to get player from different node", err)

	t.Logf("loaded: %+v\n", l)

	if ah != p.Key().Hash() {
		t.Fatal("the author key hash is is not the same as the player key hash")
	}

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

	if !p.Key().Equals(l.Key()) {
		t.Fatal("player keys do not match")
	}

	for i, v1 := range p.Nodes {
		if l.Nodes[i] != v1 {
			t.Fatal("player node lists do not match")
		}
	}

	if l.PrivateKey() != nil {
		t.Fatal("loaded player somehow has a private key")
	}
}
