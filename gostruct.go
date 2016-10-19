package gostruct

import (
	"flag"
	"errors"
)

const DB_USERNAME = "root";
const DB_PASSWORD = "Jstevens120)";

type Table struct {
	Name string
}

//Generate table model for mysql
func Generate() error {
	var err error

	table := flag.String("table", "", "Table")
	database := flag.String("database", "", "Database")
	host := flag.String("host", "", "Server")
	port := flag.String("port", "", "Port")
	all := flag.String("all", "", "Run for All Tables")
	flag.Parse()

	if *all == "true" {
		err = RunAll(*database, *host, *port)
		if err != nil {
			return err
		}
	} else {
		if (*table == "" && *all != "true") || *database == "" || *host == "" {
			return errors.New("You must include the 'table', 'database', and 'host' flags")
		} else {
			err = Run(*table, *database, *host, *port)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}
