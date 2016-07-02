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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"

	"github.com/apiarian/go-ipgs/cache"
	"github.com/apiarian/go-ipgs/ipgs/common"
	"github.com/apiarian/go-ipgs/ipgs/config"
	"github.com/apiarian/go-ipgs/ipgs/state"
	"github.com/apiarian/go-ipgs/util"
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

		gpgCfg, err := getGpgConfig(nodeDir)
		util.FatalIfErr("get the GPG configuration", err)

		ipfsCfg, err := getIpfsConfig()
		util.FatalIfErr("get the IPFS configuration", err)

		ipgsCfg, err := getIpgsConfig()
		util.FatalIfErr("get the IPGS configuration", err)

		c := config.Config{
			GPG:  gpgCfg,
			IPFS: ipfsCfg,
			IPGS: ipgsCfg,
		}
		err = c.Save(nodeDir)
		util.FatalIfErr("save the configuration", err)

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
		return nodeDir, fmt.Errorf("failed to read node directory location: %s", err)
	}
	if dir != nodeDir {
		log.Printf("you will need to use the '--node %s' flag for future invocations\n", dir)
	}
	nodeDir = dir

	nodeDirStats, err := os.Stat(nodeDir)
	if err != nil && !os.IsNotExist(err) {
		return nodeDir, fmt.Errorf("could not get stats on %s: %s", nodeDir, err)
	}

	if !os.IsNotExist(err) {
		// if nodeDir does actually exist...

		if !nodeDirStats.IsDir() {
			return nodeDir, fmt.Errorf(
				"there is a non-directory already at %s, please delete it or choose a different location for the IPGS node directory",
				nodeDir,
			)
		}

		reallyWipe, err := util.GetBoolForPrompt(
			fmt.Sprintf("proceed with deleting %s and its contents?", nodeDir),
			false,
		)
		if err != nil {
			return nodeDir, fmt.Errorf("failed to read deletion confirmation: %s", err)
		}

		if !reallyWipe {
			return nodeDir, fmt.Errorf("deletion of existing directory forbidden by user")
		}

		err = os.RemoveAll(nodeDir)
		if err != nil {
			return nodeDir, fmt.Errorf("could not delete %s: %s", nodeDir, err)
		}
	}

	err = os.MkdirAll(nodeDir, 0750)
	if err != nil {
		return nodeDir, fmt.Errorf("could not create IPGS node directory: %s", err)
	}

	return nodeDir, nil
}

func getGpgConfig(nodeDir string) (config.GpgConfig, error) {
	c := config.GpgConfig{
		Home:       os.ExpandEnv("$HOME/.gnupg/"),
		ShortKeyID: "",
	}

	newHome, err := util.GetStringForPrompt(
		"GPG Home directory",
		c.Home,
	)
	if err != nil {
		return c, fmt.Errorf("failed to get GPG home directory input: %s", err)
	}
	c.Home = newHome

	needNewKeys, err := util.GetBoolForPrompt("create new OpenPGP keypair for this node?", true)
	if err != nil {
		return c, fmt.Errorf("failed to read OpenPGP keypair creation confirmation: %s", err)
	}

	if needNewKeys {
		gpgPath, err := exec.LookPath("gpg")
		if err != nil {
			return c, fmt.Errorf("IPGS depends on the gpg keychain for key storage; failed to find gpg in the search path: %s", err)
		}

		gpgOk, err := util.GetBoolForPrompt(
			fmt.Sprintf("found gpg at %s; ok?", gpgPath),
			true,
		)
		if err != nil {
			return c, fmt.Errorf("failed to get gpg path confirmation: %s", err)
		}
		if !gpgOk {
			return c, fmt.Errorf("please make sure that the correct gpg executable is topmost in your search path")
		}

		name, err := util.GetStringForPrompt(
			"OpenPGP Entity Name",
			"",
		)
		if err != nil {
			return c, fmt.Errorf("failed to get OpenPGP Entity Name: %s", err)
		}
		comment, err := util.GetStringForPrompt(
			"OpenPGP Entity Comment",
			"IPGS Player Identity",
		)
		if err != nil {
			return c, fmt.Errorf("failed to get OpenPGP Entity Comment: %s", err)
		}
		email, err := util.GetStringForPrompt(
			"OpenPGP Entity Email",
			fmt.Sprintf(
				"%s@ipgs",
				strings.Replace(
					strings.ToLower(name),
					" ",
					"",
					-1,
				),
			),
		)
		if err != nil {
			return c, fmt.Errorf("failed to get OpenPGP Entity Email: %s", err)
		}

		entity, err := openpgp.NewEntity(name, comment, email, nil)
		if err != nil {
			return c, fmt.Errorf("failed to create new OpenPGP entity: %s", err)
		}
		c.ShortKeyID = entity.PrimaryKey.KeyIdShortString()
		log.Println("created key", c.ShortKeyID)

		for _, id := range entity.Identities {
			err := id.SelfSignature.SignUserId(
				id.UserId.Id,
				entity.PrimaryKey,
				entity.PrivateKey,
				nil,
			)
			if err != nil {
				return c, fmt.Errorf("failed to self-sign identity: %s", err)
			}
		}

		privateKeyFile, err := os.Create(filepath.Join(nodeDir, "private.asc"))
		if err != nil {
			return c, fmt.Errorf("failed to create private key file: %s", err)
		}
		defer privateKeyFile.Close()
		err = privateKeyFile.Chmod(0400)
		if err != nil {
			return c, fmt.Errorf("failed to set the private key file to read-only: %s", err)
		}
		privateEncoder, err := armor.Encode(privateKeyFile, openpgp.PrivateKeyType, nil)
		if err != nil {
			return c, fmt.Errorf("failed to create armorer for private key: %s", err)
		}
		entity.SerializePrivate(privateEncoder, nil)
		privateEncoder.Close()
		privateKeyFile.Close()
		cmd := exec.Command(
			gpgPath,
			"--import",
			privateKeyFile.Name(),
		)
		o, err := cmd.CombinedOutput()
		if err != nil {
			return c, fmt.Errorf("failed to get the combined output from gpg command: %s", err)
		} else {
			log.Printf("captured the following data from gpg:\n\n%s\n", string(o))
		}

		delPrivKey, err := util.GetBoolForPrompt(
			"delete the private key file?",
			true,
		)
		if err != nil {
			log.Printf("failed to get confirmation for deleting the private key file: %s\n\ndeleting by default")
			delPrivKey = true
		}

		if delPrivKey {
			err := os.Remove(privateKeyFile.Name())
			if err != nil {
				log.Println(
					"failed to delete the private key file; please delete it manually from",
					privateKeyFile.Name(),
				)
			} else {
				log.Println("deleted the private key file")
			}
		}
	} else {
		// do not need to create a new key

		c.ShortKeyID, err = util.GetStringForPrompt(
			"OpenPGP Short Key ID",
			c.ShortKeyID,
		)
		if err != nil {
			return c, fmt.Errorf("failed to get the OpenPGP Short Key ID: %s", err)
		}
	}

	return c, nil
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
		return c, fmt.Errorf("failed to get new IPFS path from user: %s", err)
	}
	c.Path = userPath

	if strings.HasPrefix("~/", c.Path) {
		c.Path = strings.Replace(c.Path, "~/", "$HOME/", 1)
	}
	c.Path = os.ExpandEnv(c.Path)

	return c, nil
}

