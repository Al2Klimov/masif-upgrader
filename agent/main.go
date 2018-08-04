package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

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
