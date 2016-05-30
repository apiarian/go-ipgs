package main

import (
	"fmt"
	"log"
	"os/user"
	"reflect"
	"strings"
)

// Config describes the configuration for an IPGS node. It contains various
// subsections defined by other structs.
type Config struct {
	// GPG is the GPG Configuraiton section for the IPGS node.
	GPG GpgConfig
}

// GpgConfig describes the GPG Configuration section for an IPGS node. It
// contains the various parameters required for accessing the GPG components of
// the system
type GpgConfig struct {
	// Home is the GnuPG home directory. This is usually the ~/.gnupg/ It should
	// be listed in the Home: section of the output when running `gpg --version`
	Home string `description:"the home gpg key directory" default:"~/.gnupg/"`
	// ShortKeyID is the short string version of the GPG key that will be used by
	// this node. Both the public and the private halves of this key must be in
	// the GnuPG home keyrings.
	ShortKeyID string `description:"the short ID of the node key" required:"true"`
}

func getConfigFromUser(config Config) Config {
	cv := reflect.ValueOf(&config).Elem()
	ct := reflect.TypeOf(config)
	for i := 0; i < cv.NumField(); i++ {
		sv := cv.Field(i)
		st := ct.Field(i)
		for j := 0; j < sv.NumField(); j++ {
			pv := sv.Field(j)
			pt := st.Type.Field(j)
			d := pt.Tag.Get("default")
			if d == "" {
				d = fmt.Sprintf("%s", pv)
			}
			if strings.HasPrefix(d, "~/") {
				u, err := user.Current()
				if err != nil {
					log.Fatalln("failed to detect user:", err)
				}
				d = strings.Replace(d, "~/", u.HomeDir+"/", 1)
			}
			v := getStringForPromptOrFatal(
				fmt.Sprintf(
					"%s - %s (%s)",
					st.Name,
					pt.Name,
					pt.Tag.Get("description"),
				),
				d,
			)
			if pt.Tag.Get("required") == "true" && v == "" {
				log.Fatalf(
					"missing required value for property %s",
					pt.Name,
				)
			}
			switch pv.Kind() {
			case reflect.String:
				pv.SetString(v)
			default:
				log.Fatalf(
					"do not know how to deal with reflect kind %s\n",
					pv.Kind(),
				)
			} // switch pv kind
		} // for j over section fields
	} // for i over config sections
	return config
}
