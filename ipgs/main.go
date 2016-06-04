package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	ipfs_shell "github.com/ipfs/go-ipfs-api"
	ipfs_config "github.com/ipfs/go-ipfs/repo/config"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

var (
	cache     map[string]interface{}
	cacheLock sync.Mutex
)

func writeCache(k string, v interface{}) {
	cacheLock.Lock()
	defer cacheLock.Unlock()
	cache[k] = v
}

func readCache(k string) interface{} {
	cacheLock.Lock()
	defer cacheLock.Unlock()
	return cache[k]
}

func readCacheString(k string) string {
	cacheLock.Lock()
	defer cacheLock.Unlock()
	v := cache[k]
	if v == nil {
		return ""
	}
	return v.(string)
}

func init() {
	cache = make(map[string]interface{})
}

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatalln("failed to detect user:", err)
	}

	var (
		ipgsDir = flag.String(
			"ipgs-dir",
			filepath.Join(usr.HomeDir, ".ipgs"),
			"The path to the IPGS node directory",
		)
		initialize = flag.Bool(
			"init",
			false,
			"Initialize the IPGS node. Warning: will wipe out the existing ipgs-dir contents",
		)
	)
	flag.Parse()

	if *initialize {
		initializeNode(*ipgsDir)
		log.Println("initialization complete")
	}

	config := loadConfig(*ipgsDir)
	log.Printf("configuration: %+v\n", config)

	pubRingFile, err := os.Open(filepath.Join(config.GPG.Home, "pubring.gpg"))
	if err != nil {
		log.Fatalln("failed to open public keyring:", err)
	}
	defer pubRingFile.Close()
	pubRing, err := openpgp.ReadKeyRing(pubRingFile)
	if err != nil {
		log.Fatalln("failed to read the public keyring:", err)
	}
	_ = pubRing

	prvRingFile, err := os.Open(filepath.Join(config.GPG.Home, "secring.gpg"))
	if err != nil {
		log.Fatalln("failed to open the private keyring:", err)
	}
	defer prvRingFile.Close()
	prvRing, err := openpgp.ReadKeyRing(prvRingFile)
	if err != nil {
		log.Fatalln("failed to read the private keyring:", err)
	}
	var nodeEntity *openpgp.Entity
	for _, entity := range prvRing {
		if entity.PrimaryKey.KeyIdShortString() == config.GPG.ShortKeyID {
			nodeEntity = entity
			break
		}
	}
	if nodeEntity == nil {
		log.Fatalf("could not find %s in the private keyring", config.GPG.ShortKeyID)
	}

	publicKeyFile, err := os.Create(filepath.Join(*ipgsDir, "identity.asc"))
	if err != nil {
		log.Fatalln("failed to create public key file:", err)
	}
	defer publicKeyFile.Close()
	publicEncoder, err := armor.Encode(publicKeyFile, openpgp.PublicKeyType, nil)
	if err != nil {
		log.Fatalln("failed to create armorer for private key:", err)
	}
	nodeEntity.Serialize(publicEncoder)
	publicEncoder.Close()
	publicKeyFile.Close()

	var b bytes.Buffer
	_, err = b.WriteString("IPGS hello world")
	if err != nil {
		log.Fatalln("failed to write hello world:", err)
	}
	sigFile, err := os.Create("/tmp/ipgs-hello-world.sig")
	if err != nil {
		log.Fatalln("failed to create temporary signature file:", err)
	}
	err = openpgp.ArmoredDetachSignText(sigFile, nodeEntity, &b, nil)
	if err != nil {
		log.Fatalln("failed to make signer thing:", err)
	}
	log.Printf("the signature functionality can be checked by invoking `echo -n 'IPGS hello world' | gpg --verify %s -`\n", sigFile.Name())

	fn, err := ipfs_config.Filename(config.IPFS.Path)
	if err != nil {
		log.Fatalln("failed to build IPFS config filename:", err)
	}
	ipfsConfigBytes, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Fatalln("failed to read IPFS config file:", err)
	}
	var ipfsConfig ipfs_config.Config
	err = json.Unmarshal(ipfsConfigBytes, &ipfsConfig)
	if err != nil {
		log.Fatalln("failed to process IPFS config json:", err)
	}
	log.Println("going to try connecting to IPFS node at", ipfsConfig.Addresses.API)

	s := ipfs_shell.NewShell(ipfsConfig.Addresses.API)
	id, err := s.ID()
	if err != nil {
		log.Fatalln("failed to get ID from IPFS node:", err)
	}
	log.Printf("connected to peer ID %s\n", id.ID)
	writeCache("ipfs-node-id", id.ID)

	as, err := LoadCurrentAppSpace(*ipgsDir, config, s)
	if err != nil {
		log.Fatalln("failed to load current app space:", err)
	}
	log.Printf("loaded app-space: %+v\n", as)

	as.LastUpdated = time.Now()

	err = as.Publish(*ipgsDir, config, s)
	if err != nil {
		log.Fatalln("failed to publish app space:", err)
	}
	log.Printf("published app-space: %+v\n", as)
}

