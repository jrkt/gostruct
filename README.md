# gostruct
This is a library to auto-generate Structs based on MySQL database tables and traversing down through the foreign key relationships.

USAGE

    package main
    
    import (
            "github.com/jonathankentstevens/gostruct"
    )
    
    func main() {
            gs := new(gostruct)
    }
