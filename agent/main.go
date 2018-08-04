package main

import (
	"errors"
	"flag"
	"fmt"
	ini "gopkg.in/ini.v1"
	"os"
)

type settings struct {
	master struct {
		host string
	}
}

func main() {
	if err := runAgent(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runAgent() error {
	cfg, errLC := loadCfg()
	if errLC != nil {
		return errLC
	}

	ourPkgMgr, errPM := newApt()
	if errPM != nil {
		return errPM
	}

	if ourPkgMgr == nil {
		return errors.New("package manager not available or not supported")
	}

	tasks, errWIUA := ourPkgMgr.whatIfUpgradeAll()
	if errWIUA != nil {
		return errWIUA
	}

	approvedTasks, errRT := (&api{host: cfg.master.host}).reportTasks(tasks)
	if errRT != nil {
		return errRT
	}

	_, errPF := fmt.Printf("%#v", approvedTasks)
	return errPF
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

	result := &settings{
		master: struct{ host string }{
			host: cfg.Section("master").Key("host").String(),
		},
	}

	if result.master.host == "" {
		return nil, errors.New("config: master.host missing")
	}

	return result, nil
}
