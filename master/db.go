package main

import (
	"context"
	"database/sql"
	"github.com/Al2Klimov/masif-upgrader/common"
	"github.com/go-sql-driver/mysql"
	"time"
)

func dbTx(f func(tx *sql.Tx) error) error {
	for {
		errTx := dbTryTx(f)
		if errTx != nil {
			serializationFailure := false

			switch errDb := errTx.(type) {
			case *mysql.MySQLError:
				serializationFailure = errDb.Number == 1213
			}

			if serializationFailure {
				continue
			}
		}

		return errTx
	}
}

func dbTryTx(f func(tx *sql.Tx) error) error {
	tx, errBT := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if errBT != nil {
		return errBT
	}

	if errTx := f(tx); errTx != nil {
		tx.Rollback()
		return errTx
	}

	return tx.Commit()
}

func dbQuery(tx *sql.Tx, query string, args ...interface{}) (result [][]interface{}, err error) {
	rows, errQuery := tx.Query(query, args...)
	if errQuery != nil {
		return nil, errQuery
	}

	types, errCT := rows.ColumnTypes()
	if errCT != nil {
		return nil, errCT
	}

	columns := len(types)
	currentColumns := make([]interface{}, columns)
	result = [][]interface{}{}

	for rows.Next() {
		nextRow := make([]interface{}, columns)
		for i := range currentColumns {
			currentColumns[i] = &nextRow[i]
		}

		if errScan := rows.Scan(currentColumns...); errScan != nil {
			return nil, errScan
		}

		result = append(result, nextRow)
	}

	return
}

var pkgMgrAction2db = map[common.PkgMgrAction]string{
	common.PkgMgrInstall:   "install",
	common.PkgMgrUpdate:    "update",
	common.PkgMgrConfigure: "configure",
	common.PkgMgrRemove:    "remove",
	common.PkgMgrPurge:     "purge",
}

