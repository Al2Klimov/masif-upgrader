package main

import "github.com/Al2Klimov/masif-upgrader/common"

type pkgMgr interface {
	whatIfUpgradeAll() (tasks map[common.PkgMgrTask]struct{}, err error)
	whatIfUpgrade(packageName string) (tasks map[common.PkgMgrTask]struct{}, err error)
	upgrade(packageName string) error
}
