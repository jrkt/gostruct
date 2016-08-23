package gostruct

import (
	"flag"
	"db/circlepix"
)

const DB_USERNAME = "{username}";
const DB_PASSWORD = "{password}";

type Table struct {
	Name string
}

func Generate() error {
	var err error

	table := flag.String("table", "", "Table")
	database := flag.String("database", "", "Database")
	host := flag.String("host", "", "Server")
	port := flag.String("port", "", "Port")
	all := flag.String("all", "", "Run for All Tables")
	flag.Parse()

	if *all == "true" {
		connection := db.GetConnection()
		rows, err := connection.Query("SELECT DISTINCT(TABLE_NAME) FROM `information_schema`.`COLUMNS` WHERE `TABLE_SCHEMA` LIKE ?", database)
		if err != nil {
			panic(err)
		} else {
			for rows.Next() {
				var table Table
				rows.Scan(&table.Name)

				err = Run(table.Name, *database, *host, *port)
				if err != nil {
					return err
				}
			}
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
