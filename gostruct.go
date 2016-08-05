package gostruct

import (
	"flag"
)

const DB_USERNAME = "devuser"
const DB_PASSWORD = "L!ght@m@tch"

func Generate() error {

	table := flag.String("table", "", "Table")
	database := flag.String("database", "", "Database")
	host := flag.String("host", "", "Server")
	flag.Parse()

	err := Run(*table, *database, *host)
	if err != nil {
		return err
	}

	return nil
}
