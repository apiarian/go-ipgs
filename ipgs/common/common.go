// Package common provides helper functions used throughout the ipgs code
package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	ipfs_config "github.com/ipfs/go-ipfs/repo/config"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
)

// MakeIpfsShell creates a CachedShell given the IPGS config and a Cache
func MakeIpfsShell(c config.Config, ca *cache.Cache) (*cachedshell.CachedShell, error) {
	fn, err := ipfs_config.Filename(c.IPFS.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to build IPFS config filename: %s", err)
	}

	cBytes, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to read IPFS config file: %s", err)
	}

	var ipfsCfg ipfs_config.Config
	err = json.Unmarshal(cBytes, &ipfsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal IPFS config json: %s", err)
	}

	s := cachedshell.NewCachedShell(ipfsCfg.Addresses.API, ca)
	_, err = s.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get ID from IPFS node: %s", err)
	}

	return s, nil
}