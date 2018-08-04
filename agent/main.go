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
	tls struct {
		cert, key, ca string
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

	master, errNA := newApi(cfg.master.host, cfg.tls)
	if errNA != nil {
		return errNA
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

	approvedTasks, errRT := master.reportTasks(tasks)
	if errRT != nil {
		return errRT
	}

	for {
		tasks, errWIUA := ourPkgMgr.whatIfUpgradeAll()
		if errWIUA != nil {
			return errWIUA
		}

		nextPackage := ""
		actionsNeeded := ^uint64(0)

	PossibleActions:
		for task := range tasks {
			if _, isApproved := approvedTasks[task]; isApproved && task.action == pkgMgrUpdate {
				tasksOnUpgrade, errWIU := ourPkgMgr.whatIfUpgrade(task.packageName)
				if errWIU != nil {
					return errWIU
				}

				for taskOnUpgrade := range tasksOnUpgrade {
					if _, approved := approvedTasks[taskOnUpgrade]; !approved {
						continue PossibleActions
					}
				}

				if actionsNeededForUpgrade := uint64(len(tasksOnUpgrade)); actionsNeededForUpgrade < actionsNeeded {
					actionsNeeded = actionsNeededForUpgrade
					nextPackage = task.packageName
				}
			}
		}

		if nextPackage == "" {
			break
		}

		if errU := ourPkgMgr.upgrade(nextPackage); errU != nil {
			return errU
		}
	}

	return nil
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
		master: struct{ host string }{
			host: cfg.Section("master").Key("host").String(),
		},
		tls: struct{ cert, key, ca string }{
			cert: cfgTls.Key("cert").String(),
			key:  cfgTls.Key("key").String(),
			ca:   cfgTls.Key("ca").String(),
		},
	}

	if result.master.host == "" {
		return nil, errors.New("config: master.host missing")
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
