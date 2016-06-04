package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	ipfs_shell "github.com/ipfs/go-ipfs-api"
)

type AppSpace struct {
	// Identity is the path to the identity.asc file for the user
	Identity string
	// The time when this node was last updated
	LastUpdated time.Time
}

func LoadCurrentAppSpace(nodeDir string, config Config, shell *ipfs_shell.Shell) (*AppSpace, error) {
	as := &AppSpace{}

	ident := filepath.Join(nodeDir, "identity.asc")
	identStats, err := os.Stat(ident)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return nil, fmt.Errorf("could not find %s", ident)
		default:
			return nil, fmt.Errorf("failed to get information on %s: %s", ident, err)
		}
	}
	if identStats.IsDir() {
		return nil, fmt.Errorf("%s is unexpectedly a directory", ident)
	}
	as.Identity = ident

	var foundAppSpace bool
	s, err := shell.Resolve(readCache("ipfs-node-id").(string))
	if err != nil && !strings.HasSuffix(err.Error(), "Could not resolve name.") {
		return nil, fmt.Errorf("failed to resolve IPFS node's ipns value: %s", err)
	}
	if err == nil {
		log.Println("node IPNS resolved to", s)
		appSpaceObject, err := shell.ObjectGet(
			fmt.Sprintf("%s/interplanetary-game-system", s),
		)
		if err != nil {
			if !strings.Contains(err.Error(), `no link named "interplanetary-game-system"`) {
				return nil, fmt.Errorf("error getting existing interplanetary-game-system: %s", err)
			}
			log.Println("no interplanetary-game-system node found in IPNS base")
		} else {
			as.LastUpdated, err = time.ParseInLocation(time.RFC3339, appSpaceObject.Data, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to parse last-updated timestamp: %s", err)
			}
			foundAppSpace = true
		}
	}

	if !foundAppSpace {
		// need to load the App Space data from the filesystem and hope for the best
		fsAS := filepath.Join(nodeDir, "app-space")
		fsASStats, err := os.Stat(fsAS)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("could not get information on %s: %s", fsAS, err)
		}
		if err == nil {
			if !fsASStats.IsDir() {
				return nil, fmt.Errorf("%s is unexpectedly not a directory", fsAS)
			}
			luName := filepath.Join(fsAS, "last-updated")
			luBytes, err := ioutil.ReadFile(luName)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %s", luName, err)
			}
			as.LastUpdated, err = time.ParseInLocation(time.RFC3339, string(luBytes), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to parse last-updated timestamp: %s", err)
			}
			foundAppSpace = true
		}
	}

	if !foundAppSpace {
		// still haven't found anything, lets make up sane defaults and move on
		as.LastUpdated = time.Time{}
	}

	return as, nil
}

func (as *AppSpace) Publish(nodeDir string, config Config, shell *ipfs_shell.Shell) error {
	fsAStmp := filepath.Join(nodeDir, "app-space-tmp")
	err := os.RemoveAll(fsAStmp)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %s", fsAStmp, err)
	}
	err = os.Mkdir(fsAStmp, 0700)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", fsAStmp, err)
	}

	tmpLuName := filepath.Join(fsAStmp, "last-updated")
	lu, err := os.Create(tmpLuName)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", tmpLuName, err)
	}
	defer lu.Close()
	luFormatted := as.LastUpdated.UTC().Format(time.RFC3339)
	_, err = lu.WriteString(luFormatted)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %s", tmpLuName, err)
	}

	fsAS := filepath.Join(nodeDir, "app-space")
	err = os.RemoveAll(fsAS)
	if err != nil {
		return fmt.Errorf("failed to remove %s: %s", fsAS)
	}
	err = os.Rename(fsAStmp, fsAS)
	if err != nil {
		return fmt.Errorf("failed to move rename %s: %s", fsAStmp, err)
	}

	identIPFS := readCacheString("ipgs-node-identity")
	if identIPFS == "" {
		identName := filepath.Join(nodeDir, "identity.asc")
		identFile, err := os.Open(identName)
		if err != nil {
			return fmt.Errorf("failed to open %s: %s", identName, err)
		}
		defer identFile.Close()
		identIPFS, err = shell.Add(identFile)
		if err != nil {
			return fmt.Errorf("failed to add identity.asc: %s", err)
		}
	}
	log.Println("identity.asc:", identIPFS)

	appSpaceIPFS, err := shell.NewObject("")
	if err != nil {
		return fmt.Errorf("failed to create app-space object: %s", err)
	}
	appSpaceIPFS, err = shell.PatchData(
		appSpaceIPFS,
		true,
		luFormatted,
	)
	if err != nil {
		return fmt.Errorf("failed to set last-updated to app-space: %s", err)
	}
	appSpaceIPFS, err = shell.PatchLink(
		appSpaceIPFS,
		"identity.asc",
		identIPFS,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to add identity.asc to app-space: %s", err)
	}
	log.Println("app-space object:", appSpaceIPFS)

	s, err := shell.Resolve(readCacheString("ipfs-node-id"))
	if err != nil && !strings.HasSuffix(err.Error(), "Could not resolve name.") {
		return fmt.Errorf("failed to resolve IPFS node's ipns value: %s", err)
	}
	var baseIPFS string
	if err == nil {
		log.Println("node IPNS resolved to", s)
		baseIPFS = strings.Replace(s, "/ipfs/", "", -1)
	} else {
		// We get here if we actually got a "Could not resolve name." error
		baseIPFS, err = shell.NewObject("")
		if err != nil {
			return fmt.Errorf("failed to create base object for IPNS: %s", err)
		}
	}
	newBaseIPFS, err := shell.Patch(
		baseIPFS,
		"rm-link",
		"interplanetary-game-system",
	)
	if err != nil && !strings.HasSuffix(err.Error(), "not found") {
		return fmt.Errorf("failed to remove old interplanetary-game-system link: %s", err)
	}
	if err == nil {
		baseIPFS = newBaseIPFS
	}
	baseIPFS, err = shell.PatchLink(
		baseIPFS,
		"interplanetary-game-system",
		appSpaceIPFS,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to add app-space link to base: %s", err)
	}
	log.Println("IPNS base object:", baseIPFS)

	err = shell.Pin(baseIPFS)
	if err != nil {
		return fmt.Errorf("failed to pin the new IPNS base object: %s", err)
	}
	err = shell.Publish("", baseIPFS)
	if err != nil {
		return fmt.Errorf("failed to publish the new base object to IPNS: %s", err)
	}

	return nil
}
