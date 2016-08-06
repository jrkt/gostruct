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
