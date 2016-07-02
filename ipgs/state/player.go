package state

import (
	"encoding/json"
	"fmt"

	"github.com/apiarian/go-ipgs/cachedshell"
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
