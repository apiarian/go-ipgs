// Package cachedshell adds some helpful caching functionality to the go-ipfs-api shell
package cachedshell

import (
	"fmt"
	"os"

	shell "github.com/apiarian/go-ipfs-api"
	"github.com/apiarian/go-ipgs/cache"
	"github.com/pkg/errors"
)

// Shell embeds the go-ipfs-api shell with an extra *Cache
type Shell struct {
	*shell.Shell
	Cache *cache.Cache
}

// NewShell takes the IPFS API url and an existing cache and returns a *Shell
func NewShell(url string, c *cache.Cache) *Shell {
	return &Shell{shell.NewShell(url), c}
}

// AddPermanentFile adds a file by its filename to IPFS returns the resulting
// object's hash. The hash is cached on a per-filename basis, so future calls to
// this method simply return the cached version of the hash for the same
// filename, assuming that the file has not changed.
func (s *Shell) AddPermanentFile(filename string) (string, error) {
	key := fmt.Sprintf("permanent-file-%s", filename)

	hash, err := s.Cache.ReadString(key)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %s from cache", filename)
	}
	if hash != "" {
		return hash, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open %s", filename)
	}
	defer file.Close()

	hash, err = s.Add(file)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add %s", filename)
	}

	s.Cache.Write(key, hash)

	return hash, nil
}

// ClearPermanentFile clears the cache value for the pernanent file hash
// previously added by AddPermanentFile.
func (s *Shell) ClearPermanentFile(filename string) {
	key := fmt.Sprintf("permanent-file-%s", filename)

	s.Cache.Clear(key)
}
