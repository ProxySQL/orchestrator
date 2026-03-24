-- Orchestrator user with full privileges
CREATE USER IF NOT EXISTS 'orchestrator'@'%' IDENTIFIED BY 'orch_pass';
GRANT ALL PRIVILEGES ON *.* TO 'orchestrator'@'%' WITH GRANT OPTION;

-- Replication user
CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY 'repl_pass';
GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%';

FLUSH PRIVILEGES;
