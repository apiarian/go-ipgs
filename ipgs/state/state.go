package state

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
)

// State describes the state of IPGS node
type State struct {
	// Identity is the path to the identity.asc file for the user
	Identity string
	// The time when this node was last updated
	LastUpdated time.Time
}

// LastUpdatedForOutput formats the LastUpdated timestamp into a string for
// uniform storage
func (st *State) LastUpdatedForOutput() string {
	return st.LastUpdated.UTC().Format(time.RFC3339)
}

// LastUpdatedFromInput reads a string into the LastUpdated timestamp, that the
// string was originally written by the LastUpdatedForOutput method.
// Specifically, that the string is in RFC-3339 format in UTC.
func (st *State) LastUpdatedFromInput(s string) error {
	t, err := time.ParseInLocation(time.RFC3339, s, nil)
	if err != nil {
		return fmt.Errorf("could not parse string '%s': %s", s, err)
	}

	st.LastUpdated = t

	return nil
}

// Publish safely saves the state to the node directory and to IPFS. It writes
// the new state the nodeDir/state-tmp directory, then removes the old
// nodeDir/state and replaces it with nodeDir/state-tmp.  Finally it builds a
// new state IPFS object and publishes it into the node's IPNS object at the
// /ipns/[node]/interplanetary-game-system link.
func (st *State) Publish(
	nodeDir string,
	cfg config.Config,
	s *cachedshell.CachedShell,
) error {
	fsStTmp := filepath.Join(nodeDir, "state-tmp")
	err := os.RemoveAll(fsStTmp)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %s", fsStTmp, err)
	}
	err = os.Mkdir(fsStTmp, 0700)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", fsStTmp, err)
	}

	err = ioutil.WriteFile(
		filepath.Join(fsStTmp, "last-updated"),
		[]byte(st.LastUpdatedForOutput()),
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to write temporary last-updated file: %s", err)
	}

	fsSt := filepath.Join(nodeDir, "state")
	err = os.RemoveAll(fsSt)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %s", fsSt)
	}
	err = os.Rename(fsStTmp, fsSt)
	if err != nil {
		return fmt.Errorf("failed to rename %s to %s: %s", fsStTmp, fsSt, err)
	}

	identHash, err := s.AddPermanentFile(st.Identity)
	if err != nil {
		return fmt.Errorf("failed to add identity file: %s", err)
	}

	stHash, err := s.NewObject("")
	if err != nil {
		return fmt.Errorf("failed to create state object: %s", err)
	}
	stHash, err = s.PatchData(stHash, true, st.LastUpdatedForOutput())
	if err != nil {
		return fmt.Errorf("failed to add last-updated to state: %s", err)
	}

	stHash, err = s.PatchLink(stHash, "identity.asc", identHash, false)
	if err != nil {
		return fmt.Errorf("failed to add identity.asc to state: %s", err)
	}

	curObjHash, err := s.Resolve("")
	if err != nil {
		return fmt.Errorf("failed to resolve nodes IPNS: %s", err)
	}

	newObjHash, err := s.Patch(
		curObjHash,
		"rm-link",
		"interplanetary-game-system",
	)
	if err != nil {
		if !strings.HasSuffix(err.Error(), "not found") {
			return fmt.Errorf("failed to remove old interplanetary-game-system link: %s", err)
		}
		// the interplanetary-game-system link didn't exist anyway, so we'll use
		// the existing object for our purposes
		newObjHash = curObjHash
	}

	newObjHash, err = s.PatchLink(
		newObjHash,
		"interplanetary-game-system",
		stHash,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to add state link to the base: %s", err)
	}

	err = s.Pin(newObjHash)
	if err != nil {
		return fmt.Errorf("failed to pin the new IPNS base object: %s", err)
	}

	err = s.Publish("", newObjHash)
	if err != nil {
		return fmt.Errorf("failed to publish new IPNS base object: %s", err)
	}

	log.Printf("published new IPNS base: /ipfs/%s", newObjHash)

	if cfg.IPGS.UnpinIPNS {
		err = s.Unpin(curObjHash)
		if err != nil {
			log.Printf("failed to unpin old IPNS base object %s: %s", curObjHash, err)
		}
	}

	return nil
}
