package main

import (
	"database/sql"
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
	db struct {
		typ, dsn string
	}
}

var db *sql.DB = nil

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

	var errDB error
	if db, errDB = sql.Open(cfg.db.typ, cfg.db.dsn); errDB != nil {
		return errDB
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
	cfgDb := cfg.Section("db")
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
		db: struct{ typ, dsn string }{
			typ: cfgDb.Key("type").String(),
			dsn: cfgDb.Key("dsn").String(),
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

	if result.db.typ == "" {
		return nil, errors.New("config: db.type missing")
	}

	if result.db.dsn == "" {
		return nil, errors.New("config: db.dsn missing")
	}

	return result, nil
}
