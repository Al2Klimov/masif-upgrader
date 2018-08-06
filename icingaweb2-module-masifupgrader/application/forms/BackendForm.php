<?php

namespace Icinga\Module\Masifupgrader\Forms;

use Icinga\Data\ResourceFactory;
use Icinga\Forms\ConfigForm;

class BackendForm extends ConfigForm
{
    public function init()
    {
        $this->setName('form_config_backend');
        $this->setTitle($this->translate('Database backend'));
        $this->setSubmitLabel($this->translate('Save changes'));
    }

    public function createElements(array $formData)
    {
        $resources = [];
        foreach (ResourceFactory::getResourceConfigs('db') as $resource => $config) {
            if ($config->db === 'mysql') {
                $resources[$resource] = $resource;
            }
        }

        $resources[''] = '';
        ksort($resources);

        $this->addElement('select', 'backend_resource', [
            'label'         => $this->translate('Resource'),
            'description'   => $this->translate('Database resource'),
            'required'      => true,
            'multiOptions'  => $resources
        ]);
    }
}
