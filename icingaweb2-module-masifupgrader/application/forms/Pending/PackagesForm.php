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

    public function init()
    {
        $this->setName('form_pending_packages');
        $this->setSubmitLabel($this->translate('Approve selection'));
    }

    public function createElements(array $formData)
    {
        foreach ($this->getTasks() as $package => list($agents, $actions)) {
            foreach ($actions as $action => $toVersions) {
                foreach ($toVersions as $toVersion => $_) {
                    $this->addHeadlessElement(
                        'checkbox',
                        implode('_', [bin2hex($package), $action, bin2hex($toVersion)]),
                        []
                    );
                }
            }
        }
    }

    public function render(Zend_View_Interface $view = null)
    {
        if ($view === null) {
            $view = $this->getView();
        }

        $result = $this->renderHeadless($view)
            . "<table class='common-table'><thead><tr><th colspan='2'>{$view->escape($this->translate('Package, agents awaiting approval'))}</th>"
            . "<th colspan='2'>{$view->escape($this->translate('Action'))}</th>"
            . "<th colspan='2'>{$view->escape($this->translate('Target version'))}</th></tr></thead><tbody>";

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
                    if ($toVersion === null) {
                        $toVersion = $this->translate('N/A');
                    }

                    $approve = $this->getElement(implode('_', [bin2hex($package), $action, bin2hex($toVersion)]));

                    $rows[] = [
                        null,
                        null,
                        null,
                        "<td colspan='2'>{$approve->render($view)}<label for='{$view->escape($approve->getId())}'>&emsp;{$view->escape($toVersion)}</label></td>"
                    ];

                    ++$currentRow;
                }

                $actionRows = count($toVersions);
                $packageRows += $actionRows;
                $rows[$actionOnRow][2] = "<td rowspan='$actionRows' colspan='2'>{$view->escape($actionLabels[$action])}</td>";
            }

            $rows[$packageOnRow][0] = "<td rowspan='$packageRows'>{$view->escape($package)}</td>";
            $rows[$packageOnRow][1] = "<td rowspan='$packageRows'>$agents</td>";
        }

        foreach ($rows as $row) {
            $result .= '<tr>' . implode('', $row) . '</tr>';
        }

        return "$result<tr><td colspan='4'></td><td colspan='2'>{$this->getElement('btn_submit')->render($view)}</td></tr></tbody></table>";
    }

    public function onSuccess()
    {
        // TODO
        return true;
    }
}
