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
                SOURCE_AUTO_POSITION=1,
                GET_SOURCE_PUBLIC_KEY=1;
            START REPLICA;
        " 2>/dev/null && break
        sleep 1
    done
done

echo "Verifying replication..."
for REPLICA in mysql2 mysql3; do
    for i in $(seq 1 60); do
        STATUS=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
            "SELECT SERVICE_STATE FROM performance_schema.replication_connection_status" 2>/dev/null | tr -d '[:space:]')
        if [ "$STATUS" = "ON" ]; then
            echo "$REPLICA: replication OK (IO thread ON)"
            break
        fi
        if [ "$i" -eq 60 ]; then
            echo "$REPLICA: replication FAILED after 60s"
            $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "SHOW REPLICA STATUS\G" 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done
done

echo "Replication setup complete"
