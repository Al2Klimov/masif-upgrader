<?php

namespace Icinga\Module\Masifupgrader\Forms\Pending;

use Icinga\Module\Masifupgrader\Web\DbAwareFormTrait;
use Icinga\Module\Masifupgrader\Web\HeadlessFormTrait;
use Icinga\Web\Form;
use Zend_View_Interface;

class PackagesForm extends Form
{
    use DbAwareFormTrait;
    use HeadlessFormTrait;

    /**
     * @var array
     */
    protected $tasks;

    /**
     * @var array
     */
    protected $agents;

    /**
     * @var bool
     */
    protected $holdOn = false;

    /**
     * @return array
     */
    protected function getTasks()
    {
        if ($this->tasks === null) {
            $rawTasks = $this->fetchAll(
                <<<EOQ
SELECT (SELECT p.name FROM package p WHERE p.id=t1.package) AS package, t1.action, t1.to_version,
  (SELECT COUNT(DISTINCT t2.agent) FROM task t2 WHERE t2.approved=0 AND t2.package=t1.package) AS agents
FROM task t1
WHERE t1.approved=0
GROUP BY t1.package, t1.action, t1.to_version
ORDER BY (SELECT COUNT(DISTINCT t2.agent) FROM task t2 WHERE t2.approved=0 AND t2.package=t1.package) DESC,
  (SELECT p.name FROM package p WHERE p.id=t1.package) ASC
EOQ
            );

            $this->tasks = [];

            foreach ($rawTasks as list($package, $action, $toVersion, $agents)) {
                $this->tasks[$package][0] = $agents;
                $this->tasks[$package][1][$action][$toVersion] = null;
            }
        }

        return $this->tasks;
    }

    /**
     * @param array $filter
     *
     * @return array
     */
    protected function getAgents($filter = [])
    {
        if ($this->agents === null) {
            if (empty($filter)) {
                return $this->agents = [];
            }

            list($packageFilters, $params) = $this->filterTasksByActions('t2', 'p', $filter);

            $rawAgents = $this->fetchAll(
                <<<EOQ
SELECT a.name, (SELECT COUNT(DISTINCT t1.package) FROM task t1 WHERE t1.agent=a.id AND t1.approved=0)
FROM agent a
WHERE a.id IN (SELECT t2.agent FROM task t2 WHERE t2.approved=0 AND ($packageFilters))
ORDER BY (SELECT COUNT(DISTINCT t1.package) FROM task t1 WHERE t1.agent=a.id AND t1.approved=0) DESC, a.name ASC
EOQ
                ,
                $params
            );

            $this->agents = [];

            foreach ($rawAgents as list($agent, $packages)) {
                $this->agents[$agent] = $packages;
            }
        }

        return $this->agents;
    }

    /**
     * @param string $tasksTableAlias
     * @param string $packagesTableAlias
     * @param string $filter
     *
     * @return array
     */
    protected function filterTasksByActions($tasksTableAlias, $packagesTableAlias, $filter)
    {
        $t = $tasksTableAlias;
        $p = $packagesTableAlias;
        $params = [];
        $packageFilters = [];

        foreach ($filter as $package => $actions) {
            $params[] = $package;
            $actionFilters = [];

            foreach ($actions as $action => $toVersions) {
                $params[] = $action;

                $toVersionHasNull = false;
                $toVersionsNotNull = 0;

                foreach ($toVersions as $toVersion => $_) {
                    if ($toVersion === '') {
                        $toVersionHasNull = true;
                    } else {
                        ++$toVersionsNotNull;
                        $params[] = $toVersion;
                    }
                }

                $toVersionFilters = [];

                if ($toVersionHasNull) {
                    $toVersionFilters[] = "$t.to_version IS NULL";
                }

                if ($toVersionsNotNull) {
                    $toVersionFilters[] = "$t.to_version IN (" . implode(',', array_fill(0, $toVersionsNotNull, '?')) . ')';
                }

                $actionFilters[] = "($t.action=? AND (" . implode(' OR ', $toVersionFilters) . '))';
            }

            $packageFilters[] = "($t.package=(SELECT $p.id FROM package $p WHERE $p.name=?) AND (" . implode(' OR ', $actionFilters) . '))';
        }

        return [implode(' OR ', $packageFilters), $params];
    }

