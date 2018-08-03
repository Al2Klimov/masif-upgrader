package main

import (
	"bytes"
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

type apt struct{}

func (self *apt) whatIfUpgradeAll() (tasks map[pkgMgrTask]struct{}, err error) {
	return self.whatIf("upgrade")
}

func (self *apt) whatIfUpgrade(packageName string) (tasks map[pkgMgrTask]struct{}, err error) {
	return self.whatIf("upgrade", packageName)
}

func (self *apt) upgrade(packageName string) error {
	cmd := exec.Command("apt-get", "-yqq", "upgrade", packageName)

	cmd.Env = []string{"LC_ALL=C", "DEBIAN_FRONTEND=noninteractive", "PATH=" + os.Getenv("PATH")}
	cmd.Dir = "/"
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

var aptLineRgx = regexp.MustCompile(`\A([^ ]+) ([^ ]+)(?: \[([^]]+)])?(?: \(([^)]+)\))?\z`)

func (self *apt) whatIf(args ...string) (tasks map[pkgMgrTask]struct{}, err error) {
	cmd := exec.Command("apt-get", append([]string{"-sqq"}, args...)...)
	outBuf := bytes.Buffer{}

	cmd.Env = []string{"LC_ALL=C"}
	cmd.Dir = "/"
	cmd.Stdin = nil
	cmd.Stdout = &outBuf
	cmd.Stderr = nil

	if errRun := cmd.Run(); errRun != nil {
		return nil, errRun
	}

	tasks = map[pkgMgrTask]struct{}{}

	for _, line := range strings.Split(outBuf.String(), "\n") {
		if line != "" {
			match := aptLineRgx.FindStringSubmatch(line)
			if match == nil {
				return nil, &aptBadOutput{output: line}
			}

			nextTask := pkgMgrTask{
				packageName: match[2],
				fromVersion: match[3],
				toVersion:   match[4],
			}

			switch match[1] {
			case "Inst":
				if nextTask.toVersion == "" {
					return nil, &aptBadOutput{output: line}
				}

				if nextTask.fromVersion == "" {
					nextTask.action = pkgMgrInstall
				} else {
					nextTask.action = pkgMgrUpdate
				}

			case "Conf":
				if (nextTask.fromVersion == "") == (nextTask.toVersion == "") {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.action = pkgMgrConfigure

			case "Remv":
				if nextTask.fromVersion == "" || nextTask.toVersion != "" {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.action = pkgMgrRemove

			case "Purg":
				if nextTask.fromVersion == "" || nextTask.toVersion != "" {
					return nil, &aptBadOutput{output: line}
				}

				nextTask.action = pkgMgrPurge
			}

			tasks[nextTask] = struct{}{}
		}
	}

	return
}
