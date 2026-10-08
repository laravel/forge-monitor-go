package store

// schemaStatements is the exact on-disk schema produced by the Laravel
// migrations, captured from a live database/database.sqlite via `.schema`.
// Every statement uses IF NOT EXISTS so it is a no-op against an existing
// Forge database and produces an identical schema on a fresh one.
//
// Note the differences from the migration source: monitor_state materializes
// as varchar (not char(7)), the unsignedInteger columns are plain integer, and
// created_at/updated_at are nullable datetime.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS "alerts" (` +
		`"id" integer primary key autoincrement not null, ` +
		`"monitor_id" varchar not null, ` +
		`"monitor_type" varchar not null, ` +
		`"monitor_state" varchar not null default 'UNKNOWN', ` +
		`"created_at" datetime, "updated_at" datetime)`,
	`CREATE INDEX IF NOT EXISTS "alerts_created_at_index" on "alerts" ("created_at")`,

	`CREATE TABLE IF NOT EXISTS "disk_usages" (` +
		`"id" integer primary key autoincrement not null, ` +
		`"total" integer not null, "free" integer not null, "used" integer not null, ` +
		`"created_at" datetime, "updated_at" datetime)`,
	`CREATE INDEX IF NOT EXISTS "disk_usages_created_at_index" on "disk_usages" ("created_at")`,

	`CREATE TABLE IF NOT EXISTS "memory_usages" (` +
		`"id" integer primary key autoincrement not null, ` +
		`"total" integer not null, "available" integer not null, ` +
		`"used" integer not null, "free" integer not null, ` +
		`"created_at" datetime, "updated_at" datetime)`,
	`CREATE INDEX IF NOT EXISTS "memory_usages_created_at_index" on "memory_usages" ("created_at")`,

	`CREATE TABLE IF NOT EXISTS "load_avgs" (` +
		`"id" integer primary key autoincrement not null, ` +
		`"load_avg" integer not null, "load_avg_percent" integer, "cpus" integer not null, ` +
		`"created_at" datetime, "updated_at" datetime)`,
	`CREATE INDEX IF NOT EXISTS "load_avgs_created_at_index" on "load_avgs" ("created_at")`,
}
