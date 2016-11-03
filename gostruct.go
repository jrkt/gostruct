package gostruct

import (
	"flag"
	"errors"
)

type Gostruct struct {
	DB_Username string
	DB_Password string
}

var DB_USERNAME string
var DB_PASSWORD string

//Generate table model for mysql
func (gs *Gostruct) Generate() error {
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

func (gs *Gostruct) SetUsername(username string) {
	gs.DB_Username = username
}

func (gs *Gostruct) Username() string {
	return gs.DB_Username
}

func (gs *Gostruct) SetPassword(password string) {
	gs.DB_Password = password
}

func (gs *Gostruct) Password() string {
	return gs.DB_Password
}