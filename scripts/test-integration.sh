#!/bin/sh
set -eu

project=bp-companion-integration
export MYSQL_PORT=${TEST_MYSQL_PORT:-33306}
export MYSQL_ROOT_PASSWORD=integration-root-only
export MYSQL_DATABASE=bp_companion_integration
export MYSQL_USER=bp_companion
export MYSQL_PASSWORD=integration-local-only
export MYSQL_DSN="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(127.0.0.1:${MYSQL_PORT})/${MYSQL_DATABASE}?parseTime=true&charset=utf8mb4&loc=UTC"
export DATA_ENCRYPTION_KEY_B64="MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
export DATA_ENCRYPTION_KEY_VERSION="test-v1"

cleanup() {
  docker compose -p "$project" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose -p "$project" up -d mysql

attempt=0
until docker compose -p "$project" exec -T mysql mysqladmin ping \
  -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD" --silent >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "MySQL did not become ready"
    docker compose -p "$project" logs mysql
    exit 1
  fi
  sleep 1
done

go run ./server/cmd/migrate -direction up

table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'system_metadata'")
test "$table_count" = "1"

users_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'")
test "$users_table_count" = "1"
care_plans_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'care_plans'")
test "$care_plans_table_count" = "1"
task_instances_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'task_instances'")
test "$task_instances_table_count" = "1"
bp_records_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'bp_records'")
test "$bp_records_table_count" = "1"

go run ./server/cmd/migrate -direction down-one
bp_records_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'bp_records'")
test "$bp_records_table_count" = "0"
task_instances_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'task_instances'")
test "$task_instances_table_count" = "1"
care_plans_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'care_plans'")
test "$care_plans_table_count" = "1"
users_table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'")
test "$users_table_count" = "1"
table_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'system_metadata'")
test "$table_count" = "1"

go run ./server/cmd/migrate -direction up
go run ./server/cmd/migrate -direction up

applied_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM schema_migrations WHERE version = '000001_bootstrap_metadata'")
test "$applied_count" = "1"

users_migration_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM schema_migrations WHERE version = '000002_users'")
test "$users_migration_count" = "1"
care_plans_migration_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM schema_migrations WHERE version = '000003_care_plans'")
test "$care_plans_migration_count" = "1"
task_instances_migration_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM schema_migrations WHERE version = '000004_task_instances'")
test "$task_instances_migration_count" = "1"
bp_records_migration_count=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT COUNT(*) FROM schema_migrations WHERE version = '000005_bp_records'")
test "$bp_records_migration_count" = "1"

applied_state=$(docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse \
  "SELECT state FROM schema_migrations WHERE version = '000001_bootstrap_metadata'")
test "$applied_state" = "applied"

docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e \
  "INSERT INTO schema_migrations (version, checksum, state) VALUES ('999998_missing', REPEAT('0', 64), 'applied')" >/dev/null
if go run ./server/cmd/migrate -direction up >/dev/null 2>&1; then
  echo "migration runner accepted an applied migration missing from disk"
  exit 1
fi
docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e \
  "DELETE FROM schema_migrations WHERE version = '999998_missing'" >/dev/null

docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e \
  "INSERT INTO schema_migrations (version, checksum, state) VALUES ('999999_missing', REPEAT('0', 64), 'applying')" >/dev/null
if go run ./server/cmd/migrate -direction up >/dev/null 2>&1; then
  echo "migration runner accepted a dirty migration missing from disk"
  exit 1
fi
docker compose -p "$project" exec -T mysql mysql \
  -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e \
  "DELETE FROM schema_migrations WHERE version = '999999_missing'" >/dev/null

go test -tags=integration ./server/integration

echo "migration integration: reversible/idempotent runs and missing/dirty migration rejection passed"
