package db

var drivers []Driver

// Driver defines a structure for backend drivers to use when they registered
// themselves as a backend which implements the DB interface.
type Driver struct {
	// DbType is the identifier used to uniquely identify a specific
	// database driver.  There can be only one driver with the same name.
	DbType string

	// Create is the function that will be invoked with all user-specified
	// arguments to create the database.  This function must return
	// ErrDbExists if the database already exists.
	Create func(args ...interface{}) (DB, error)

	// Open is the function that will be invoked with all user-specified
	// arguments to open the database.  This function must return
	// ErrDbDoesNotExist if the database has not already been created.
	Open func(args ...interface{}) (DB, error)
}

func RegisterDriver(ins Driver) {
	for _, driver := range drivers {
		if driver.DbType == ins.DbType {
			return
		}
	}
	drivers = append(drivers, ins)
}

func RegisteredDbTypes() []string {
	var types []string
	for _, drv := range drivers {
		types = append(types, drv.DbType)
	}
	return types
}

func Create(dbType string, args ...interface{}) (DB, error) {
	for _, driver := range drivers {
		if driver.DbType == dbType {
			return driver.Create(args...)
		}
	}
	return nil, ErrDbUnknownType
}

func Open(dbType string, args ...interface{}) (DB, error) {
	for _, driver := range drivers {
		if driver.DbType == dbType {
			return driver.Open(args...)
		}
	}
	return nil, ErrDbUnknownType
}
