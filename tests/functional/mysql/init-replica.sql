-- Orchestrator user (replicated from master, but define here for safety)
CREATE USER IF NOT EXISTS 'orchestrator'@'%' IDENTIFIED BY 'orch_pass';
GRANT ALL PRIVILEGES ON *.* TO 'orchestrator'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;

-- Configure replication to master
CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='mysql1',
  SOURCE_PORT=3306,
  SOURCE_USER='repl',
  SOURCE_PASSWORD='repl_pass',
  SOURCE_AUTO_POSITION=1;

START REPLICA;
