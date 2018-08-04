package main

import (
	"encoding/json"
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

var apiPkgMgrActions = map[pkgMgrAction]string{
	pkgMgrInstall:   "install",
	pkgMgrUpdate:    "update",
	pkgMgrConfigure: "configure",
	pkgMgrRemove:    "remove",
	pkgMgrPurge:     "purge",
}

func runAgent() error {
	_, errLC := loadCfg()
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

	apiTasks := make([]interface{}, len(tasks))
	apiTaskIdx := 0

	for task := range tasks {
		record := map[string]interface{}{
			"package": task.packageName,
			"action":  apiPkgMgrActions[task.action],
		}

		if task.fromVersion != "" {
			record["from_version"] = task.fromVersion
		}

		if task.toVersion != "" {
			record["to_version"] = task.toVersion
		}

		apiTasks[apiTaskIdx] = record
		apiTaskIdx++
	}

	jsn, errJM := json.Marshal(apiTasks)
	if errJM != nil {
		return errJM
	}

	_, errPL := fmt.Println(string(jsn))
	return errPL
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
