// Package config describes the configuration required for the ipgs system
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Config describes the configuration for an IPGS node. It contains various
// subsections defined by other structs.
type Config struct {
	// IPFS is the IPFS Configuration section for the IPGS node.
	IPFS IpfsConfig
	// IPGS is the IPGS Configuration section for the IPGS node.
	IPGS IpgsConfig
}

// IpfsConfig describes the IPFS Configuration section for an IPGS node. It
// contains the location of the IPFS path and other information required to
// connect to the IPFS node hosting this IPGS node.
type IpfsConfig struct {
	// Path is the IPFS Path. This is usually ~/.ipfs/ . The IPGS node will look
	// inside this directory for the config file to figure out how to connect to
	// the IPFS node's API endpoint. The initializeNode function is actually a
	// bit smarter than all that and asks the local IPGS node for its path.
	Path string
}

// IpgsConfig describes the IPGS Configuration section for an IPGS node. It
// contains flags affecting the behavior of the node during normal operation.
type IpgsConfig struct {
	// UnpinIPNS can be set to true to unpin the previous IPNS object when
	// publishing a new state congiguration
	UnpinIPNS bool
	// APIPort is the port on localhost where the IPGS API will listen for HTTP
	// requests
	APIPort int
}

// Save marshals the config into a proper JSON file in th nodeDir provided
func (c Config) Save(nodeDir string) error {
	cJSON, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return errors.Wrap(err, "failed to marshal config into json")
	}

	cFile, err := os.Create(filepath.Join(nodeDir, "config.json"))
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer cFile.Close()

	cFile.Write(cJSON)
	cFile.WriteString("\n")

	return nil
}
