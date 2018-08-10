<?php

namespace Icinga\Module\Masifupgrader\Web;

use Icinga\Application\Config;
use Icinga\Data\Db\DbConnection;
use Icinga\Data\ResourceFactory;
use Icinga\Exception\ConfigurationError;
use Icinga\Web\Controller;
use PDO;

trait DbAwareControllerTrait
{
    /**
     * @var PDO
     */
    protected $db;

    public function init()
    {
        /** @var Controller $that */
        $that = $this;
        $resource = Config::module('masifupgrader')->get('backend', 'resource');

        if ($resource === null) {
            if ($that->hasPermission('config/modules')) {
                $that->redirectNow('masifupgrader/config/backend');
            }

            throw new ConfigurationError('%s', $that->translate(
                'Database backend missing in Masif Upgrader configuration'
            ));
        }

        /** @var DbConnection $db */
        $db = ResourceFactory::create($resource);
        $this->db = $db->getDbAdapter()->getConnection();

        unset($db);

        $this->db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
        $this->db->prepare('SET SESSION TRANSACTION ISOLATION LEVEL SERIALIZABLE')->execute();
    }
}
