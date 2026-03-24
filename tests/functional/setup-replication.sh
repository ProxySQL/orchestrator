#!/bin/bash
# Set up replication after all MySQL containers are running
set -euo pipefail

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

echo "Setting up replication..."

for REPLICA in mysql2 mysql3; do
    echo "Configuring $REPLICA to replicate from mysql1..."
    for i in $(seq 1 30); do
        $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "
            CHANGE REPLICATION SOURCE TO
                SOURCE_HOST='mysql1',
                SOURCE_PORT=3306,
                SOURCE_USER='repl',
                SOURCE_PASSWORD='repl_pass',
                SOURCE_AUTO_POSITION=1;
            START REPLICA;
        " 2>/dev/null && break
        sleep 1
    done
done

echo "Verifying replication..."
for REPLICA in mysql2 mysql3; do
    for i in $(seq 1 30); do
        IO_RUNNING=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
            "SHOW REPLICA STATUS\G" 2>/dev/null | grep "Replica_IO_Running: Yes" || true)
        SQL_RUNNING=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
            "SHOW REPLICA STATUS\G" 2>/dev/null | grep "Replica_SQL_Running: Yes" || true)
        if [ -n "$IO_RUNNING" ] && [ -n "$SQL_RUNNING" ]; then
            echo "$REPLICA: replication OK"
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "$REPLICA: replication FAILED"
            $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null
            exit 1
        fi
        sleep 1
    done
done

echo "Replication setup complete"
