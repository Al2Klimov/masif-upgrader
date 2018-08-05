package main

import (
	"errors"
	"flag"
	"fmt"
	"gopkg.in/ini.v1"
	"os"
)

type settings struct {
	api struct {
		listen string
	}
	tls struct {
		cert, key, ca, crl string
	}
}

func main() {
	if err := runMaster(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runMaster() error {
	cfg, errLC := loadCfg()
	if errLC != nil {
		return errLC
	}

	httpd, errNA := newApi(cfg.api.listen, cfg.tls)
	if errNA != nil {
		return errNA
	}

	return httpd.ListenAndServeTLS("", "")
}

func loadCfg() (config *settings, err error) {
	cfgFile := flag.String("config", "", "config file")
	flag.Parse()

	if *cfgFile == "" {
		return nil, errors.New("config file missing")
	}

	cfg, errLI := ini.Load(*cfgFile)
	if errLI != nil {
		return nil, errLI
	}

	cfgTls := cfg.Section("tls")
	result := &settings{
		api: struct{ listen string }{
			listen: cfg.Section("api").Key("listen").String(),
		},
		tls: struct{ cert, key, ca, crl string }{
			cert: cfgTls.Key("cert").String(),
			key:  cfgTls.Key("key").String(),
			ca:   cfgTls.Key("ca").String(),
			crl:  cfgTls.Key("crl").String(),
		},
	}

	if result.api.listen == "" {
		return nil, errors.New("config: api.listen missing")
	}

	if result.tls.cert == "" {
		return nil, errors.New("config: tls.cert missing")
	}

	if result.tls.key == "" {
		return nil, errors.New("config: tls.key missing")
	}

	if result.tls.ca == "" {
		return nil, errors.New("config: tls.ca missing")
	}

	return result, nil
}
