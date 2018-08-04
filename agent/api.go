package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type badHttpStatus struct {
	status int
}

func (self *badHttpStatus) Error() string {
	return fmt.Sprintf("bad HTTP response status %d (expected 200)", self.status)
}

type badHttpBody struct {
	body []byte
}

func (self *badHttpBody) Error() string {
	return "bad HTTP response: " + string(self.body)
}

type api struct {
	host string
}

var pkgMgrActions2api = map[pkgMgrAction]string{
	pkgMgrInstall:   "install",
	pkgMgrUpdate:    "update",
	pkgMgrConfigure: "configure",
	pkgMgrRemove:    "remove",
	pkgMgrPurge:     "purge",
}

var api2pkgMgrActions = map[string]pkgMgrAction{
	"install":   pkgMgrInstall,
	"update":    pkgMgrUpdate,
	"configure": pkgMgrConfigure,
	"remove":    pkgMgrRemove,
	"purge":     pkgMgrPurge,
}

func (self *api) reportTasks(tasks map[pkgMgrTask]struct{}) (approvedTasks map[pkgMgrTask]struct{}, err error) {
	apiTasks := make([]interface{}, len(tasks))
	apiTaskIdx := 0

	for task := range tasks {
		record := map[string]interface{}{
			"package": task.packageName,
			"action":  pkgMgrActions2api[task.action],
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
		return nil, errJM
	}

	res, errPost := http.Post(
		"https://"+self.host+"/v1/pending-tasks",
		"application/json",
		bytes.NewBuffer(jsn),
	)
	if errPost != nil {
		return nil, errPost
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &badHttpStatus{status: res.StatusCode}
	}

	body, errRA := ioutil.ReadAll(res.Body)
	if errRA != nil {
		return nil, errRA
	}

	var unJson interface{}
	if json.Unmarshal(body, &unJson) != nil {
		return nil, &badHttpBody{body: body}
	}

	approvedTasks = map[pkgMgrTask]struct{}{}

	if rootArray, rootIsArray := unJson.([]interface{}); rootIsArray {
		for _, approvedTask := range rootArray {
			if approvedTaskObject, approvedTaskIsObject := approvedTask.(map[string]interface{}); approvedTaskIsObject {
				nextTask := pkgMgrTask{}

				if packageName, hasPackageName := approvedTaskObject["package"]; hasPackageName {
					packageNameString, packageNameIsString := packageName.(string)
					if packageNameIsString && packageNameString != "" {
						nextTask.packageName = packageNameString
					} else {
						return nil, &badHttpBody{body: body}
					}
				} else {
					return nil, &badHttpBody{body: body}
				}

				if action, hasAction := approvedTaskObject["action"]; hasAction {
					if actionString, actionIsString := action.(string); actionIsString && actionString != "" {
						if validAction, actionIsValid := api2pkgMgrActions[actionString]; actionIsValid {
							nextTask.action = validAction
						} else {
							return nil, &badHttpBody{body: body}
						}
					} else {
						return nil, &badHttpBody{body: body}
					}
				} else {
					return nil, &badHttpBody{body: body}
				}

				if fromVersion, hasFromVersion := approvedTaskObject["from_version"]; hasFromVersion {
					fromVersionString, fromVersionIsString := fromVersion.(string)
					if fromVersionIsString && fromVersionString != "" {
						nextTask.fromVersion = fromVersionString
					} else {
						return nil, &badHttpBody{body: body}
					}
				}

				if toVersion, hasToVersion := approvedTaskObject["to_version"]; hasToVersion {
					toVersionString, toVersionIsString := toVersion.(string)
					if toVersionIsString && toVersionString != "" {
						nextTask.toVersion = toVersionString
					} else {
						return nil, &badHttpBody{body: body}
					}
				}

				var hasVersions bool

				switch nextTask.action {
				case pkgMgrInstall:
					hasVersions = nextTask.fromVersion == "" && nextTask.toVersion != ""
				case pkgMgrUpdate:
					hasVersions = nextTask.fromVersion != "" && nextTask.toVersion != ""
				case pkgMgrConfigure:
					hasVersions = (nextTask.fromVersion == "") != (nextTask.toVersion == "")
				case pkgMgrRemove:
				case pkgMgrPurge:
					hasVersions = nextTask.fromVersion != "" && nextTask.toVersion != ""
				}

				if !hasVersions {
					return nil, &badHttpBody{body: body}
				}

				approvedTasks[nextTask] = struct{}{}
			} else {
				return nil, &badHttpBody{body: body}
			}
		}
	} else {
		return nil, &badHttpBody{body: body}
	}

	return
}
