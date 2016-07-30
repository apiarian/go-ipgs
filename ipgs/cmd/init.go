// Copyright © 2016 Aleksandr Pasechnik
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/crypto"
	"github.com/apiarian/go-ipgs/ipgs/common"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/apiarian/go-ipgs/ipgs/state"
	"github.com/apiarian/go-ipgs/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	ipfs_config "github.com/ipfs/go-ipfs/repo/config"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "sets up the configuration for a new IPGS node",
	Long:  `Works with the user to build an initial configuration file for the IPGS node.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagNode, err := cmd.Flags().GetString("node")
		util.FatalIfErr("read the node flag", err)

		nodeDir, err := getCleanNodeDir(flagNode)
		util.FatalIfErr("get a clean node directory", err)
		log.Println("using the following node directory:", nodeDir)

		ipfsCfg, err := getIpfsConfig()
		util.FatalIfErr("get the IPFS configuration", err)

		ipgsCfg, err := getIpgsConfig()
		util.FatalIfErr("get the IPGS configuration", err)

		c := config.Config{
			IPFS: ipfsCfg,
			IPGS: ipgsCfg,
		}
		err = c.Save(nodeDir)
		util.FatalIfErr("save the configuration", err)

		err = initCrypto(nodeDir)
		util.FatalIfErr("initialize cryptographic components", err)

		err = bootstrapState(nodeDir, c)
		util.FatalIfErr("bootstrap the state", err)

		log.Println("ipgs is now configured")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func getCleanNodeDir(nodeDir string) (string, error) {
	if nodeDir == "" {
		nodeDir = os.ExpandEnv("$HOME/.ipgs")
	}

	dir, err := util.GetStringForPrompt("IPGS node directory", nodeDir)
	if err != nil {
		return nodeDir, errors.Wrap(err, "failed to read directory node location")
	}
	if dir != nodeDir {
		log.Printf("you will need to use the '--node %s' flag for future invocations\n", dir)
	}
	nodeDir = dir

	nodeDirStats, err := os.Stat(nodeDir)
	if err != nil && !os.IsNotExist(err) {
		return nodeDir, errors.Wrapf(err, "could not get stats on %s", nodeDir)
	}

	if !os.IsNotExist(err) {
		// if nodeDir does actually exist...

		if !nodeDirStats.IsDir() {
			return nodeDir, errors.Errorf(
				"there is a non-directory already at %s, please delete it or choose a different location for the IPGS node directory",
				nodeDir,
			)
		}

		reallyWipe, err := util.GetBoolForPrompt(
			fmt.Sprintf("proceed with deleting %s and its contents?", nodeDir),
			false,
		)
		if err != nil {
			return nodeDir, errors.Wrap(err, "failed to read deletion confirmation")
		}

		if !reallyWipe {
			return nodeDir, errors.Errorf("deletion of existing directory forbidden by user")
		}

		err = os.RemoveAll(nodeDir)
		if err != nil {
			return nodeDir, errors.Wrapf(err, "could not delete %s", nodeDir)
		}
	}

	err = os.MkdirAll(nodeDir, 0750)
	if err != nil {
		return nodeDir, errors.Wrap(err, "could not create IPGS node directory")
	}

	return nodeDir, nil
}

func getIpfsConfig() (config.IpfsConfig, error) {
	c := config.IpfsConfig{
		Path: ipfs_config.DefaultPathRoot,
	}

	betterPath, err := ipfs_config.Path("", "")
	if err != nil {
		log.Println("failed to get the path from IPFS:", err, "; falling back to default")
	} else {
		c.Path = betterPath
	}

	userPath, err := util.GetStringForPrompt(
		"IPFS path",
		c.Path,
	)
	if err != nil {
		return c, errors.Wrap(err, "failed to get new IPFS path from user")
	}
	c.Path = userPath

	if strings.HasPrefix("~/", c.Path) {
		c.Path = strings.Replace(c.Path, "~/", "$HOME/", 1)
	}
	c.Path = os.ExpandEnv(c.Path)

	return c, nil
}

func initCrypto(nodeDir string) error {
	reuseIdent, err := util.GetBoolForPrompt("use existing identity?", false)
	if err != nil {
		return errors.Wrap(err, "failed to get identity reuse choice from user")
	}

	var k *crypto.PrivateKey

	if reuseIdent {
		identPath, err := util.GetStringForPrompt("existing identity path", "")
		if err != nil {
			return errors.Wrap(err, "failed to get existing identity path from user")
		}

		if identPath == "" {
			return errors.New("the user wanted to reuse an identity but did not provide its path")
		}

		identFile, err := os.Open(identPath)
		if err != nil {
			return errors.Wrap(err, "failed to open identity path")
		}
		defer identFile.Close()

		k, err = crypto.ReadPrivateKey(identFile)
		if err != nil {
			return errors.Wrap(err, "failed to read private key from identity file")
		}
	} else {
		k, err = crypto.NewPrivateKey()
		if err != nil {
			return errors.Wrap(err, "failed to create a new private key")
		}

		n, err := util.GetStringForPrompt("name for the new identity", "")
		if err != nil {
			return errors.Wrap(err, "failed to get name from the user")
		}

		c, err := util.GetStringForPrompt("comment for the new identity", "IPGS Identity")
		if err != nil {
			return errors.Wrap(err, "failed to get comment from the user")
		}

		k.Name = n
		k.Comment = c
	}

	f, err := os.Create(filepath.Join(nodeDir, state.PrivateKeyFileName))
	if err != nil {
		return errors.Wrap(err, "failed to create private key file")
	}
	defer f.Close()

	err = f.Chmod(0600)
	if err != nil {
		return errors.Wrap(err, "failed to set private key file to user r/w only")
	}

	err = k.Write(f)
	if err != nil {
		return errors.Wrap(err, "failed to write private key to file")
	}

	err = f.Chmod(0400)
	if err != nil {
		return errors.Wrap(err, "failed to set private key file to user read-only")
	}

	return nil
}

func getIpgsConfig() (config.IpgsConfig, error) {
	c := config.IpgsConfig{
		UnpinIPNS: true,
		APIPort:   9090,
	}

	reallyUnpin, err := util.GetBoolForPrompt(
		"unpin the IPNS object when overwriting it?",
		c.UnpinIPNS,
	)
	if err != nil {
		return c, errors.Wrap(err, "failed to get IPNS overwrite confirmation from user")
	}
	c.UnpinIPNS = reallyUnpin

	requestedPort, err := util.GetIntForPrompt(
		"port on which to listen for HTTP API requests",
		c.APIPort,
	)
	if err != nil {
		return c, errors.Wrap(err, "failed to get IPGS API port from user")
	}
	c.APIPort = requestedPort

	return c, nil
}

func bootstrapState(nodeDir string, cfg config.Config) error {
	s, err := common.MakeIpfsShell(cfg, cache.NewCache())
	if err != nil {
		return errors.Wrap(err, "could not create IPFS shell")
	}

	privFile, err := os.Open(filepath.Join(nodeDir, state.PrivateKeyFileName))
	if err != nil {
		return errors.Wrap(err, "failed to open private key file")
	}
	defer privFile.Close()

	k, err := crypto.ReadPrivateKey(privFile)
	if err != nil {
		return errors.Wrap(err, "failed to read the private key")
	}

	name, err := util.GetStringForPrompt(
		"player name",
		k.Name,
	)
	if err != nil {
		return errors.Errorf("could not get player name from user")
	}

	nodeId, err := s.ID()
	if err != nil {
		return errors.Wrap(err, "failed to read ID from IPFS node")
	}
	nodesStr, err := util.GetStringForPrompt(
		"IPFS backing nodes (comma separated list of IDs)",
		nodeId.ID,
	)
	nodes := strings.Split(nodesStr, ",")

	owner := state.NewPlayer(
		state.NewPublicKey(k.GetPublicKey(), ""),
		state.NewPrivateKey(k),
	)
	owner.Timestamp = time.Now()
	owner.Name = name
	owner.Nodes = nodes

	st := state.NewState()
	st.Owner = owner
	st.LastUpdated = time.Now()

	err = st.Write(nodeDir)
	if err != nil {
		return errors.Wrap(err, "failed to write initial state")
	}

	h, err := st.Publish(s)
	if err != nil {
		return errors.Wrap(err, "failed to publish initial state")
	}

	h, err = common.InstallIpgsStateHash(h, s, cfg.IPGS.UnpinIPNS)
	if err != nil {
		return errors.Wrap(err, "failed to install initial IPGS state")
	}

	log.Println("published IPGS state to", h)

	return nil
}
