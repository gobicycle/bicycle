#!/bin/bash

if [ -z "$POSTGRES_READONLY_PASSWORD" ]; then
  echo "Environment variable POSTGRES_READONLY_PASSWORD is not set. Exiting."
  exit 1
fi

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  CREATE USER pp_readonly WITH PASSWORD '$POSTGRES_READONLY_PASSWORD';
  GRANT CONNECT ON DATABASE $POSTGRES_DB TO pp_readonly;
  GRANT USAGE ON SCHEMA payments TO pp_readonly;
  GRANT SELECT ON ALL TABLES IN SCHEMA payments TO pp_readonly;
  ALTER DEFAULT PRIVILEGES IN SCHEMA payments GRANT SELECT ON TABLES TO pp_readonly;
EOSQL
