#!/usr/bin/env bash
set -euo pipefail

sudo -u postgres psql -c "drop database if exists stonehenge_vendor;"
sudo -u postgres psql -c "drop user if exists stonehenge_vendor;"
sudo -u postgres psql -f scripts/create-local-db.sql
echo "stonehenge_vendor database recreated."
