# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table and all other tables related through all foreign key relationships. 

# flags 

table - This is the MySQL database table
    
database - This is the name of the MySQL database
    
host - This is the hostname or server of where the database is located

# usage

    package main

    import (
    	_ "github.com/go-sql-driver/mysql"
    	"github.com/jonathankentstevens/gostruct"
    	"log"
    )

    func main() {
    	err := gostruct.Generate()
    	if err != nil {
    	        panic(err)
    	}
    }
