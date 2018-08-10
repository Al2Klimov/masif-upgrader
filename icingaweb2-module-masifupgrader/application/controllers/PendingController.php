<?php

namespace Icinga\Module\Masifupgrader\Controllers;

use Icinga\Module\Masifupgrader\Forms\Pending\PackagesForm;
use Icinga\Module\Masifupgrader\Web\DbAwareControllerTrait;
use Icinga\Web\Controller;
use Icinga\Web\Url;
use Icinga\Web\Widget\Tabs;

class PendingController extends Controller
{
    use DbAwareControllerTrait;

    public function indexAction()
    {
        $this->view->form = $form = new PackagesForm();
        $form->setDb($this->db)
            ->handleRequest();

        $this->view->tabs = (new Tabs)->add('pending', [
            'label'     => $this->translate('Pending updates'),
            'title'     => $this->translate('Pending package updates'),
            'icon'      => 'reschedule',
            'url'       => Url::fromRequest(),
            'active'    => true
        ]);
    }
}
