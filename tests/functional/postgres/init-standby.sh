#!/bin/bash
# Initialize a PostgreSQL standby via pg_basebackup from the primary.
# This script replaces the default entrypoint for standby containers.

set -e

PGDATA="/var/lib/postgresql/data"
PRIMARY_HOST="pgprimary"
PRIMARY_PORT=5432

echo "Waiting for primary to accept connections..."
until pg_isready -h "$PRIMARY_HOST" -p "$PRIMARY_PORT" -U postgres; do
    sleep 1
done

echo "Running pg_basebackup from $PRIMARY_HOST..."
rm -rf "$PGDATA"/*
pg_basebackup \
    -h "$PRIMARY_HOST" \
    -p "$PRIMARY_PORT" \
    -U repl \
    -D "$PGDATA" \
    -Fp -Xs -P -R

# pg_basebackup with -R creates standby.signal and sets primary_conninfo
# in postgresql.auto.conf. Override to ensure correct connection string.
cat > "$PGDATA/postgresql.auto.conf" <<EOF
primary_conninfo = 'host=$PRIMARY_HOST port=$PRIMARY_PORT user=repl password=repl_pass'
EOF

touch "$PGDATA/standby.signal"

# Fix permissions (pg_basebackup may set them correctly, but be safe)
chmod 0700 "$PGDATA"

echo "Starting PostgreSQL in standby mode..."
exec postgres -D "$PGDATA"
