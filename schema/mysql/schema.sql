CREATE TABLE agent (
  id    BIGINT unsigned PRIMARY KEY AUTO_INCREMENT,
  name  VARCHAR(191)    NOT NULL UNIQUE,
  ctime BIGINT          NOT NULL,
  mtime BIGINT          NOT NULL
);

CREATE TABLE package (
  id    BIGINT unsigned PRIMARY KEY AUTO_INCREMENT,
  name  VARCHAR(191)    NOT NULL UNIQUE
);

CREATE TABLE task (
  agent         BIGINT unsigned REFERENCES agent(id),
  package       BIGINT unsigned REFERENCES package(id),
  from_version  VARCHAR(191),
  to_version    VARCHAR(191),
  action        ENUM('install', 'update', 'configure', 'remove', 'purge'),
  approved      TINYINT(1) unsigned NOT NULL,

  KEY (agent),
  KEY (package)
);