func dbUpdatePendingTasks(agent string, tasks map[common.PkgMgrTask]struct{}) (approvedTasks map[common.PkgMgrTask]struct{}, err error) {
	err = dbTx(func(tx *sql.Tx) error {
		approvedTasksInDb, errDGT := dbGetTasks(tx, nil, 1)
		if errDGT != nil {
			return errDGT
		}

		rows, errQuery := dbQuery(tx, `SELECT id FROM agent WHERE name=?`, agent)
		if errQuery != nil {
			return errQuery
		}

		dbHasAgent := len(rows) > 0
		var dbAgentId int64

		if dbHasAgent {
			dbAgentId = rows[0][0].(int64)

			approvedTasksForAgent, errDGT := dbGetTasks(tx, dbAgentId, 1)
			if errDGT != nil {
				return errDGT
			}

			for task := range approvedTasksForAgent {
				approvedTasksInDb[task] = struct{}{}
			}
		}

		approvedTasks = map[common.PkgMgrTask]struct{}{}
		pendingTasks := map[common.PkgMgrTask]struct{}{}

	CheckTask:
		for task := range tasks {
			for packageName := range map[string]struct{}{task.PackageName: {}, "": {}} {
				for fromVersion := range map[string]struct{}{task.FromVersion: {}, "": {}} {
					for toVersion := range map[string]struct{}{task.ToVersion: {}, "": {}} {
						for action := range map[common.PkgMgrAction]struct{}{task.Action: {}, 255: {}} {
							pseudoTask := common.PkgMgrTask{
								PackageName: packageName,
								FromVersion: fromVersion,
								ToVersion:   toVersion,
								Action:      action,
							}

							if _, isApproved := approvedTasksInDb[pseudoTask]; isApproved {
								approvedTasks[task] = struct{}{}
								continue CheckTask
							}
						}
					}
				}
			}

			pendingTasks[task] = struct{}{}
		}

		if len(pendingTasks) > 0 {
			var pendingTasksForDb map[common.PkgMgrTask]struct{}

			if dbHasAgent {
				pendingTasksInDb, errDGT := dbGetTasks(tx, dbAgentId, 0)
				if errDGT != nil {
					return errDGT
				}

				pendingTasksForDb = map[common.PkgMgrTask]struct{}{}

				for task := range pendingTasks {
					if _, isInDb := pendingTasksInDb[task]; !isInDb {
						pendingTasksForDb[task] = struct{}{}
					}
				}
			} else {
				pendingTasksForDb = pendingTasks
			}

			if len(pendingTasksForDb) > 0 {
				now := time.Now().Unix()

				if dbHasAgent {
					_, errExec := tx.Exec(`UPDATE agent SET mtime=? WHERE id=?`, now, dbAgentId)
					if errExec != nil {
						return errExec
					}
				} else {
					result, errExec := tx.Exec(
						`INSERT INTO agent(name, ctime, mtime) VALUES (?, ?, ?)`,
						agent,
						now,
						now,
					)
					if errExec != nil {
						return errExec
					}

					id, errLII := result.LastInsertId()
					if errLII != nil {
						return errLII
					}

					dbAgentId = id
				}

				insertedPackages := map[string]int64{}

				for task := range pendingTasksForDb {
					var packageId int64

					if id, insertedPackage := insertedPackages[task.PackageName]; insertedPackage {
						packageId = id
					} else {
						rows, errQuery := dbQuery(tx, `SELECT id FROM package WHERE name=?`, task.PackageName)
						if errQuery != nil {
							return errQuery
						}

						if len(rows) > 0 {
							packageId = rows[0][0].(int64)
							insertedPackages[task.PackageName] = packageId
						} else {
							result, errExec := tx.Exec(`INSERT INTO package(name) VALUES (?)`, task.PackageName)
							if errExec != nil {
								return errExec
							}

							id, errLII := result.LastInsertId()
							if errLII != nil {
								return errLII
							}

							packageId = id
							insertedPackages[task.PackageName] = packageId
						}
					}

					var fromVersion interface{} = nil
					if task.FromVersion != "" {
						fromVersion = task.FromVersion
					}

					var toVersion interface{} = nil
					if task.ToVersion != "" {
						toVersion = task.ToVersion
					}

					_, errExec := tx.Exec(
						`
INSERT INTO task(agent, package, from_version, to_version, action, approved)
VALUES (?, ?, ?, ?, ?, 0)
`,
						dbAgentId,
						packageId,
						fromVersion,
						toVersion,
						pkgMgrAction2db[task.Action],
					)
					if errExec != nil {
						return errExec
					}
				}
			}
		}

		return nil
	})

	return
}

var db2pkgMgrAction = map[string]common.PkgMgrAction{
	"install":   common.PkgMgrInstall,
	"update":    common.PkgMgrUpdate,
	"configure": common.PkgMgrConfigure,
	"remove":    common.PkgMgrRemove,
	"purge":     common.PkgMgrPurge,
}

var dbGetTasksQuery = `
SELECT p.name, t.from_version, t.to_version, t.action
FROM task t
LEFT JOIN package p ON p.id=t.package
`

func dbGetTasks(tx *sql.Tx, agent interface{}, approved uint8) (tasks map[common.PkgMgrTask]struct{}, err error) {
	var rows [][]interface{}
	var errQuery error

	if agent == nil {
		rows, errQuery = dbQuery(tx, dbGetTasksQuery+" WHERE t.agent IS NULL AND t.approved=?", approved)
	} else {
		rows, errQuery = dbQuery(tx, dbGetTasksQuery+" WHERE t.agent=? AND t.approved=?", agent, approved)
	}

	if errQuery != nil {
		return nil, errQuery
	}

	tasks = map[common.PkgMgrTask]struct{}{}

	for _, row := range rows {
		nextTask := common.PkgMgrTask{
			PackageName: "",
			FromVersion: "",
			ToVersion:   "",
			Action:      255,
		}

		if row[0] != nil {
			nextTask.PackageName = string(row[0].([]byte))
		}

		if row[1] != nil {
			nextTask.FromVersion = string(row[1].([]byte))
		}

		if row[2] != nil {
			nextTask.ToVersion = string(row[2].([]byte))
		}

		if row[3] != nil {
			nextTask.Action = db2pkgMgrAction[string(row[3].([]byte))]
		}

		tasks[nextTask] = struct{}{}
	}

	return
}
