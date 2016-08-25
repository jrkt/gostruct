# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table and all other tables related through all foreign key relationships. 

# usage

    go get github.com/jonathankentstevens/gostruct

Replace the {username} and {password} constants in gostruct.go to the credentials of your database. Then create a generate.go file with the following contents:

    package main

    import (
    	_ "github.com/go-sql-driver/mysql"
    	"github.com/jonathankentstevens/gostruct"
    	"log"
    )
    
    func main() {
    	err := gostruct.Generate()
    	if err != nil {
    		log.Fatalln(err)
    	}
    }
    
Then, run:

    go run generate.go -table User -database main -host localhost
    
A package with a struct and a method to read by the primary key as well as a method to handle updating the record will be created in the $GOPATH/src/models/{table} directory. It will also build packages for any other tables that have foreign keys of the table give. In addition, it will generate a connection package to share a connection between all your models to prevent multiple open database connections.


# implementation

    package main

    import (
    	"models/User"
    	"log"
    	"fmt"
    )
    
    func main() {
        //retrieve existing user by id
    	user := User.ReadById(12345)
    	user.Email = "test@email.com"
    	user.Save()
    	
    	//create new user
    	user := User.UserObj{}
    	user.Email = "test@email.com"
    	user.Save()
    	
    	//delete user
    	user.Delete()
    }

# flags 

table
    
    MySQL database table
    
database
    
    Name of the MySQL database
    
host
    
    Hostname or server of where the database is located
    
port

    Defaults to 3306 if not provided
    
all

    If this option is passed in as "true", it will run for all tables based on the database

# sample file - User_Crux.go

    package User

    import (
    	"database/sql"
    	"connection"
    	"reflect"
    	"strconv"
    	"errors"
    )
    
    type UserObj struct {
    	Id		    string          `column:"id"`
    	Fname		sql.NullString  `column:"fname"`
    	Lname		sql.NullString  `column:"lname"`
    	Phone		sql.NullString  `column:"phone"`
    	Cell		sql.NullString  `column:"cell"`
    	Fax		    sql.NullString  `column:"fax"`
    	Email		string          `column:"email"`
    }
    
    var primaryKey = "id"
    
    func (Object UserObj) Save() {
    	v := reflect.ValueOf(&Object).Elem()
    	objType := v.Type()
    
    	var firstValue string
    	if v.Field(1).Type() == reflect.TypeOf(sql.NullString{}) {
    		if reflect.Value(v.Field(1)).Field(0).String() == "" {
    			firstValue = "null"
    		} else {
    			firstValue = "'" + reflect.Value(v.Field(1)).Field(0).String() + "'"
    		}
    	} else {
    		if reflect.Value(v.Field(0)).String() == "" {
    			firstValue = "null"
    		} else {
    			firstValue = "'" + reflect.Value(v.Field(1)).String() + "'"
    		}
    	}
    
    	var values string
    	var columns string
    	var query string
    
    	if Object.Id == "" {
    		query = "INSERT INTO User "
    		columns += "("
    		if firstValue != "null" {
    			columns += string(objType.Field(1).Tag.Get("column")) + ","
    			values += firstValue + ","
    		}
    	} else {
    		query = "UPDATE User SET " + string(objType.Field(1).Tag.Get("column")) + " = " + firstValue
    	}
    
    	for i := 2; i < v.NumField(); i++ {
    		propType := v.Field(i).Type()
    		value := ""
    		if propType == reflect.TypeOf(sql.NullString{}) {
    			if reflect.Value(v.Field(i)).Field(0).String() == "" {
    				value = "null"
    			} else {
    				value = "'" + reflect.Value(v.Field(i)).Field(0).String() + "'"
    			}
    		} else {
    			if reflect.Value(v.Field(i)).String() == "" {
    				value = "null"
    			} else {
    				value = "'" + reflect.Value(v.Field(i)).String() + "'"
    			}
    		}
    
    		if Object.Id == "" {
    			if value != "null" {
    				columns += string(objType.Field(i).Tag.Get("column")) + ","
    				values += value + ","
    			}
    		} else {
    			query += ", " + string(objType.Field(i).Tag.Get("column")) + " = " + value
    		}
    	}
    	if Object.Id == "" {
    		query += columns[:len(columns) - 1] + ") VALUES (" + values[:len(values) - 1] + ")"
    	} else {
    		query += " WHERE " + primaryKey + " = '" + Object.Id + "'"
    	}
    
    	con := connection.GetConnection()
    	_, err := con.Exec(query)
    	if err != nil {
    		panic(err.Error())
    	}
    }
    
    func (Object UserObj) Delete() {
    	query := "DELETE FROM User WHERE id = '" + Object.Id + "'"
    
    	con := connection.GetConnection()
    	_, err := con.Exec(query)
    	if err != nil {
    		panic(err.Error())
    	}
    }
    
    func ReadById(id int) UserObj {
    	con := connection.GetConnection()
    
    	var user UserObj
    	con.QueryRow("SELECT * FROM user WHERE id = ?", strconv.Itoa(id)).Scan(&user.Id, &user.Fname, &user.Lname, &user.Phone, &user.Cell, &user.Fax, &user.Email)
    
    	return user
    }
    
    func ReadByQuery(query string) []UserObj {
    	connection := db.GetConnection()
    	var objects []UserObj
    	rows, err := connection.Query(query)
    	if err != nil {
    		panic(err)
    	} else {
    		for rows.Next() {
    			var user UserObj
    			rows.Scan(&user.Id, &user.Fname, &user.Lname, &user.Phone, &user.Cell, &user.Fax, &user.Email)
    			objects = append(objects, user)
    		}
    		err = rows.Err()
    		if err != nil {
    			panic(err)
    		}
    		rows.Close()
    	}
    
    	return objects
    }
    
    func ReadOneByQuery(query string) UserObj {
    	var object UserObj
    
    	con := db.GetConnection()
    	err := con.QueryRow(query).Scan(&user.Id, &user.Fname, &user.Lname, &user.Phone, &user.Cell, &user.Fax, &user.Email)
    
    	switch {
    	case err == sql.ErrNoRows:
    	//do something?
    	case err != nil:
    		panic(err)
    	}
    
    	return object
    }
