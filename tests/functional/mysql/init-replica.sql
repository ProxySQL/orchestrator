-- Orchestrator user
CREATE USER IF NOT EXISTS 'orchestrator'@'%' IDENTIFIED BY 'orch_pass';
GRANT ALL PRIVILEGES ON *.* TO 'orchestrator'@'%' WITH GRANT OPTION;
-- Replication user (needed if this replica gets promoted)
CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY 'repl_pass';
GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%';
FLUSH PRIVILEGES;
-- NOTE: Replication is configured by setup-replication.sh after all containers are up