func bootstrapState(nodeDir string, cfg config.Config) error {
	// write the identity file to the node directory, not the state since it
	// won't be changing for the life of the node and we don't need to keep
	// copying and moving it around when we do our state dance
	idFilename, err := writeIdentityFile(nodeDir, cfg.GPG)
	if err != nil {
		return fmt.Errorf("could not write identity file: %s", err)
	}

	c := cache.NewCache()
	s, err := common.MakeIpfsShell(cfg, c)
	if err != nil {
		return fmt.Errorf("could not create IPFS shell: %s", err)
	}

	pubKeyHash, err := s.AddPermanentFile(idFilename)
	if err != nil {
		return fmt.Errorf("could not add identity.asc permanently: %s", err)
	}

	_, prvRing, err := util.GetPublicPrivateRings(cfg.GPG.Home)
	if err != nil {
		return fmt.Errorf("could not load private keyring: %s", err)
	}
	entity, err := util.FindEntityForKeyId(prvRing, cfg.GPG.ShortKeyID)
	if err != nil {
		return fmt.Errorf("could not find the node's identity: %s", err)
	}
	var n string
	for _, v := range entity.Identities {
		n = v.UserId.Name
		break
	}

	name, err := util.GetStringForPrompt(
		"player name",
		n,
	)
	if err != nil {
		return fmt.Errorf("could not get player name from user: %s", err)
	}

	nodeId, err := s.ID()
	if err != nil {
		return fmt.Errorf("failed to read ID from IPFS node: %s", err)
	}
	nodesStr, err := util.GetStringForPrompt(
		"IPFS backing nodes (comma separated list of IDs)",
		nodeId.ID,
	)
	nodes := strings.Split(nodesStr, ",")

	player := &state.Player{
		PublicKeyHash:       pubKeyHash,
		PreviousVersionHash: "",
		Timestamp:           state.IPGSTime{time.Now()},
		Name:                name,
		Nodes:               nodes,
	}

	st := state.State{
		Identity:    idFilename,
		LastUpdated: state.IPGSTime{time.Now()},
		Players:     []*state.Player{player},
	}

	err = st.Publish(nodeDir, cfg, s)
	if err != nil {
		return fmt.Errorf("could not publish initial state: %s", err)
	}

	return nil
}

func writeIdentityFile(nodeDir string, gpgCfg config.GpgConfig) (string, error) {
	_, prvRing, err := util.GetPublicPrivateRings(gpgCfg.Home)
	if err != nil {
		return "", fmt.Errorf("failed to load private keyring: %s", err)
	}

	entity, err := util.FindEntityForKeyId(prvRing, gpgCfg.ShortKeyID)
	if err != nil {
		return "", fmt.Errorf("failed to find the node's entity: %s", err)
	}

	pubKeyFile, err := os.Create(filepath.Join(nodeDir, "identity.asc"))
	if err != nil {
		return "", fmt.Errorf("failed to create public key file: %s", err)
	}
	defer pubKeyFile.Close()

	pubEncoder, err := armor.Encode(pubKeyFile, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create armorer for the public key: %s", err)
	}
	defer pubEncoder.Close()

	entity.Serialize(pubEncoder)

	return pubKeyFile.Name(), nil
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
		return c, fmt.Errorf("failed to get IPNS overwrite confirmation from user: %s", err)
	}
	c.UnpinIPNS = reallyUnpin

	requestedPort, err := util.GetIntForPrompt(
		"port on which to listen for HTTP API requests",
		c.APIPort,
	)
	if err != nil {
		return c, fmt.Errorf("failed to get IPGS API port from user: %s", err)
	}
	c.APIPort = requestedPort

	return c, nil
}
