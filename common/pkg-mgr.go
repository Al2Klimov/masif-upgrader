package common

type PkgMgrAction uint8

type PkgMgrTask struct {
	PackageName, FromVersion, ToVersion string
	Action                              PkgMgrAction
}

const (
	PkgMgrInstall   PkgMgrAction = 0
	PkgMgrUpdate    PkgMgrAction = 1
	PkgMgrConfigure PkgMgrAction = 2
	PkgMgrRemove    PkgMgrAction = 3
	PkgMgrPurge     PkgMgrAction = 4
)
