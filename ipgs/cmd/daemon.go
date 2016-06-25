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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	ipfs_config "github.com/ipfs/go-ipfs/repo/config"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/apiarian/go-ipgs/ipgs/state"
	"github.com/apiarian/go-ipgs/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := cache.NewCache()

		cFile := viper.ConfigFileUsed()
		if cFile == "" {
			log.Fatalln("no configuration found, please run ipgs init to set up your node")
		}
		nodeDir = filepath.Dir(cFile)

		cmdNodeDir, err := cmd.Parent().PersistentFlags().GetString("node")
		util.FatalIfErr("get node flag string value", err)
		if nodeDir != cmdNodeDir {
			log.Fatalln("the node parameter on the command line does not match the directory of the config file; this is not supported")
		}

		var cfg config.Config
		err = viper.Unmarshal(&cfg)
		util.FatalIfErr("unmarshal config", err)

		s, err := makeIpfsShell(cfg, c)
		util.FatalIfErr("make IPFS shell", err)

		id, err := s.ID()
		util.FatalIfErr("get IPFS node ID", err)

		log.Println("connected to IPFS node", id.ID)

		st, err := loadLatestState(nodeDir, cfg, s)
		util.FatalIfErr("load latest state", err)

		st.LastUpdated = time.Now()

		err = st.Publish(nodeDir, cfg, s)
		util.FatalIfErr("publish state", err)
	},
}

func init() {
	RootCmd.AddCommand(daemonCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// daemonCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// daemonCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func makeIpfsShell(c config.Config, ca *cache.Cache) (*cachedshell.CachedShell, error) {
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

func loadLatestState(
	nodeDir string,
	c config.Config,
	s *cachedshell.CachedShell,
) (*state.State, error) {
	stDir := filepath.Join(nodeDir, "state")

	fsSt := &state.State{}

	fsLastUpdated, err := ioutil.ReadFile(filepath.Join(stDir, "last-updated"))
	if err != nil {
		return nil, fmt.Errorf("failed to read last-updated from file: %s", err)
	}
	err = fsSt.LastUpdatedFromInput(string(fsLastUpdated))
	if err != nil {
		return nil, fmt.Errorf("failed to process last-updated from file: %s", err)
	}

	ipfsSt := &state.State{}
	ipnsHash, err := s.Resolve("")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve nodes's IPNS: %s", err)
	}

	stObject, err := s.ObjectGet(fmt.Sprintf("%s/interplanetary-game-system", ipnsHash))
	if err != nil {
		if !strings.Contains(err.Error(), `no link named "interplanetary-game-system"`) {
			return nil, fmt.Errorf("failed to request IPGS state under IPNS: %s", err)
		}
	} else {
		log.Println("found state under IPNS base", ipnsHash)
		err = ipfsSt.LastUpdatedFromInput(stObject.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to process last-updated from IPFS object: %s", err)
		}
	}

	var curSt *state.State
	if ipfsSt.LastUpdated.After(fsSt.LastUpdated) {
		log.Println("IPFS state is more fresh than the filesystem one")
		curSt = ipfsSt
	} else {
		log.Println("filesystem state is at least as fresh as the IPFS one")
		curSt = fsSt
	}

	curSt.Identity = filepath.Join(nodeDir, "identity.asc")

	return curSt, nil
}
