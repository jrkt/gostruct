package gostruct

import (
	"flag"
)

func Generate(username string, password string) error {

	table := flag.String("table", "", "Table")
	database := flag.String("database", "", "Database")
	host := flag.String("host", "", "Server")
	flag.Parse()

	err := Run(*table, *database, *host, username, password)
	if err != nil {
		return err
	}

	return nil
}
