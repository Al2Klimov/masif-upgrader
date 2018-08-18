package main

import (
	"context"
	"database/sql"
	"github.com/Al2Klimov/masif-upgrader/common"
	"github.com/go-sql-driver/mysql"
	"strings"
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
					pendingTasksForDb[task] = struct{}{}
				}

				pendingTasksOutDb := map[common.PkgMgrTask]struct{}{}

				for task := range pendingTasksInDb {
					if _, exists := pendingTasksForDb[task]; exists {
						delete(pendingTasksForDb, task)
					} else {
						pendingTasksOutDb[task] = struct{}{}
					}
				}

				if errDT := dbDeleteTasks(tx, dbAgentId, 0, pendingTasksOutDb); errDT != nil {
					return errDT
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
		} else if dbHasAgent {
			_, errExec := tx.Exec(`DELETE FROM task WHERE agent=? AND approved=0`, dbAgentId)
			return errExec
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

func dbDeleteTasks(tx *sql.Tx, agent interface{}, approved uint8, tasks map[common.PkgMgrTask]struct{}) error {
	if len(tasks) > 0 {
		filterBase := map[string]map[common.PkgMgrAction]map[string]map[string]struct{}{}

		for task := range tasks {
			if _, exists := filterBase[task.PackageName]; !exists {
				filterBase[task.PackageName] = map[common.PkgMgrAction]map[string]map[string]struct{}{}
			}

			filterPackage := filterBase[task.PackageName]
			if _, exists := filterPackage[task.Action]; !exists {
				filterPackage[task.Action] = map[string]map[string]struct{}{}
			}

			filterAction := filterPackage[task.Action]
			if _, exists := filterAction[task.FromVersion]; !exists {
				filterAction[task.FromVersion] = map[string]struct{}{}
			}

			filterAction[task.FromVersion][task.ToVersion] = struct{}{}
		}

		type subFilter struct {
			filter string
			values []interface{}
		}

		subFilters0 := make(map[string]subFilter, len(filterBase))
		valuesLen0 := 2

		for packageName, actions := range filterBase {
			subFilters1 := make(map[common.PkgMgrAction]subFilter, len(actions))
			valuesLen1 := 0

			for action, fromVersions := range actions {
				subFilters2 := make(map[string]subFilter, len(fromVersions))
				valuesLen2 := 0

				for fromVersion, toVersions := range fromVersions {
					_, toVersionHasNull := toVersions[""]
					if toVersionHasNull {
						delete(toVersions, "")
					}

					if toVersionHasNull && len(toVersions) < 1 {
						subFilters2[fromVersion] = subFilter{
							filter: "(t.to_version IS NULL)",
							values: []interface{}{},
						}

						valuesLen2++
					} else {
						filters3 := make([]string, len(toVersions))
						values3 := make([]interface{}, len(toVersions))
						filterIdx3 := 0

						for toVersion := range toVersions {
							filters3[filterIdx3] = "?"
							values3[filterIdx3] = toVersion
							filterIdx3++
						}

						filter3 := "(t.to_version IN (" + strings.Join(filters3, ",") + "))"

						if toVersionHasNull {
							filter3 = "(CASE WHEN (t.to_version IS NULL) THEN 1 ELSE " + filter3 + " END)"
						}

						subFilters2[fromVersion] = subFilter{
							filter: filter3,
							values: values3,
						}

						valuesLen2 += 1 + len(values3)
					}
				}

				fromVersionNullFilter, fromVersionHasNull := subFilters2[""]
				if fromVersionHasNull {
					delete(subFilters2, "")
					valuesLen2--
				}

				valuesLen1 += 1 + valuesLen2

				var filter2 string
				var values2 []interface{}

				if fromVersionHasNull && len(subFilters2) < 1 {
					filter2 = "0"
					values2 = fromVersionNullFilter.values
				} else {
					filters2 := make([]string, len(subFilters2))
					filterIdx2 := 0
					values2 = make([]interface{}, valuesLen2)
					valueIdx2 := 0

					if fromVersionHasNull {
						copy(values2[valueIdx2:], fromVersionNullFilter.values)
						valueIdx2 += len(fromVersionNullFilter.values)
					}

					for fromVersion, filter := range subFilters2 {
						filters2[filterIdx2] = "WHEN ? THEN " + filter.filter
						filterIdx2++

						values2[valueIdx2] = fromVersion
						valueIdx2++

						copy(values2[valueIdx2:], filter.values)
						valueIdx2 += len(filter.values)
					}

					filter2 = "(CASE t.from_version " + strings.Join(filters2, " ") + " ELSE 0 END)"
				}

				if fromVersionHasNull {
					filter2 = "(CASE WHEN (t.from_version IS NULL) THEN " + fromVersionNullFilter.filter + " ELSE " + filter2 + " END)"
				}

				subFilters1[action] = subFilter{
					filter: filter2,
					values: values2,
				}
			}

			valuesLen0 += 1 + valuesLen1

			filters1 := make([]string, len(subFilters1))
			filterIdx1 := 0
			values1 := make([]interface{}, valuesLen1)
			valueIdx1 := 0

			for action, filter := range subFilters1 {
				filters1[filterIdx1] = "WHEN ? THEN " + filter.filter
				filterIdx1++
				values1[valueIdx1] = pkgMgrAction2db[action]
				valueIdx1++

				copy(values1[valueIdx1:], filter.values)
				valueIdx1 += len(filter.values)
			}

			subFilters0[packageName] = subFilter{
				filter: "(CASE t.action " + strings.Join(filters1, " ") + " ELSE 0 END)",
				values: values1,
			}
		}

		filters0 := make([]string, len(subFilters0))
		filterIdx0 := 0
		values0 := make([]interface{}, valuesLen0)
		valueIdx0 := 2

		values0[0] = agent
		values0[1] = approved

		for packageName, filter := range subFilters0 {
			filters0[filterIdx0] = "WHEN (SELECT p.id FROM package p WHERE p.name=?) THEN " + filter.filter
			filterIdx0++
			values0[valueIdx0] = packageName
			valueIdx0++

			copy(values0[valueIdx0:], filter.values)
			valueIdx0 += len(filter.values)
		}

		_, errExec := tx.Exec(
			`DELETE t FROM task t WHERE t.agent=? AND t.approved=? AND (CASE t.package `+strings.Join(filters0, " ")+` ELSE 0 END)`,
			values0...,
		)
		return errExec
	}

	return nil
}
