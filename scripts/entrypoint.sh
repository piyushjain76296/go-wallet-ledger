#!/bin/sh
set -e

echo "Running Database Migrations..."
goose -dir ./db/migrations postgres "$DATABASE_URL" up

echo "Starting API Server..."
exec ./wallet-api
