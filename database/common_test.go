package database_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sukhavati-Labs/go-miner/database"
	_ "github.com/Sukhavati-Labs/go-miner/database/ldb"
	_ "github.com/Sukhavati-Labs/go-miner/database/memdb"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var zeroHash = wire.Hash{}

// testDbRoot is the root directory used to create all test databases.
const testDbRoot = "testdbs"

// filesExists returns whether or not the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// openDB is used to open an existing database based on the database type and
// name.
func openDB(dbType, dbName string) (database.DB, error) {
	// Handle memdb specially since it has no files on disk.
	if dbType == "memdb" {
		db, err := database.Open(dbType)
		if err != nil {
			return nil, fmt.Errorf("error opening db: %v", err)
		}
		return db, nil
	}

	dbPath := filepath.Join(testDbRoot, dbName)
	db, err := database.Open(dbType, dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %v", err)
	}

	return db, nil
}

// createDB creates a new db instance and returns a teardown function the caller
// should invoke when done testing to clean up.  The close flag indicates
// whether or not the teardown function should sync and close the database
// during teardown.
func createDB(dbType, dbName string, close bool) (database.DB, func(), error) {
	// Handle memory database specially since it doesn't need the disk
	// specific handling.
	if dbType == "memdb" {
		db, err := database.Create(dbType)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating db: %v", err)
		}

		// Setup a teardown function for cleaning up.  This function is
		// returned to the caller to be invoked when it is done testing.
		teardown := func() {
			if close {
				db.Close()
			}
		}

		return db, teardown, nil
	}

	// Create the root directory for test databases.
	if !fileExists(testDbRoot) {
		if err := os.MkdirAll(testDbRoot, 0700); err != nil {
			err := fmt.Errorf("unable to create test db "+
				"root: %v", err)
			return nil, nil, err
		}
	}

	// Create a new database to store the accepted blocks into.
	dbPath := filepath.Join(testDbRoot, dbName)
	_ = os.RemoveAll(dbPath)
	db, err := database.Create(dbType, dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating db: %v", err)
	}

	// Setup a teardown function for cleaning up.  This function is
	// returned to the caller to be invoked when it is done testing.
	teardown := func() {
		dbVersionPath := filepath.Join(testDbRoot, dbName+".ver")
		if close {
			db.Sync()
			db.Close()
		}
		os.RemoveAll(dbPath)
		os.Remove(dbVersionPath)
		os.RemoveAll(testDbRoot)
	}

	return db, teardown, nil
}
