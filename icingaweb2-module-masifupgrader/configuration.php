<?php

/** @var \Icinga\Application\Modules\Module $this */

$this->provideConfigTab('backend', [
    'url'   => 'config/backend',
    'label' => $this->translate('Backend'),
    'title' => $this->translate('Database backend')
]);
