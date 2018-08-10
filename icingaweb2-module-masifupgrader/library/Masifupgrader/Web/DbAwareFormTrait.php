<?php

namespace Icinga\Module\Masifupgrader\Web;

use PDO;

trait DbAwareFormTrait
{
    /**
     * @var PDO
     */
    protected $db;

    /**
     * @param PDO $db
     *
     * @return $this
     */
    public function setDb($db)
    {
        $this->db = $db;
        return $this;
    }

    /**
     * @param string $query
     *
     * @return array
     */
    protected function fetchAll($query)
    {
        $stmt = $this->db->prepare($query);
        $stmt->execute();
        return $stmt->fetchAll(PDO::FETCH_NUM);
    }
}
