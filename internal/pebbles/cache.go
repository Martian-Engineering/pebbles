package pebbles

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// EnsureCache rebuilds the cache if the events log is newer than the DB.
func EnsureCache(root string) error {
	needs, err := needsRebuild(EventsPath(root), DBPath(root))
	if err != nil {
		return err
	}
	if needs {
		return RebuildCache(root)
	}
	schemaNeeds, err := needsSchemaUpdate(DBPath(root))
	if err != nil {
		return err
	}
	if schemaNeeds {
		return RebuildCache(root)
	}
	return nil
}

// RebuildCache recreates the SQLite cache from the event log.
func RebuildCache(root string) error {
	events, err := LoadEvents(root)
	if err != nil {
		return err
	}
	// Normalize event order before replay.
	sortEvents(events)
	db, err := openDB(DBPath(root))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	// Recreate schema and replay the event log.
	if err := resetSchema(db); err != nil {
		return err
	}
	if err := ensureSchema(db); err != nil {
		return err
	}
	if err := applyEvents(db, events); err != nil {
		return err
	}
	return nil
}

// needsRebuild compares timestamps to decide if the cache is stale.
func needsRebuild(eventsPath, dbPath string) (bool, error) {
	eventsInfo, err := os.Stat(eventsPath)
	if err != nil {
		return false, fmt.Errorf("stat events log: %w", err)
	}
	dbInfo, err := os.Stat(dbPath)
	if err != nil {
		// Missing cache means a rebuild is required.
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat cache: %w", err)
	}
	return eventsInfo.ModTime().After(dbInfo.ModTime()), nil
}

// needsSchemaUpdate checks whether the cache schema is missing expected columns.
func needsSchemaUpdate(dbPath string) (bool, error) {
	// Open the cache database and inspect the deps table schema.
	db, err := openDB(dbPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()
	hasDepType, err := depsTableHasColumn(db, "dep_type")
	if err != nil {
		return false, err
	}
	// Trigger a rebuild if the new column is missing.
	return !hasDepType, nil
}

// depsTableHasColumn reports whether the deps table contains a column name.
func depsTableHasColumn(db *sql.DB, name string) (bool, error) {
	// PRAGMA table_info returns one row per column.
	rows, err := db.Query("PRAGMA table_info(deps)")
	if err != nil {
		return false, fmt.Errorf("deps schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	// Scan column metadata looking for the requested name.
	for rows.Next() {
		var cid int
		var colName string
		var colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan deps schema: %w", err)
		}
		if colName == name {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("deps schema rows: %w", err)
	}
	return false, nil
}

// openDB opens a SQLite database at the given path.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return db, nil
}

// sortEvents orders events by timestamp with a stable fallback.
func sortEvents(events []Event) {
	// Preserve original ordering by embedding an index.
	type indexed struct {
		Event
		Index int
	}
	indexedEvents := make([]indexed, 0, len(events))
	for i, event := range events {
		indexedEvents = append(indexedEvents, indexed{Event: event, Index: i})
	}
	// Sort by timestamp, then by original index.
	sort.SliceStable(indexedEvents, func(i, j int) bool {
		timeI, errI := time.Parse(time.RFC3339Nano, indexedEvents[i].Timestamp)
		timeJ, errJ := time.Parse(time.RFC3339Nano, indexedEvents[j].Timestamp)
		if errI == nil && errJ == nil && !timeI.Equal(timeJ) {
			return timeI.Before(timeJ)
		}
		return indexedEvents[i].Index < indexedEvents[j].Index
	})
	for i := range indexedEvents {
		events[i] = indexedEvents[i].Event
	}
}
