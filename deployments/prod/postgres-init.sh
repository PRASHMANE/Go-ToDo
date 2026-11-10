#!/bin/sh
# Runs once, when the Postgres volume is empty. Creates one database + one
# user per service, with passwords from /etc/go-todo/infra.env.
# Use hex passwords (openssl rand -hex 24) so no quoting problems can happen.
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<EOSQL
CREATE USER auth_user WITH PASSWORD '${AUTH_DB_PASSWORD}';
CREATE DATABASE auth_db OWNER auth_user;

CREATE USER todo_user WITH PASSWORD '${TODO_DB_PASSWORD}';
CREATE DATABASE todo_db OWNER todo_user;

CREATE USER activity_user WITH PASSWORD '${ACTIVITY_DB_PASSWORD}';
CREATE DATABASE activity_db OWNER activity_user;
EOSQL
