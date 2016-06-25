// Package cachedshell adds some helpful caching functionality to the go-ipfs-api shell
package cachedshell

import (
	"fmt"
	"os"

	shell "github.com/apiarian/go-ipfs-api"
	"github.com/apiarian/go-ipgs/cache"
)

// CachedShell embeds the go-ipfs-api shell with an extra *Cache
type CachedShell struct {
	*shell.Shell
	Cache *cache.Cache
}

// NewCachedShell takes the IPFS API url and an existing cache and returns a *CachedShell
func NewCachedShell(url string, c *cache.Cache) *CachedShell {
	return &CachedShell{shell.NewShell(url), c}
}

// AddPermanentFile adds a file by its filename to IPFS returns the resulting
// object's hash. The hash is cached on a per-filename basis, so future calls to
// this method simply return the cached version of the hash for the same
// filename, assuming that the file has not changed.
func (s *CachedShell) AddPermanentFile(filename string) (string, error) {
	key := fmt.Sprintf("permanent-file-%s", filename)

	hash, err := s.Cache.ReadString(key)
	if err != nil {
		return "", fmt.Errorf("failed to read %s from cache: %s", filename, err)
	}
	if hash != "" {
		return hash, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %s", filename, err)
	}
	defer file.Close()

	hash, err = s.Add(file)
	if err != nil {
		return "", fmt.Errorf("failed to add %s: %s", filename, err)
	}

	s.Cache.Write(key, hash)

	return hash, nil
}

// ClearPermanentFile clears the cache value for the pernanent file hash
// previously added by AddPermanentFile.
func (s *CachedShell) ClearPermanentFile(filename string) {
	key := fmt.Sprintf("permanent-file-%s", filename)

	s.Cache.Clear(key)
}
