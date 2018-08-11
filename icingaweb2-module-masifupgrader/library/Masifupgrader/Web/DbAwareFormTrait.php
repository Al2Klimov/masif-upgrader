<?php

namespace Icinga\Module\Masifupgrader\Web;

use Exception;
use PDO;
use PDOException;

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

    /**
     * @param string $query
     * @param array $params
     *
     * @return int
     */
    protected function execSql($query, $params = [])
    {
        $stmt = $this->db->prepare($query);
        $stmt->execute($params);
        return $stmt->rowCount();
    }

    /**
     * @param callable $tx
     */
    protected function transaction($tx)
    {
        for (;;) {
            $this->db->beginTransaction();

            try {
                call_user_func($tx);
                $this->db->commit();
            } catch (Exception $e) {
                if ($e instanceof PDOException && (string)$e->getCode() === '40001') {
                    $this->db->rollBack();
                    continue;
                }

                $this->db->rollBack();
                throw $e;
            }

            break;
        }
    }
}
