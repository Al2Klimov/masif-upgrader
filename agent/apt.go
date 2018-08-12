package main

import (
	"bytes"
	"github.com/Al2Klimov/masif-upgrader/common"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type aptBadOutput struct {
	output string
}

func (self *aptBadOutput) Error() string {
	return "Got bad output from apt-get -sqq: " + self.output
}

type apt struct {
	exe string
}

func newApt() (result *apt, err error) {
	path, errGEP := getExePath("apt-get")
	if errGEP != nil {
		return nil, errGEP
	}

	if path != "" {
		result = &apt{exe: path}
	}

	return
}

func (self *apt) whatIfUpgradeAll(critOpRunner criticalOperationRunner) (tasks map[common.PkgMgrTask]struct{}, err error) {
	critOpRunner.runCritical(func() {
		tasks, err = self.whatIf("upgrade")
	})

	return
}

func (self *apt) whatIfUpgrade(critOpRunner criticalOperationRunner, packageName string) (tasks map[common.PkgMgrTask]struct{}, err error) {
	critOpRunner.runCritical(func() {
		tasks, err = self.whatIf("upgrade", packageName)
	})

	return
}

func (self *apt) upgrade(critOpRunner criticalOperationRunner, packageName string) (err error) {
	cmd := exec.Command(self.exe, "-yqq", "upgrade", packageName)

	cmd.Env = []string{"LC_ALL=C", "DEBIAN_FRONTEND=noninteractive", "PATH=" + os.Getenv("PATH")}
	cmd.Dir = "/"
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	critOpRunner.runCritical(func() {
		err = cmd.Run()
	})

	return
}

var aptLineRgx = regexp.MustCompile(`\A([^ ]+) ([^ ]+)(?: \[([^]]+)])?(?: \(([^)]+)\))?\z`)

func (self *apt) whatIf(args ...string) (tasks map[common.PkgMgrTask]struct{}, err error) {
	cmd := exec.Command(self.exe, append([]string{"-sqq"}, args...)...)
	outBuf := bytes.Buffer{}

	cmd.Env = []string{"LC_ALL=C"}
	cmd.Dir = "/"
	cmd.Stdin = nil
	cmd.Stdout = &outBuf
	cmd.Stderr = nil

	if errRun := cmd.Run(); errRun != nil {
		return nil, errRun
	}

	tasks = map[common.PkgMgrTask]struct{}{}

	for _, line := range strings.Split(outBuf.String(), "\n") {
		if line != "" {
			match := aptLineRgx.FindStringSubmatch(line)
			if match == nil {
				return nil, &aptBadOutput{output: line}
			}

			nextTask := common.PkgMgrTask{
				PackageName: match[2],
				FromVersion: match[3],
				ToVersion:   match[4],
			}

			switch match[1] {
			case "Inst":
				if nextTask.ToVersion == "" {
					return nil, &aptBadOutput{output: line}
				}

				if nextTask.FromVersion == "" {
					nextTask.Action = common.PkgMgrInstall
				} else {
					nextTask.Action = common.PkgMgrUpdate
				}

			case "Conf":
				if (nextTask.FromVersion == "") == (nextTask.ToVersion == "") {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.Action = common.PkgMgrConfigure

			case "Remv":
				if nextTask.FromVersion == "" || nextTask.ToVersion != "" {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.Action = common.PkgMgrRemove

			case "Purg":
				if nextTask.FromVersion == "" || nextTask.ToVersion != "" {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.Action = common.PkgMgrPurge
			}

			tasks[nextTask] = struct{}{}
		}
	}

	return
}
