#!/bin/bash
# Set up replication after all MySQL containers are running
set -euo pipefail

COMPOSE="docker compose -f tests/functional/docker-compose.yml"

echo "Setting up replication..."

# Detect MySQL version
MYSQL_FULL_VERSION=$($COMPOSE exec -T mysql1 mysql -uroot -ptestpass -Nse "SELECT VERSION()" 2>/dev/null | tr -d '[:space:]')
MYSQL_MAJOR=$(echo "$MYSQL_FULL_VERSION" | sed -E 's/^([0-9]+\.[0-9]+).*/\1/')
echo "Detected MySQL version: $MYSQL_FULL_VERSION (major: $MYSQL_MAJOR)"

if [ "$MYSQL_MAJOR" = "5.7" ]; then
    CHANGE_SOURCE_CMD="CHANGE MASTER TO MASTER_HOST='mysql1', MASTER_PORT=3306, MASTER_USER='repl', MASTER_PASSWORD='repl_pass', MASTER_AUTO_POSITION=1"
    START_REPLICA_CMD="START SLAVE"
    SHOW_REPLICA_CMD="SHOW SLAVE STATUS\G"
else
    CHANGE_SOURCE_CMD="CHANGE REPLICATION SOURCE TO SOURCE_HOST='mysql1', SOURCE_PORT=3306, SOURCE_USER='repl', SOURCE_PASSWORD='repl_pass', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1"
    START_REPLICA_CMD="START REPLICA"
    SHOW_REPLICA_CMD="SHOW REPLICA STATUS\G"
fi

for REPLICA in mysql2 mysql3; do
    echo "Configuring $REPLICA to replicate from mysql1..."
    for i in $(seq 1 30); do
        $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "
            $CHANGE_SOURCE_CMD;
            $START_REPLICA_CMD;
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
            $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "$SHOW_REPLICA_CMD" 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done
done

echo "Replication setup complete"
