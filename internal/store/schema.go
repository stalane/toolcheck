package store

const SchemaVersion = 1

var migrations = map[int]string{
	1: `
CREATE TABLE tools (
	id                  INTEGER PRIMARY KEY,
	name                TEXT NOT NULL,
	path                TEXT NOT NULL,
	real_path           TEXT NOT NULL,
	active              INTEGER NOT NULL DEFAULT 1,
	family              TEXT NOT NULL DEFAULT '',
	version             TEXT NOT NULL DEFAULT '',
	origin              TEXT NOT NULL DEFAULT '',
	managed_by          TEXT NOT NULL DEFAULT '',
	writable            INTEGER NOT NULL DEFAULT 0,
	updateable          INTEGER NOT NULL DEFAULT 0,
	risk                TEXT NOT NULL DEFAULT 'unknown',
	source              TEXT NOT NULL DEFAULT '',
	latest_version      TEXT NOT NULL DEFAULT '',
	latest_url          TEXT NOT NULL DEFAULT '',
	latest_published_at TEXT,
	latest_security     INTEGER NOT NULL DEFAULT 0,
	checked_at          TEXT,
	UNIQUE(name, real_path)
);

CREATE TABLE checks (
	id          INTEGER PRIMARY KEY,
	started_at  TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	status      TEXT NOT NULL,
	tools       INTEGER NOT NULL DEFAULT 0,
	updates     INTEGER NOT NULL DEFAULT 0,
	error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE tool_updates (
	id           INTEGER PRIMARY KEY,
	tool_id      INTEGER NOT NULL DEFAULT 0,
	tool_name    TEXT NOT NULL,
	from_version TEXT NOT NULL,
	to_version   TEXT NOT NULL,
	command      TEXT NOT NULL,
	status       TEXT NOT NULL,
	output       TEXT NOT NULL DEFAULT '',
	started_at   TEXT NOT NULL,
	finished_at  TEXT NOT NULL
);

CREATE TABLE tool_notifications (
	id             INTEGER PRIMARY KEY,
	tool_id        INTEGER NOT NULL,
	latest_version TEXT NOT NULL,
	sent_at        TEXT NOT NULL,
	UNIQUE(tool_id, latest_version)
);

CREATE INDEX tools_name ON tools(name);
CREATE INDEX tool_updates_tool ON tool_updates(tool_id, started_at);
`,
}
