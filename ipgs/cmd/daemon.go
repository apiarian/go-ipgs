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
	"net/http"
	"path/filepath"
	"time"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/apiarian/go-ipgs/ipgs/common"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/apiarian/go-ipgs/ipgs/state"
	"github.com/apiarian/go-ipgs/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"goji.io"
	"goji.io/pat"
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

		s, err := common.MakeIpfsShell(cfg, c)
		util.FatalIfErr("make IPFS shell", err)

		id, err := s.ID()
		util.FatalIfErr("get IPFS node ID", err)

		log.Println("connected to IPFS node", id.ID)

		st, err := state.FindLatestState(nodeDir, s, cfg.IPGS.UnpinIPNS)
		util.FatalIfErr("load latest state", err)

		b := state.NewBroker(st, nodeDir, s, cfg.IPGS.UnpinIPNS)

		st = b.Checkout()
		log.Printf("initial state: %+v\n", st)
		b.Return()

		go periodicallyUpdateState(b, s)

		root := goji.NewMux()

		players := goji.SubMux()
		root.HandleC(pat.New("/players/*"), players)

		players.HandleFuncC(
			pat.Get("/:id"),
			state.MakePlayersGetOneHandler(b),
		)
		players.HandleFuncC(
			pat.Patch("/:id"),
			state.MakePlayersPatchHandler(b),
		)
		players.HandleFuncC(
			pat.Get("/"),
			state.MakePlayersGetHandler(b),
		)
		players.HandleFuncC(
			pat.Post("/"),
			state.MakePlayersPostHandler(b, s),
		)

		challenges := goji.SubMux()
		root.HandleC(pat.New("/challenges/*"), challenges)

		challenges.HandleFuncC(
			pat.Get("/:id"),
			state.MakeChallengesGetOneHandler(b),
		)
		challenges.HandleFuncC(
			pat.Post("/:id/accept"),
			state.MakeChallengesAcceptHandler(b),
		)
		challenges.HandleFuncC(
			pat.Get("/"),
			state.MakeChallengesGetHandler(b),
		)
		challenges.HandleFuncC(
			pat.Post("/"),
			state.MakeChallengesPostHandler(b),
		)

		games := goji.SubMux()
		root.HandleC(pat.New("/games/*"), games)

		games.HandleFuncC(
			pat.Get("/:id"),
			state.MakeGamesGetOneHandler(b),
		)
		games.HandleFuncC(
			pat.Get("/"),
			state.MakeGamesGetHandler(b),
		)

		addr := fmt.Sprintf("127.0.0.1:%v", cfg.IPGS.APIPort)
		log.Println("HTTP API starting at", addr)
		log.Fatal(http.ListenAndServe(addr, root))
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

func periodicallyUpdateState(
	b *state.Broker,
	s *cachedshell.Shell,
) {
	for {
		updateState(b, s)

		log.Println("sleeping for 5 seconds")
		time.Sleep(5 * time.Second)
	}
}

func updateState(
	b *state.Broker,
	s *cachedshell.Shell,
) {
	st := b.Checkout()
	defer b.Return()

	log.Println("updating state")

	var changed bool

	for _, p := range st.Players {
		var pSt *state.State
		for _, n := range p.Nodes {
			stN, err := state.FindStateForNode(n, s)
			if err != nil {
				log.Printf("could not find IPGS state for player %s node %s: %+v\n", p, n, err)
				continue
			}

			if pSt == nil || stN.LastUpdated.After(pSt.LastUpdated) {
				pSt = stN
			}
		}

		if pSt == nil {
			log.Printf("could not find any IPGS state for player %s\n", p)
			continue
		}

		c, err := st.Combine(pSt)
		if err != nil {
			log.Printf("failed to combine state with the state for player %s: %+v\n", p, err)
		}

		if c {
			changed = true
		}
	}

	if changed {
		err := b.Checkin()
		if err != nil {
			log.Printf("failed to checkin state: %+v\n", err)
		}
	}
}
