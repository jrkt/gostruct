package gostruct

import (
	"flag"
	"errors"
)

type Gostruct struct {
	Table       string
	Database    string
	Host        string
	Port        string
	Username string
	Password string
}

//Generate table model for mysql
func (gs *Gostruct) Generate() error {
	var err error

	table := flag.String("table", "", "Table")
	db := flag.String("db", "", "Database")
	host := flag.String("host", "", "DB Host")
	port := flag.String("port", "", "DB Port (MySQL 3306 is default)")
	all := flag.String("all", "", "Run for All Tables")
	flag.Parse()

	gs.Table = *table
	gs.Database = *db
	gs.Host = *host
	gs.SetPort(*port)

	if *all == "true" {
		err = gs.RunAll()
		if err != nil {
			return err
		}
	} else {
		if (*table == "" && *all != "true") || *db == "" || *host == "" {
			return errors.New("You must include the 'table', 'database', and 'host' flags")
		} else {
			err = gs.Run(*table)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (gs *Gostruct) SetPort(port string) {
	if port == "" {
		port = "3306"
	}
	gs.Port = port
}

