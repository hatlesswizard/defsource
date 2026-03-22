//go:build sqlite_fts5 || fts5

package sqlite

const migrationV1 = `
CREATE TABLE IF NOT EXISTS schema_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '1');

CREATE TABLE IF NOT EXISTS libraries (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    source_url    TEXT NOT NULL,
    version       TEXT NOT NULL DEFAULT '',
    trust_score   REAL NOT NULL DEFAULT 0.8,
    snippet_count INTEGER NOT NULL DEFAULT 0,
    crawled_at    DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entities (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'class',
    description TEXT NOT NULL DEFAULT '',
    source_file TEXT NOT NULL DEFAULT '',
    source_code TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(library_id, slug)
);

CREATE TABLE IF NOT EXISTS properties (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id   INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    visibility  TEXT NOT NULL DEFAULT 'public',
    since       TEXT NOT NULL DEFAULT '',
    UNIQUE(entity_id, name)
);

CREATE TABLE IF NOT EXISTS methods (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id       INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    slug            TEXT NOT NULL,
    name            TEXT NOT NULL,
    signature       TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    parameters_json TEXT NOT NULL DEFAULT '[]',
    return_type     TEXT NOT NULL DEFAULT '',
    return_desc     TEXT NOT NULL DEFAULT '',
    source_code     TEXT NOT NULL DEFAULT '',
    wrapped_source  TEXT NOT NULL DEFAULT '',
    wrapped_method  TEXT NOT NULL DEFAULT '',
    url             TEXT NOT NULL,
    since           TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_id, slug)
);

CREATE TABLE IF NOT EXISTS relations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    method_id   INTEGER NOT NULL REFERENCES methods(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    target_name TEXT NOT NULL,
    target_url  TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_relations_method ON relations(method_id);

CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
    library_id,
    entity_name,
    method_name,
    content,
    tokenize='porter unicode61'
);

CREATE TABLE IF NOT EXISTS search_index_map (
    rowid        INTEGER PRIMARY KEY,
    entity_id    INTEGER NOT NULL,
    method_id    INTEGER,
    snippet_type TEXT NOT NULL
);
`

const migrationV2 = `
CREATE TABLE IF NOT EXISTS crawl_sessions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'in_progress',
    total_urls      INTEGER DEFAULT 0,
    success_count   INTEGER DEFAULT 0,
    fail_count      INTEGER DEFAULT 0,
    skip_count      INTEGER DEFAULT 0,
    started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE TABLE IF NOT EXISTS crawl_progress (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      INTEGER NOT NULL REFERENCES crawl_sessions(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    item_type       TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    error_type      TEXT,
    error_message   TEXT,
    parent_entity   TEXT,
    processed_at    DATETIME,
    UNIQUE(session_id, url)
);
CREATE INDEX IF NOT EXISTS idx_cp_session_status ON crawl_progress(session_id, status);

UPDATE schema_meta SET value = '2' WHERE key = 'version';
`