func initializeNode(nodeDir string) {
	dir := getStringForPromptOrFatal("IPGS node directory", nodeDir)
	if dir != nodeDir {
		log.Println("you will need to set the -ipgs-dir flag for future invocations to", dir)
	}
	dirStats, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("could not get information about %s: %s\n", dir, err)
	}
	if !os.IsNotExist(err) {
		if !dirStats.IsDir() {
			log.Fatalf(
				"there is a non-directory already at %s, please delete it or choose a different location for the IPGS node directory\n",
				dir,
			)
		}
		reallyWipe := getBoolForPromptOrFatal(
			fmt.Sprintf("about to delete %s and is contents; proceed?", dir),
			false,
		)
		if !reallyWipe {
			log.Fatalln("aborting.")
		}
		err := os.RemoveAll(dir)
		if err != nil {
			log.Fatalf("could not delete %s: %s\n", dir, err)
		}
	}
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		log.Fatalf("could not create IPGS node directory: %s\n", err)
	}

	var config Config

	needNewKeys := getBoolForPromptOrFatal(
		"create a new OpenPGP keypair for this node?",
		true,
	)
	if needNewKeys {
		gpgPath, err := exec.LookPath("gpg")
		if err != nil {
			log.Fatalln(
				"failed to find gpg on in the search path; IPGS depends on the gpg keychain for key storage:",
				err,
			)
		}
		gpgOk := getBoolForPromptOrFatal(
			fmt.Sprintf("found gpg at %s; ok?", gpgPath),
			true,
		)
		if !gpgOk {
			log.Fatalln("please make sure that the correct gpg executable can be found in your search path")
		}
		name := getStringForPromptOrFatal(
			"OpenPGP Entity Name",
			"",
		)
		comment := getStringForPromptOrFatal(
			"OpenPGP Entity Comment",
			"IPGS Player Identity",
		)
		email := getStringForPromptOrFatal(
			"OpenPGP Entity Email",
			"@ipgs",
		)
		entity, err := openpgp.NewEntity(name, comment, email, nil)
		if err != nil {
			log.Fatalln("failed to create OpenPGP entity", err)
		}
		config.GPG.ShortKeyID = entity.PrimaryKey.KeyIdShortString()
		log.Println("created key", config.GPG.ShortKeyID)
		for _, id := range entity.Identities {
			err := id.SelfSignature.SignUserId(
				id.UserId.Id,
				entity.PrimaryKey,
				entity.PrivateKey,
				nil,
			)
			if err != nil {
				log.Fatalln("failed to selfsign identity:", err)
			}
		}
		privateKeyFile, err := os.Create(filepath.Join(dir, "private.asc"))
		if err != nil {
			log.Fatalln("failed to create private key file:", err)
		}
		defer privateKeyFile.Close()
		err = privateKeyFile.Chmod(0400)
		if err != nil {
			log.Fatalln("failed to set the private key file to read-only:", err)
		}
		privateEncoder, err := armor.Encode(privateKeyFile, openpgp.PrivateKeyType, nil)
		if err != nil {
			log.Fatalln("failed to create armorer for private key:", err)
		}
		entity.SerializePrivate(privateEncoder, nil)
		privateEncoder.Close()
		privateKeyFile.Close()
		c := exec.Command(
			gpgPath,
			"--import",
			privateKeyFile.Name(),
		)
		o, err := c.CombinedOutput()
		if err != nil {
			log.Println("failed to get combined output from gpg command:", err)
		} else {
			log.Printf("captured the following data from gpg:\n\n%s\n", string(o))
		}
		delPrivKey := getBoolForPromptOrFatal(
			"delete the private key file?",
			true,
		)
		if delPrivKey {
			err := os.Remove(privateKeyFile.Name())
			if err != nil {
				log.Println(
					"failed to delete the private key file; please delete it manually at",
					privateKeyFile.Name(),
				)
			} else {
				log.Println("deleted private key file")
			}
		}
	}

	config.IPFS.Path, err = ipfs_config.Path("", "")
	if err != nil {
		log.Println("failed to get the path from IPFS:", err, "; using default")
		config.IPFS.Path = ipfs_config.DefaultPathRoot
	}

	config = getConfigFromUser(config)
	configJSON, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		log.Fatalf("could not marshal config into json: %s\n", err)
	}
	configFile, err := os.Create(filepath.Join(dir, "config.json"))
	defer configFile.Close()
	if err != nil {
		log.Fatalf("could not create config file: %s\n", err)
	}
	configFile.Write(configJSON)
	configFile.WriteString("\n")
}

func loadConfig(nodeDir string) Config {
	configBytes, err := ioutil.ReadFile(filepath.Join(nodeDir, "config.json"))
	if err != nil {
		log.Fatalf("could not read config file: %s\n", err)
	}
	var config Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("could not process config as json: %s\n", err)
	}
	return config
}

var inputScanner *bufio.Scanner

func getStringForPromptOrFatal(p, d string) string {
	if inputScanner == nil {
		inputScanner = bufio.NewScanner(os.Stdin)
	}

	fmt.Printf("%s [%s]: ", p, d)

	inputScanner.Scan()
	err := inputScanner.Err()
	if err != nil {
		log.Fatalf("failed to read input from STDIN: %s\n", err)
	}

	t := inputScanner.Text()
	if t == "" {
		return d
	}
	return t
}

func getBoolForPromptOrFatal(p string, d bool) bool {
	var ds string
	if d {
		ds = "y"
	} else {
		ds = "n"
	}

	s := getStringForPromptOrFatal(
		fmt.Sprintf("%s (y/n)", p),
		ds,
	)
	if s == "y" {
		return true
	}
	return false
}
