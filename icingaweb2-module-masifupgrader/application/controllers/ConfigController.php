<?php

namespace Icinga\Module\Masifupgrader\Controllers;

use Icinga\Application\Config;
use Icinga\Module\Masifupgrader\Forms\BackendForm;
use Icinga\Web\Controller;

class ConfigController extends Controller
{
    public function backendAction()
    {
        $this->assertPermission('config/modules');

        $this->view->form = $form = new BackendForm();
        $form->setIniConfig(Config::module('masifupgrader'))
            ->handleRequest();

        $this->view->tabs = $this->Module()->getConfigTabs()->activate('backend');
    }
}
