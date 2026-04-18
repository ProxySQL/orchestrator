#!/bin/bash
# Initialize PostgreSQL primary for functional tests.
# This script runs inside the postgres Docker entrypoint (initdb phase).

set -e

# ---- WAL / replication settings ----
cat >> "$PGDATA/postgresql.conf" <<EOF
wal_level = replica
max_wal_senders = 10
wal_keep_size = 256MB
hot_standby = on
listen_addresses = '*'
EOF

# ---- pg_hba: allow replication and orchestrator connections ----
cat >> "$PGDATA/pg_hba.conf" <<EOF
# Allow replication connections from any host in the Docker network
host    replication     repl        all     md5
# Allow orchestrator monitoring user
host    all             orchestrator all     md5
# Allow orchestrator to act as a replication client when graceful-switchover
# sets primary_conninfo on a demoted primary using orchestrator's credentials
host    replication     orchestrator all     md5
EOF

# ---- Create users ----
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    -- Replication user
    CREATE ROLE repl WITH REPLICATION LOGIN PASSWORD 'repl_pass';

    -- Orchestrator monitoring user
    CREATE ROLE orchestrator WITH LOGIN PASSWORD 'orch_pass';
    GRANT pg_monitor TO orchestrator;
    -- Allow orchestrator to promote standbys and reload config
    ALTER ROLE orchestrator SUPERUSER;
EOSQL

echo "Primary initialization complete."