    public function init()
    {
        $this->setName('form_pending_packages');
        $this->setSubmitLabel($this->translate('Approve selection'));
    }

    public function createElements(array $formData)
    {
        $agentFilter = [];

        foreach ($this->getTasks() as $package => list($agents, $actions)) {
            foreach ($actions as $action => $toVersions) {
                foreach ($toVersions as $toVersion => $_) {
                    $checkboxName = implode('_', [bin2hex($package), $action, bin2hex($toVersion)]);
                    $this->addHeadlessElement('checkbox', $checkboxName, []);

                    if (isset($formData[$checkboxName]) && $formData[$checkboxName]) {
                        $agentFilter[$package][$action][$toVersion] = null;
                    }
                }
            }
        }

        if (($this->holdOn = isset($formData['filter_agents_first']) && $formData['filter_agents_first'])
            || isset($formData['filter_agents'])) {
            $this->addElement('hidden', 'filter_agents', ['value' => '1']);

            foreach ($this->getAgents($agentFilter) as $agent => $packages) {
                $this->addHeadlessElement('checkbox', 'agent_' . bin2hex($agent), []);
            }
        } else {
            $this->addHeadlessElement('checkbox', 'filter_agents_first', []);
        }
    }

    public function render(Zend_View_Interface $view = null)
    {
        if ($view === null) {
            $view = $this->getView();
        }

        $t1header1 = $this->translate('Package, agents awaiting approval');
        $t1header2 = $this->translate('Action');
        $t1header3 = $this->translate('Target version');
        $t1checkbox1 = $this->translate('Select specific agents first');

        $result = $this->renderHeadless($view)
            . "<table class='common-table'><thead><tr><th colspan='3'>{$view->escape($t1header1)}</th>"
            . "<th>{$view->escape($t1header2)}</th>"
            . "<th colspan='3'>{$view->escape($t1header3)}</th></tr></thead><tbody>";

        $rows = [];
        $currentRow = 0;

        $actionLabels = [
            'install'   => $this->translate('Install'),
            'update'    => $this->translate('Update'),
            'configure' => $this->translate('Configure'),
            'remove'    => $this->translate('Remove'),
            'purge'     => $this->translate('Purge')
        ];

        $actionsOrder = array_flip(array_keys($actionLabels));

        foreach ($this->getTasks() as $package => list($agents, $actions)) {
            $packageRows = 0;
            $packageOnRow = $currentRow;

            uksort($actions, function($lhs, $rhs) use($actionsOrder) {
                return $actionsOrder[$lhs] - $actionsOrder[$rhs];
            });

            foreach ($actions as $action => $toVersions) {
                $actionOnRow = $currentRow;

                krsort($toVersions, SORT_NATURAL);

                foreach ($toVersions as $toVersion => $_) {
                    if ($toVersion === '') {
                        $toVersion = $this->translate('N/A');
                    }

                    $approve = $this->getElement(implode('_', [bin2hex($package), $action, bin2hex($toVersion)]));

                    $rows[] = [
                        null,
                        null,
                        null,
                        "<td colspan='3'>{$approve->render($view)}<label for='{$view->escape($approve->getId())}'>&emsp;{$view->escape($toVersion)}</label></td>"
                    ];

                    ++$currentRow;
                }

                $actionRows = count($toVersions);
                $packageRows += $actionRows;
                $rows[$actionOnRow][2] = "<td rowspan='$actionRows'>{$view->escape($actionLabels[$action])}</td>";
            }

            $rows[$packageOnRow][0] = "<td rowspan='$packageRows' colspan='2'>{$view->escape($package)}</td>";
            $rows[$packageOnRow][1] = "<td rowspan='$packageRows'>$agents</td>";
        }

        foreach ($rows as $row) {
            $result .= '<tr>' . implode('', $row) . '</tr>';
        }

        if ($this->getElement('filter_agents') === null) {
            $filterAgents = $this->getElement('filter_agents_first');

            $result .= "<tr><td rowspan='2' colspan='4'></td><td colspan='3'>{$filterAgents->render($view)}"
                . "<label for='{$view->escape($filterAgents->getId())}'>&emsp;{$view->escape($t1checkbox1)}</label></td></tr>"
                . "<tr><td colspan='3'>{$this->getElement('btn_submit')->render($view)}</td></tr></tbody></table>";
        } else {
            $t2header1 = $this->translate('Agent, pending packages');
            $result .= "</tbody></table><table class='common-table'><thead><tr><th colspan='2'>{$view->escape($t2header1)}</th></tr></thead><tbody>";

            foreach ($this->getAgents() as $agent => $packages) {
                $approve = $this->getElement('agent_' . bin2hex($agent));
                $result .= "<tr><td>{$approve->render($view)}<label for='{$view->escape($approve->getId())}'>&emsp;{$view->escape($agent)}</label></td><td>$packages</td></tr>";
            }

            $result .= "<tr><td colspan='2'>{$this->getElement('btn_submit')->render($view)}</td></tr></tbody></table>";
        }

        return $result;
    }

