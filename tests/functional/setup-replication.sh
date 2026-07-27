#!/bin/bash
# Set up replication after all MySQL containers are running
set -euo pipefail

COMPOSE="${COMPOSE:-docker compose -f tests/functional/docker-compose.yml}"

echo "Setting up replication..."

# Detect MySQL / MariaDB version
MYSQL_FULL_VERSION=$($COMPOSE exec -T mysql1 mysql -uroot -ptestpass -Nse "SELECT VERSION()" 2>/dev/null | tr -d '[:space:]')
MYSQL_MAJOR=$(echo "$MYSQL_FULL_VERSION" | sed -E 's/^([0-9]+\.[0-9]+).*/\1/')
echo "Detected MySQL version: $MYSQL_FULL_VERSION (major: $MYSQL_MAJOR)"

if echo "$MYSQL_FULL_VERSION" | grep -qi mariadb; then
    CHANGE_SOURCE_CMD="CHANGE MASTER TO MASTER_HOST='mysql1', MASTER_PORT=3306, MASTER_USER='repl', MASTER_PASSWORD='repl_pass', MASTER_USE_GTID=slave_pos"
    START_REPLICA_CMD="START SLAVE"
    SHOW_REPLICA_CMD="SHOW SLAVE STATUS\G"
elif [ "$MYSQL_MAJOR" = "5.7" ]; then
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
            SET GLOBAL super_read_only=0;
            SET GLOBAL read_only=0;
            STOP REPLICA; STOP SLAVE;
        " 2>/dev/null || true
        if $COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e "
            $CHANGE_SOURCE_CMD;
            SET GLOBAL read_only=1;
            $START_REPLICA_CMD;
        " 2>/dev/null; then
            break
        fi
        sleep 1
    done
done

echo "Verifying replication..."
for REPLICA in mysql2 mysql3; do
    for i in $(seq 1 60); do
        if echo "$MYSQL_FULL_VERSION" | grep -qi mariadb; then
            IO=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e \
                "SHOW SLAVE STATUS\G" 2>/dev/null | awk -F': *' '/Slave_IO_Running:/{print $2; exit}' | tr -d '[:space:]')
            SQL=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -e \
                "SHOW SLAVE STATUS\G" 2>/dev/null | awk -F': *' '/Slave_SQL_Running:/{print $2; exit}' | tr -d '[:space:]')
            if [ "$IO" = "Yes" ] && [ "$SQL" = "Yes" ]; then
                echo "$REPLICA: replication OK (IO+SQL Yes)"
                break
            fi
        else
            IO=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
                "SELECT SERVICE_STATE FROM performance_schema.replication_connection_status WHERE CHANNEL_NAME='' LIMIT 1" 2>/dev/null | tr -d '[:space:]')
            SQL=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
                "SELECT SERVICE_STATE FROM performance_schema.replication_applier_status WHERE CHANNEL_NAME='' LIMIT 1" 2>/dev/null | tr -d '[:space:]')
            if [ -z "$IO" ]; then
                IO=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
                    "SELECT SERVICE_STATE FROM performance_schema.replication_connection_status LIMIT 1" 2>/dev/null | tr -d '[:space:]')
            fi
            if [ -z "$SQL" ]; then
                SQL=$($COMPOSE exec -T "$REPLICA" mysql -uroot -ptestpass -Nse \
                    "SELECT SERVICE_STATE FROM performance_schema.replication_applier_status LIMIT 1" 2>/dev/null | tr -d '[:space:]')
            fi
            if [ "$IO" = "ON" ] && [ "$SQL" = "ON" ]; then
                echo "$REPLICA: replication OK (IO+SQL ON)"
                break
            fi
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
