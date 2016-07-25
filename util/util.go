// Package util provides generally helpful functions used by the ipgs system
package util

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/apiarian/go-ipgs/cachedshell"
	"github.com/pkg/errors"
)

var inputScanner *bufio.Scanner

// GetStringForPrompt writes a prompt to STDOUT and returns the string entered
// into STDIN. If the user does not enter anything, the default string is
// returned.
func GetStringForPrompt(prompt, def string) (string, error) {
	if inputScanner == nil {
		inputScanner = bufio.NewScanner(os.Stdin)
	}

	fmt.Printf("%s [%s]: ", prompt, def)

	inputScanner.Scan()
	err := inputScanner.Err()
	if err != nil {
		return def, errors.Wrap(err, "failed to read from STDIN")
	}

	t := inputScanner.Text()
	if t == "" {
		return def, nil
	}
	return t, nil
}

// GetBoolForPrompt writes a prompt to STDOUT and expects a "y" or a "n" in
// response. Returns true if "y" was entered, otherwise returns false.
func GetBoolForPrompt(prompt string, def bool) (bool, error) {
	var ds string
	if def {
		ds = "y"
	} else {
		ds = "n"
	}

	s, err := GetStringForPrompt(
		fmt.Sprintf("%s (y/n)", prompt),
		ds,
	)
	if s == "y" {
		return true, err
	}
	return false, err
}

// GetIntForPromt writes a prompt to STDOUT and expects an integer in response.
// Returns the input converted to an integer if possible, or an error if not.
func GetIntForPrompt(prompt string, def int) (int, error) {
	ds := strconv.Itoa(def)

	s, err := GetStringForPrompt(
		prompt,
		ds,
	)
	if err != nil {
		return def, err
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return def, errors.Wrap(err, "failed to parse string input to integer")
	}

	return i, nil
}

// FatalIfErr uses log.Fatalln to halt execution if err is not nil with the
// message "failed to [note]: [err]"
func FatalIfErr(note string, err error) {
	if err != nil {
		log.Fatalf("failed to %v: %+v\n", note, err)
	}
}

// FindIpgsHash looks for an IPGS state structure under a node's IPNS root
func FindIpgsHash(nodeId string, s *cachedshell.Shell) (string, error) {
	ipfsHash, err := s.Resolve(fmt.Sprintf("/ipns/%s", nodeId))
	if err != nil {
		return "", errors.Wrapf(err, "could not resolve %s", nodeId)
	}

	stateHash, err := s.ResolvePath(fmt.Sprintf("%s/interplanetary-game-system", ipfsHash))
	if err != nil {
		return "", errors.Wrapf(err, "no IPGS object for node %s", nodeId)
	}

	return stateHash, nil
}