    public function onSuccess()
    {
        if ($this->holdOn) {
            return false;
        }

        $agentsWildcard = $this->getElement('filter_agents') === null;

        if ($agentsWildcard) {
            $agentFilter = '';
            $params = [];
        } else {
            $params = array_keys($this->getAgents());
            if (empty($params)) {
                return false;
            }

            $agentFilter = ' AND t.agent IN (SELECT a.id FROM agent a WHERE a.name IN (' . implode(',', array_fill(0, count($params), '?')) . '))';
        }

        $taskFilter = [];

        foreach ($this->getTasks() as $package => list($agents, $actions)) {
            foreach ($actions as $action => $toVersions) {
                foreach ($toVersions as $toVersion => $_) {
                    /** @var \Zend_Form_Element_Checkbox $checkbox */
                    $checkbox = $this->getElement(implode('_', [bin2hex($package), $action, bin2hex($toVersion)]));

                    if ($checkbox->isChecked()) {
                        $taskFilter[$package][$action][$toVersion] = null;
                    }
                }
            }
        }

        if (empty($taskFilter)) {
            return false;
        }

        list($packageFilters, $packageFilterParams) = $this->filterTasksByActions('t', 'p2', $taskFilter);

        $filter = "t.approved=0$agentFilter AND ($packageFilters)";
        $params = array_merge($params, $packageFilterParams);

        $this->transaction(function() use($filter, $params, $agentsWildcard) {
            $pending = $this->fetchAll(
                "SELECT t.agent, t.package, t.action, t.to_version FROM task t WHERE $filter",
                $params
            );

            if (empty($pending)) {
                return;
            }

            $this->execSql("DELETE t FROM task t WHERE $filter", $params);

            $approvals = [];

            foreach ($pending as list($agent, $package, $action, $toVersion)) {
                if ($agentsWildcard) {
                    $agent = null;
                }

                $approvals[$agent][$package][$action][$toVersion] = null;
            }

            $insertParams = [];
            $rows = 0;

            foreach ($approvals as $agent => $tasks) {
                if ($agent === '') {
                    $agent = null;
                }

                foreach ($tasks as $package => $actions) {
                    foreach ($actions as $action => $toVersions) {
                        foreach ($toVersions as $toVersion => $_) {
                            if ($toVersion === '') {
                                $toVersion = null;
                            }

                            ++$rows;

                            $insertParams[] = $agent;
                            $insertParams[] = $package;
                            $insertParams[] = $toVersion;
                            $insertParams[] = $action;
                        }
                    }
                }
            }

            $this->execSql(
                'INSERT INTO task(agent, package, from_version, to_version, action, approved) VALUES '
                    . implode(',', array_fill(0, $rows, '(?,?,NULL,?,?,1)')),
                $insertParams
            );
        });

        return true;
    }
}
