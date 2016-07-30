// Package common provides helper functions used throughout the ipgs code
package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	ipfs_config "github.com/ipfs/go-ipfs/repo/config"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/apiarian/go-ipgs/ipgs/state"
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

// FindIpgsHash looks for an IPGS state structure under a node's IPNS root
func FindIpgsHash(nodeId string, s *cachedshell.Shell) (string, error) {
	var ipfsHash string
	var err error
	if nodeId == "" {
		ipfsHash, err = s.Resolve("")
	} else {
		ipfsHash, err = s.Resolve(fmt.Sprintf("/ipns/%s", nodeId))
	}
	if err != nil {
		return "", errors.Wrapf(err, "could not resolve %s", nodeId)
	}

	stateHash, err := s.ResolvePath(fmt.Sprintf("%s/%s", ipfsHash, state.StateLinkName))
	if err != nil {
		return "", errors.Wrapf(err, "no IPGS object for node %s", nodeId)
	}

	return stateHash, nil
}

func InstallIpgsStateHash(
	h string,
	s *cachedshell.Shell,
	unpinOldIPNS bool,
) (string, error) {
	curObjHash, err := s.Resolve("")
	if err != nil {
		if !strings.HasSuffix(err.Error(), "Could not resolve name.") {
			return "", errors.Wrap(err, "failed to resolve node's IPNS")
		}

		curObjHash, err = s.NewObject("")
		if err != nil {
			return "", errors.Wrap(err, "failed to create new IPNS base object")
		}
	}

	newObjHash, err := s.Patch(curObjHash, "rm-link", state.StateLinkName)
	if err != nil {
		if !strings.HasSuffix(err.Error(), "not found") {
			return "", errors.Wrap(err, "failed to remove old state link")
		}

		newObjHash = curObjHash
	}

	newObjHash, err = s.PatchLink(newObjHash, state.StateLinkName, h, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to add state link to the base")
	}

	err = s.Pin(newObjHash)
	if err != nil {
		return "", errors.Wrap(err, "failed to pin new IPNS base object")
	}

	err = s.Publish("", newObjHash)
	if err != nil {
		return "", errors.Wrap(err, "failed to publish new IPNS base object")
	}

	if unpinOldIPNS {
		err = s.Unpin(curObjHash)
		if err != nil {
			log.Printf("failed to unpin old IPNS base object %s: %+v\n", curObjHash, err)
		}
	}

	return newObjHash, nil
}
