# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table and all other tables related through all foreign key relationships. 

# flags 

table
    
    MySQL database table
    
database
    
    Name of the MySQL database
    
host
    
    Hostname or server of where the database is located

# usage - test.go

Replace the {username} and {password} constants in gostruct.go to the credentials of your database.

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

# run it

    go run test.go -table user -database main -host localhost
    
A package with a struct and a method to read by the primary key will be created in the $GOPATH/src/models/ directory. It will also build packages for any other tables that have foreign keys of the table give.
