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
	if !needs {
		return nil
	}
	return RebuildCache(root)
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
