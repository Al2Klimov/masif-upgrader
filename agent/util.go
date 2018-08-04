package main

import "os/exec"

func getExePath(exe string) (path string, err error) {
	path, errLP := exec.LookPath(exe)
	if errLP != nil {
		if errCmd, isErrCmd := errLP.(*exec.Error); isErrCmd {
			if errCmd.Err == exec.ErrNotFound {
				return "", nil
			}
		}

		return "", errLP
	}

	return
}
