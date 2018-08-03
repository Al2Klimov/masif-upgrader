package main

type pkgMgrAction uint8

type pkgMgrTask struct {
	packageName, fromVersion, toVersion string
	action                              pkgMgrAction
}

type pkgMgr interface {
	whatIfUpgradeAll() (tasks map[pkgMgrTask]struct{}, err error)
	whatIfUpgrade(packageName string) (tasks map[pkgMgrTask]struct{}, err error)
	upgrade(packageName string) error
}

const (
	pkgMgrInstall   pkgMgrAction = 0
	pkgMgrUpdate    pkgMgrAction = 1
	pkgMgrConfigure pkgMgrAction = 2
	pkgMgrRemove    pkgMgrAction = 3
	pkgMgrPurge     pkgMgrAction = 4
)
