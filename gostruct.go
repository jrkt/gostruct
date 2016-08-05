package gostruct

import (
	"flag"
)

var table string
var database string
var ip string

const DB_USERNAME = "devuser"
const DB_PASSWORD = "L!ght@m@tch"

func Generate() error {

	t := flag.String("table", "", "Table")
	d := flag.String("database", "", "Database")
	i := flag.String("ip", "", "Server")
	flag.Parse()

	table = *t
	database = *d
	ip = *i

	err := Run(table, database, ip)
	if err != nil {
		return err
	}

	return nil
}
