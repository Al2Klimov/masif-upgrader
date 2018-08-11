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
     * @param array $params
     *
     * @return array
     */
    protected function fetchAll($query, $params = [])
    {
        $stmt = $this->db->prepare($query);
        $stmt->execute($params);
        return $stmt->fetchAll(PDO::FETCH_NUM);
    }
}
