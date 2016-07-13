// Package util provides generally helpful functions used by the ipgs system
package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
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

// GetPublicPrivateRings returns the public and private keyrings from the
// gpgHome directory.
func GetPublicPrivateRings(gpgHome string) (openpgp.EntityList, openpgp.EntityList, error) {
	pubRing, err := getRingFromFile(filepath.Join(gpgHome, "pubring.gpg"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get public keyring")
	}

	prvRing, err := getRingFromFile(filepath.Join(gpgHome, "secring.gpg"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get private keyring")
	}

	return pubRing, prvRing, nil
}

func getRingFromFile(filename string) (openpgp.EntityList, error) {
	ringFile, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open keyring file")
	}
	defer ringFile.Close()

	ring, err := openpgp.ReadKeyRing(ringFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read keyring file")
	}

	return ring, nil
}

// FindEntityForKeyId searches the keyring provided for an *openpgp.Entity with
// a primary key short id string equal to the id provided
func FindEntityForKeyId(ring openpgp.EntityList, id string) (*openpgp.Entity, error) {
	var e *openpgp.Entity
	for _, v := range ring {
		if v.PrimaryKey.KeyIdShortString() == id {
			e = v
			break
		}
	}

	if e == nil {
		return nil, errors.Errorf("could not find %s in the keyring", id)
	}

	return e, nil
}

// ArmoredDetachedSignToFile signs the contents of m to a new file at filename
// using the entity e. The signature may be checked with gpg on the command
// line by invoking `echo 'data in the message' | gpg --verify filename -` .
// The echo command may need a -n flag if the message did not have a trailing
// newline.
func ArmoredDetachedSignToFile(e *openpgp.Entity, m io.Reader, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrap(err, "failed to create signature file")
	}
	defer f.Close()

	err = openpgp.ArmoredDetachSignText(f, e, m, nil)
	if err != nil {
		return errors.Wrap(err, "failed to make signer")
	}

	return nil
}

// FatalIfErr uses log.Fatalln to halt execution if err is not nil with the
// message "failed to [note]: [err]"
func FatalIfErr(note string, err error) {
	if err != nil {
		log.Fatalf("failed to %v: %+v\n", note, err)
	}
}
