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
    	user, err := User.ReadById(12345)
    	if err != nil {
    		log.Println(err.Error())
    	}
    	
    	user.Email = "test@email.com"
    	User.Save(user)
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
    	Id		string
    	Fname		sql.NullString
    	Lname		sql.NullString
    	Phone		sql.NullString
    	Cell		sql.NullString
    	Fax		sql.NullString
    	Email		string
    }
    
    var primaryKey = "Id"
    
    func Save(Object UserObj) {
    	v := reflect.ValueOf(&Object).Elem()
    	objType := v.Type()
    
    	firstValue := reflect.Value(v.Field(1)).String()
    	if firstValue == "<sql.NullString Value>" {
    		firstValue = "null"
    	} else {
    		firstValue = "'" + firstValue + "'"
    	}
    
    	query := "UPDATE user SET " + objType.Field(1).Name + " = " + firstValue
    
    	for i := 2; i < v.NumField(); i++ {
    		property := string(objType.Field(i).Name)
    		value := reflect.Value(v.Field(i)).String()
    		if value == "<sql.NullString Value>" {
    			value = "null"
    		} else {
    			value = "'" + value + "'"
    		}
    
    		query += ", " + property + " = " + value
    	}
    	query += " WHERE " + primaryKey + " = '" + Object.Id + "'"
    
    	con := connection.GetConnection()
    	_, err := con.Exec(query)
    	if err != nil {
    		panic(err.Error())
    	}
    }
    
    func ReadById(id int) (UserObj, error) {
    	con := connection.GetConnection()
    
    	var user UserObj
    	err := con.QueryRow("SELECT * FROM user WHERE Id = ?", strconv.Itoa(id)).Scan(&user.Id, &user.Fname, &user.Lname, &user.Phone, &user.Cell, &user.Fax, &user.Email)
    
    	switch {
    	case err == sql.ErrNoRows:
    		return user, errors.New("ERROR User::ReadById - No result")
    	case err != nil:
    		return user, errors.New("ERROR User::ReadById - " + err.Error())
    	default:
    		return user, nil
    	}
    
    	return user, nil
    }
