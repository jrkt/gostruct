# gostruct
This is a library to auto-generate Structs based on MySQL database tables and traversing down through the foreign key relationships.

USAGE

FLAGS 

table 
    This is the MySQL database table
    
database
    This is the name of the MySQL database
    
host
    This is the hostname or server of where the database is located

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
