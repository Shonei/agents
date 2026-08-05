package storage

import (
	"fmt"

	"github.com/doug-martin/goqu/v9"
)

type migrations struct {
	name string
	run  func(db *goqu.Database) error
}

var registeredMigrations = []migrations{
	{
		name: "create_documents_table",
		run: func(db *goqu.Database) error {
			_, err := db.Exec(`CREATE TABLE IF NOT EXISTS documents (
				meta JSON,
				content TEXT,
				document_store TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				vec FLOAT[2048]
			);`)

			return err
		},
	},
	{
		name: "create_documents_index",
		run: func(db *goqu.Database) error {
			_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_documents_vec ON documents USING HNSW (vec) WITH (metric = 'cosine');`)

			return err
		},
	},
	{
		name: "create_documents_store_index",
		run: func(db *goqu.Database) error {
			_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_documents_store ON documents (document_store, created_at);`)

			return err
		},
	},
	{
		name: "create_audit_tables",
		run: func(db *goqu.Database) error {
			_, err := db.Exec(`CREATE TABLE IF NOT EXISTS audit_sessions (
				id TEXT PRIMARY KEY,
				session_hash TEXT,
				system_prompt TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);`)
			if err != nil {
				return err
			}

			_, err = db.Exec(`CREATE TABLE IF NOT EXISTS audit_events (
				id TEXT PRIMARY KEY,
				session_id TEXT,
				type TEXT,
				content TEXT,
				payload JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (session_id) REFERENCES audit_sessions(id)
			);`)

			return err
		},
	},
	{
		name: "add_parent_session_id_to_audit_sessions",
		run: func(db *goqu.Database) error {
			_, err := db.Exec(`ALTER TABLE audit_sessions ADD COLUMN parent_session_id TEXT;`)

			return err
		},
	},
	{
		name: "add_tools_to_audit_sessions",
		run: func(db *goqu.Database) error {
			if _, err := db.Exec(`ALTER TABLE audit_sessions ADD COLUMN IF NOT EXISTS tools TEXT;`); err != nil {
				return err
			}
			_, err := db.Exec(`ALTER TABLE audit_sessions ADD COLUMN IF NOT EXISTS server_tools TEXT;`)

			return err
		},
	},
}

func initDatabase(db *goqu.Database) error {
	// Load the VSS extension for vector similarity search.
	if _, err := db.Exec("INSTALL vss; LOAD vss;"); err != nil {
		return fmt.Errorf("failed to load vss extension: %w", err)
	}

	// Enable persisting the vector index on disk
	if _, err := db.Exec("SET hnsw_enable_experimental_persistence = True"); err != nil {
		return fmt.Errorf("failed to load vss extension: %w", err)
	}

	const createMigrationsTable = `CREATE TABLE IF NOT EXISTS migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createMigrationsTable); err != nil {
		return err
	}

	return nil
}

func runMigrations(db *goqu.Database) error {
	executed, err := getExecutedMigrations(db)
	if err != nil {
		return err
	}

	// validate all migration names are unique
	// mostly to stop myself from doing something bad
	seen := make(map[string]struct{}, len(registeredMigrations))
	for _, m := range registeredMigrations {
		_, ok := seen[m.name]
		if ok {
			return fmt.Errorf("duplicate migration name: %s", m.name)
		}

		seen[m.name] = struct{}{}
	}

	ranAny := false

	for _, migration := range registeredMigrations {
		if _, alreadyRan := executed[migration.name]; alreadyRan {
			continue
		}

		if err := migration.run(db); err != nil {
			return fmt.Errorf("failed to run migration %q: %w", migration.name, err)
		}

		if err := addMigration(db, migration.name); err != nil {
			return fmt.Errorf("failed to record migration %q: %w", migration.name, err)
		}

		ranAny = true
	}

	// Force a checkpoint so any DDL we just applied is materialized into the
	// main DB file rather than left in the WAL. This avoids an unreplayable
	// WAL on the next startup if the process is killed before a clean shutdown
	// (see DuckDB issues #19712 / #22124).
	if ranAny {
		if _, err := db.Exec("CHECKPOINT;"); err != nil {
			return fmt.Errorf("failed to checkpoint after migrations: %w", err)
		}
	}

	return nil
}

type migrationRow struct {
	Name string `db:"name"`
}

func getExecutedMigrations(goquDB *goqu.Database) (map[string]struct{}, error) {
	ds := goquDB.From("migrations").Select("name")

	var rows []migrationRow
	if err := ds.ScanStructs(&rows); err != nil {
		return nil, fmt.Errorf("failed to scan executed migrations: %w", err)
	}

	ranMigrations := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		ranMigrations[r.Name] = struct{}{}
	}

	return ranMigrations, nil
}

func addMigration(goquDB *goqu.Database, name string) error {
	ds := goquDB.Insert("migrations").Cols("name").Vals(goqu.Vals{name})

	query, args, err := ds.ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build insert migration query: %w", err)
	}

	if _, err := goquDB.Exec(query, args...); err != nil {
		return fmt.Errorf("failed to insert migration record: %w", err)
	}

	return nil
}
