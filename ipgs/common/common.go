// Package common provides helper functions used throughout the ipgs code
package common

import (
	"encoding/json"
	"io/ioutil"

	ipfs_config "github.com/ipfs/go-ipfs/repo/config"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/pkg/errors"
)

// MakeIpfsShell creates a Shell given the IPGS config and a Cache
func MakeIpfsShell(c config.Config, ca *cache.Cache) (*cachedshell.Shell, error) {
	fn, err := ipfs_config.Filename(c.IPFS.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build IPFS config filename")
	}

	cBytes, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read IPFS config file")
	}

	var ipfsCfg ipfs_config.Config
	err = json.Unmarshal(cBytes, &ipfsCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal IPFS config json")
	}

	s := cachedshell.NewShell(ipfsCfg.Addresses.API, ca)
	_, err = s.ID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ID from IPFS node")
	}

	return s, nil
}
