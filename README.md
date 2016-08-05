# gostruct
This is a library to auto-generate Structs based on MySQL database tables and traversing down through the foreign key relationships.

USAGE

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
