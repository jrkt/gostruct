[![License](http://img.shields.io/:license-gpl3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0.html)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonathankentstevens/gostruct)](https://goreportcard.com/report/github.com/jonathankentstevens/gostruct)
[![GoDoc](https://godoc.org/github.com/jonathankentstevens/gostruct?status.svg)](https://godoc.org/github.com/jonathankentstevens/gostruct)
[![Build Status](https://travis-ci.org/jonathankentstevens/gostruct.svg?branch=master)](https://travis-ci.org/jonathankentstevens/gostruct)

# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table.

# implementation

    go get github.com/go-sql-driver/mysql
    go get github.com/jonathankentstevens/gostruct

Create a generate.go file with the following contents (including your db username/password):

```go
package main

import (
    _ "github.com/go-sql-driver/mysql"
    "github.com/jonathankentstevens/gostruct"
    "log"
)

func main() {
    gs := new(gostruct.Gostruct)
    gs.Username = "<db_user>"
    gs.Password = "<db_pass>"
    err := gs.Generate()
    if err != nil {
        log.Fatalln(err)
    }
}
```
    
Then, run:

    go run generate.go -tables User -db main -host localhost
    
A package with a struct of the table and several methods to handle common requests will be created in the $GOPATH/src/models/{table} directory. The files that are created, for a 'User' model (for example) would be: CRUX_User.go (containing the main CRUX methods and common methods such as ReadById, ReadAll, ReadOneByQuery, ReadByQuery, and Exec), DAO_User.go (this will hold any custom methods used to return User object(s)), BO_User.go (this contains methods to be called on the User object itself), a User_test.go to serve as a base for your unit testing and an examples_test.go with auto-generated example methods for godoc readability. In addition, it will generate a connection package to share a connection between all your models to prevent multiple open database connections and a date package to implement a "sql.NullTime"-like struct type for null date values in any MySQL result set.

# flags 

tables
    
    comma-separated list of MySQL database tables
    
db
    
    Name of the MySQL database
    
host
    
    Hostname or server of where the database is located
    
port

    Defaults to 3306 if not provided
    
all

    If this option is passed in as "true", it will run for all tables based on the database

# usage
```go
package main

import (
    "models/User"
)

func main() {
    //retrieve existing user by id
    user, err := User.ReadById(12345)
    if err != nil {
        //handle error
    } else {
	    user.Email = "test@email.com"
	    user.IsActive = false
	    user.Save()
    }
    
    //create new user
    user := new(User.UserObj)
    user.Email = "test@email.com"
    res, err := user.Save()
    if err != nil {
    	//Save failed
    }

    //delete user
    _, err := user.Delete()
    if err != nil {
    	//Delete failed
    }
}
```
# DAO_User.go - sample method to include
```go
func ReadAllActive(order string) ([]*UserObj, error) {
	orderStr := ""
	if order != "" {
		orderStr = " ORDER BY " + order
	}
	return ReadByQuery("SELECT * FROM User WHERE IsActive = '1'" + orderStr)
}
```
Usage:
```go
func main() {
	users, err := User.ReadAllActive("Name ASC")
	if err != nil {
		//handle error
	}
	
	//handle users
	for _, user := range users {
	    
	}
}
```
# BO_User.go - sample method to include
```go
func (user *UserObj) Terminate() error {
	user.IsActive = false
	user.TerminationDate.Time = time.Now().Local()
	_, err := user.Save()
	if err != nil {
		//Save failed
		return err
	}
	return nil
}
```
Usage:
```go
func main() {
	users, err := User.ReadAllActive("Name ASC")
	if err != nil {
	    //read failed or no results found
	} else {
	    //handle users
		for _, user := range users {
			user.Terminate()
		}
	}
}
```
# CRUX_User.go - sample file
```go
//Package User serves as the base structure for the User table
//and contains base methods and CRUD functionality to
//interact with the User table in the main database
package User

import (
	"connection"
	"database/sql"
	"date"
	"reflect"
	"strings"
	"utils"
)

//UserObj is the structure of the User table
type UserObj struct {
	Id              int           `column:"id" default:"" type:"int(10) unsigned" key:"PRI" extra:"auto_increment"`
	Name            string        `column:"name" default:"" type:"varchar(150)" key:"" extra:""`
	Email           string        `column:"email" default:"" type:"varchar(250)" key:"" extra:""`
	Income          float64       `column:"income" default:"0" type:"decimal(10,0)" key:"" extra:""`
	IsActive        bool          `column:"isActive" default:"1" type:"tinyint(1)" key:"" extra:""`
	SignupDate      date.NullTime `column:"signupDate" default:"" type:"datetime" key:"" extra:""`
	TerminationDate date.NullTime `column:"terminationDate" default:"" type:"datetime" key:"" extra:""`
}

//Save does just that. It will save if the object key exists, otherwise it will add the record
//by running INSERT ON DUPLICATE KEY UPDATE
func (user *UserObj) Save() (sql.Result, error) {
	v := reflect.ValueOf(user).Elem()
	objType := v.Type()

	var columnArr []string
	var args []interface{}
	var q []string

	updateStr := ""
	query := "INSERT INTO User"
	for i := 0; i < v.NumField(); i++ {
		val, err := utils.ValidateField(v.Field(i), objType.Field(i))
		if err != nil {
			return nil, err
		}
		args = append(args, val)
		column := string(objType.Field(i).Tag.Get("column"))
		columnArr = append(columnArr, "`"+column+"`")
		q = append(q, "?")
		if i > 0 && updateStr != "" {
			updateStr += ", "
		}
		updateStr += "`" + column + "` = ?"
	}

	query += " (" + strings.Join(columnArr, ", ") + ") VALUES (" + strings.Join(q, ", ") + ") ON DUPLICATE KEY UPDATE " + updateStr
	newArgs := append(args, args...)
	newRecord := false
	if utils.Empty(user.Id) {
		newRecord = true
	}

	res, err := Exec(query, newArgs...)
	if err == nil && newRecord {
		id, _ := res.LastInsertId()
		user.Id = int(id)
	}
	return res, err
}

//Delete does just that. It removes that record from the database based on the primary key.
func (user *UserObj) Delete() (sql.Result, error) {
	return Exec("DELETE FROM User WHERE id = ?", user.Id)
}

//ReadById returns a pointer to a(n) UserObj
func ReadById(id int) (*UserObj, error) {
	return ReadOneByQuery("SELECT * FROM User WHERE id = ?", id)
}

//ReadAll returns all records in the User table
func ReadAll(order string) ([]*UserObj, error) {
	query := "SELECT * FROM User"
	if order != "" {
		query += " ORDER BY " + order
	}
	return ReadByQuery(query)
}

//ReadByQuery returns an array of UserObj pointers
func ReadByQuery(query string, args ...interface{}) ([]*UserObj, error) {
	con := connection.Get()
	var objects []*UserObj
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.Query(query, args...)
	if err != nil {
		return objects, err
	} else if rows.Err() != nil {
		return objects, rows.Err()
	}

	defer rows.Close()
	for rows.Next() {
		var user UserObj
		err = rows.Scan(&user.Id, &user.Name, &user.Email, &user.Income, &user.IsActive, &user.SignupDate, &user.TerminationDate)
		if err != nil {
			return objects, err
		}
		objects = append(objects, &user)
	}

	return objects, nil
}

//ReadOneByQuery returns a pointer to a(n) UserObj
func ReadOneByQuery(query string, args ...interface{}) (*UserObj, error) {
	var user UserObj

	con := connection.Get()
	query = strings.Replace(query, "'", "\"", -1)
	err := con.QueryRow(query, args...).Scan(&user.Id, &user.Name, &user.Email, &user.Income, &user.IsActive, &user.SignupDate, &user.TerminationDate)

	return &user, err
}

//Exec allows for executing queries
func Exec(query string, args ...interface{}) (sql.Result, error) {
	con := connection.Get()
	return con.Exec(query, args...)
}

```

# User_test.go - sample skeleton file generated
```go
package User_test

import (
	"testing"
)

func TestSomething(t *testing.T) {
	//test stuff here..
}
```
# examples_test.go - sample file
```go
package User_test

import (
	"fmt"
	"models/User"
	"database/sql"
)

func ExampleUserObj_Save() {
	//existing user
	user, err := User.ReadById(12345)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		user.Email = "some string"
		_, err = user.Save()
		if err != nil {
			//Save failed
		}
	}

	//new user
	user = new(User.UserObj)
	res, err := user.Save()
	if err != nil {
		//save failed
	} else {
		lastInsertId, err := res.LastInsertId()
		numRowsAffected, err := res.RowsAffected()
	}
}

func ExampleUserObj_Delete() {
	user, err := User.ReadById(12345)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		_, err = user.Delete()
		if err != nil {
			//Delete failed
		}
	}
}

func ExampleReadAll() {
	users, err := User.ReadAll("id DESC")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		for _, user := range users {
			fmt.Println(user)
		}
	}
}

func ExampleReadById() {
	user, err := User.ReadById(12345)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		//handle user object
		fmt.Println(user)
	}
}

func ExampleReadByQuery() {
	users, err := User.ReadByQuery("SELECT * FROM User WHERE email = ? ORDER BY id DESC", "some string")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		for _, user := range users {
			fmt.Println(user)
		}
	}
}

func ExampleReadOneByQuery() {
	user, err := User.ReadOneByQuery("SELECT * FROM User WHERE email = ? ORDER BY id DESC", "some string")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		//handle user object
		fmt.Println(user)
	}
}

func ExampleExec() {
	res, err := User.Exec("UPDATE User SET email = ? WHERE id = ?", "some string", 12345)
	if err != nil {
		//save failed
	} else {
		lastInsertId, err := res.LastInsertId()
		numRowsAffected, err := res.RowsAffected()
	}
}
```
# connection.go base package
```go
package connection

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

var (
	connection *sql.DB
	err        error
)

func Get() *sql.DB {
	if connection != nil {
		//determine whether connection is still alive
		err = connection.Ping()
		if err == nil {
			return connection
		}
	}

	connection, err = sql.Open("mysql", "{username}:{password}@tcp(localhost:3306)/main?parseTime=true")
	if err != nil {
		//handle connection error
	}

	return connection
}
```
