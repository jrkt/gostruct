[![License](http://img.shields.io/:license-gpl3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0.html)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonathankentstevens/gostruct)](https://goreportcard.com/report/github.com/jonathankentstevens/gostruct)
[![GoDoc](https://godoc.org/github.com/jonathankentstevens/gostruct?status.svg)](https://godoc.org/github.com/jonathankentstevens/gostruct)
[![Build Status](https://travis-ci.org/jonathankentstevens/gostruct.svg?branch=master)](https://travis-ci.org/jonathankentstevens/gostruct)

# gostruct
Library to auto-generate packages and basic CRUD operations for a given MySQL database table.

# implementation

    go get github.com/go-sql-driver/mysql
    go get github.com/jonathankentstevens/gostruct

Create a generate.go file with the following contents (including your db username/password):

```go
package main

import (
	"github.com/jonathankentstevens/gostruct"
)

func main() {
	gs := new(gostruct.Gostruct)
	gs.Username = "root"
	gs.Password = "Jstevens120)"
	err := gs.Generate()
	if err != nil {
		println("Generate Error:", err)
	}
}
```
    
Then, run:

    go run generate.go -tables User -db main -host localhost
    
A package with a struct of the table and several methods to handle common requests will be created in the $GOPATH/src/models/{table} directory. The files that are created, for a 'User' model (for example) would be:

- User_base.go (containing the main CRUX methods and common methods such as ReadById, ReadAll, ReadOneByQuery, ReadByQuery, and Exec)
    - This also validates any enum/set data type with the value passed to ensure it is one of the required fields
- User_extended.go (this will hold any custom methods used to return User object(s))
- User_test.go to serve as a base for your unit testing
<!--- examples_test.go with auto-generated example methods for godoc readability. -->

It will also generate a connection package to share connection(s) to prevent multiple open database connections.

# flags 

tables
    
    Comma-separated list of MySQL database tables
    
db
    
    Name of the MySQL database
    
host
    
    Hostname or server of where the database is located
    
port

    Defaults to 3306 if not provided
    
all

    If this option is passed in as "true", it will run for all tables based on the database
    
nameFuncs

    Set this flag to true if you want the struct name included in the auto-generated method/function names

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
    }
	user.Email = "test@email.com"
    user.IsActive = false
    user.Save()    
    
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
# User_extended.go - sample function to include
```go
func ReadAllActive(options connection.QueryOptions) ([]*UserObj, error) {
	return ReadByQuery("SELECT * FROM User WHERE IsActive = '1'", options)
}
```
Usage:
```go
func main() {
	users, err := User.ReadAllActive(connection.QueryOptions{OrderBy: "Name ASC"})
	if err != nil {
		//handle error
	}
	
	//handle users
	for _, user := range users {
	    
	}
}
```
# User_extended.go - sample method to include
```go
func (user *UserObj) Terminate() error {
	user.IsActive = false
	user.TerminationDate = time.Now().Local()
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
# User_base.go - sample file
```go
//The User package serves as the base structure for the User table
//
//Package User contains base methods and CRUD functionality to
//interact with the User table in the main database

package User

import (
	"connection"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"reflect"
	"strings"
	"time"
	"utils"
	"utils/value"
)

//User is the structure of the home table
//
//This contains all columns that exist in the database
type User struct {
	Id              int64      `column:"id" default:"" type:"int(10) unsigned" key:"PRI" null:"NO" extra:"auto_increment"`
	Name            string     `column:"name" default:"" type:"varchar(150)" key:"" null:"NO" extra:""`
	Email           string     `column:"email" default:"" type:"varchar(250)" key:"" null:"NO" extra:""`
	Income          float64    `column:"income" default:"0" type:"decimal(10,0)" key:"" null:"NO" extra:""`
	IsActive        bool       `column:"isActive" default:"1" type:"tinyint(1)" key:"" null:"NO" extra:""`
	SignupDate      *time.Time `column:"signupDate" default:"" type:"datetime" key:"" null:"YES" extra:""`
	TerminationDate *time.Time `column:"terminationDate" default:"" type:"datetime" key:"" null:"YES" extra:""`
	Weight          *int64     `column:"weight" default:"" type:"int(11)" key:"" null:"YES" extra:""`
}

//user is the nilable structure of the home table
type user struct {
	Id              int64
	Name            string
	Email           string
	Income          float64
	IsActive        bool
	SignupDate      mysql.NullTime
	TerminationDate mysql.NullTime
	Weight          sql.NullInt64
}

//Save runs an INSERT..UPDATE ON DUPLICATE KEY and validates each value being saved
func (obj *User) Save() (sql.Result, error) {
	v := reflect.ValueOf(obj).Elem()
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
	if value.Empty(obj.Id) {
		newRecord = true
	}

	res, err := Exec(query, newArgs...)
	if err == nil && newRecord {
		id, _ := res.LastInsertId()
		obj.Id = id
	}
	return res, err
}

//Delete removes a record from the database according to the primary key
func (obj *User) Delete() (sql.Result, error) {
	return Exec("DELETE FROM User WHERE id = ?", obj.Id)
}

//ReadByKey returns a single pointer to a(n) User
func ReadByKey(id int64) (*User, error) {
	return ReadOneByQuery("SELECT * FROM User WHERE id = ?", id)
}

//ReadAll returns all records in the table
func ReadAll(options connection.QueryOptions) ([]*User, error) {
	return ReadByQuery("SELECT * FROM User", options)
}

//ReadByQuery returns an array of User pointers
func ReadByQuery(query string, args ...interface{}) ([]*User, error) {
	var objects []*User
	var err error

	con, err := connection.Get("main")
	if err != nil {
		return objects, errors.New("connection failed")
	}
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.Query(query, args...)
	if err != nil {
		return objects, err
	} else {
		rowsErr := rows.Err()
		if rowsErr != nil {
			return objects, err
		}

		defer rows.Close()
		for rows.Next() {
			var obj user
			err = rows.Scan(&obj.Id, &obj.Name, &obj.Email, &obj.Income, &obj.IsActive, &obj.SignupDate, &obj.TerminationDate, &obj.Weight)
			if err != nil {
				return objects, err
			}
			objects = append(objects, &User{obj.Id, &obj.Name, &obj.Email, &obj.Income, &obj.IsActive, &obj.SignupDate.Time, &obj.TerminationDate.Time, &obj.Weight.Int64})
		}
	}

	if len(objects) == 0 {
		err = sql.ErrNoRows
	}

	return objects, err
}

//ReadOneByQuery returns a single pointer to a(n) User
func ReadOneByQuery(query string, args ...interface{}) (*User, error) {
	var obj user

	con, err := connection.Get("main")
	if err != nil {
		return &User{}, errors.New("connection failed")
	}
	query = strings.Replace(query, "'", "\"", -1)
	err = con.QueryRow(query, args...).Scan(&obj.Id, &obj.Name, &obj.Email, &obj.Income, &obj.IsActive, &obj.SignupDate, &obj.TerminationDate, &obj.Weight)
	if err != nil && err != sql.ErrNoRows {
		return &User{}, err
	}

	return &User{obj.Id, &obj.Name, &obj.Email, &obj.Income, &obj.IsActive, &obj.SignupDate.Time, &obj.TerminationDate.Time, &obj.Weight.Int64}, nil

}

//Exec allows for update queries
func Exec(query string, args ...interface{}) (sql.Result, error) {
	con, err := connection.Get("main")
	if err != nil {
		var result sql.Result
		return result, errors.New("connection failed")
	}
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