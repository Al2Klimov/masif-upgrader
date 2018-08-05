package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/Al2Klimov/masif-upgrader/common"
	"gopkg.in/ini.v1"
	"os"
	"time"
)

type settings struct {
	interval struct {
		check, report int64
	}
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

	var errWIUA error
	var tasks map[common.PkgMgrTask]struct{} = nil
	approvedTasks := map[common.PkgMgrTask]struct{}{}
	interval := struct{ check, report time.Duration }{
		check:  time.Duration(cfg.interval.check) * time.Second,
		report: time.Duration(cfg.interval.report) * time.Second,
	}

	for {
		if tasks == nil {
			if tasks, errWIUA = ourPkgMgr.whatIfUpgradeAll(); errWIUA != nil {
				return errWIUA
			}
		}

		if len(tasks) > 0 {
			notApprovedTasks := map[common.PkgMgrTask]struct{}{}

			for task := range tasks {
				if _, isApproved := approvedTasks[task]; !isApproved {
					notApprovedTasks[task] = struct{}{}
				}
			}

			if len(notApprovedTasks) > 0 {
				tasks = nil

				freshApprovedTasks, errRT := master.reportTasks(notApprovedTasks)
				if errRT != nil {
					return errRT
				}

				for task := range freshApprovedTasks {
					approvedTasks[task] = struct{}{}
				}
			}

			for {
				if tasks == nil {
					if tasks, errWIUA = ourPkgMgr.whatIfUpgradeAll(); errWIUA != nil {
						return errWIUA
					}
				}

				nextPackage := ""
				actionsNeeded := ^uint64(0)

			PossibleActions:
				for task := range tasks {
					if _, isApproved := approvedTasks[task]; isApproved && task.Action == common.PkgMgrUpdate {
						tasks = nil

						tasksOnUpgrade, errWIU := ourPkgMgr.whatIfUpgrade(task.PackageName)
						if errWIU != nil {
							return errWIU
						}

						for taskOnUpgrade := range tasksOnUpgrade {
							if _, approved := approvedTasks[taskOnUpgrade]; !approved {
								continue PossibleActions
							}
						}

						actionsNeededForUpgrade := uint64(len(tasksOnUpgrade))
						if actionsNeededForUpgrade < actionsNeeded {
							actionsNeeded = actionsNeededForUpgrade
							nextPackage = task.PackageName
						}
					}
				}

				if nextPackage == "" {
					break
				}

				tasks = nil

				if errU := ourPkgMgr.upgrade(nextPackage); errU != nil {
					return errU
				}
			}

			if tasks == nil {
				if tasks, errWIUA = ourPkgMgr.whatIfUpgradeAll(); errWIUA != nil {
					return errWIUA
				}
			}

			if len(tasks) > 0 {
				tasks = nil
				time.Sleep(interval.report)
			} else {
				approvedTasks = map[common.PkgMgrTask]struct{}{}
			}
		} else {
			approvedTasks = map[common.PkgMgrTask]struct{}{}
			tasks = nil
			time.Sleep(interval.check)
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

	cfgInterval := cfg.Section("interval")
	cfgTls := cfg.Section("tls")
	result := &settings{
		interval: struct{ check, report int64 }{
			check:  cfgInterval.Key("check").MustInt64(),
			report: cfgInterval.Key("report").MustInt64(),
		},
		master: struct{ host string }{
			host: cfg.Section("master").Key("host").String(),
		},
		tls: struct{ cert, key, ca string }{
			cert: cfgTls.Key("cert").String(),
			key:  cfgTls.Key("key").String(),
			ca:   cfgTls.Key("ca").String(),
		},
	}

	if result.interval.check <= 0 {
		return nil, errors.New("config: interval.check missing")
	}

	if result.interval.report <= 0 {
		return nil, errors.New("config: interval.report missing")
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
