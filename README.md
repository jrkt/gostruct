# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table and all other tables related through all foreign key relationships. 

# usage
    go get github.com/go-sql-driver/mysql
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
    
A package with a struct of the table and several methods to handle common requests will be created in the $GOPATH/src/models/{table} directory. The files that are created, for a 'User' model (for example) would be: CRUX_User.go (containing the main CRUX methods and common methods such as ReadById, ReadAll, ReadOneByQuery and ReadByQuery), DAO_User.go (this will hold any custom methods used to return User object(s)), BO_User.go (this contains methods to be called on the User object itself), and User_test.go to serve in unit testing. In addition, it will generate a connection package to share a connection between all your models to prevent multiple open database connections and a date package to implement a "sql.NullTime"-like type.

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

# implementation

    package main

    import (
    	"models/User"
    )
    
    func main() {
        //retrieve existing user by id
    	user := User.ReadById(12345)
    	user.Email = "test@email.com"
        user.IsActive = false
    	user.Save()
    	
    	//create new user
    	user := User.UserObj{}
    	user.Email = "test@email.com"
    	user.Save()
    	
    	//delete user
    	user.Delete()
    }
    
# CRUX_User.go - sample file

    package User

    import (
        "connection"
        "database/sql"
        "date"
        "reflect"
        "strconv"
        "strings"
        "time"
    )

    type UserObj struct {
        Id         int       `column:"id"`
        Name       string    `column:"name"`
        Email      string    `column:"email"`
        Income     float64   `column:"income"`
        IsActive   bool      `column:"isActive"`
        SignupDate time.Time `column:"signupDate"`
    }

    func (Object UserObj) Save() error {
        v := reflect.ValueOf(&Object).Elem()
        objType := v.Type()

        values := ""
        columns := ""

        var query string

        if strconv.Itoa(Object.Id) == "" {
            query = "INSERT INTO User "
            firstValue := getFieldValue(v.Field(0))
            if firstValue != "null" {
                columns += string(objType.Field(0).Tag.Get("column"))
                values += firstValue
            }
        } else {
            query = "UPDATE User SET "
        }

        for i := 1; i < v.NumField(); i++ {
            value := getFieldValue(v.Field(i))

            if strconv.Itoa(Object.Id) == "" {
                if value != "null" {
                    if i > 1 {
                        columns += ","
                        values += ","
                    }
                    columns += string(objType.Field(i).Tag.Get("column"))
                    values += value
                }
            } else {
                if i > 1 {
                    query += ", "
                }
                query += string(objType.Field(i).Tag.Get("column")) + " = " + value
            }
        }
        if strconv.Itoa(Object.Id) == "" {
            query += "(" + columns + ") VALUES (" + values + ")"
        } else {
            query += " WHERE id = \"" + strconv.Itoa(Object.Id) + "\""
        }

        con := connection.Get()
        _, err := con.Exec(query)
        if err != nil {
            return err
        }

        return nil
    }

    func getFieldValue(field reflect.Value) string {
        var value string

        switch t := field.Interface().(type) {
        case string:
            value = t
        case int:
            value = strconv.Itoa(t)
        case int64:
            value = strconv.FormatInt(t, 10)
        case float64:
            value = strconv.FormatFloat(t, 'f', -1, 64)
        case bool:
            if t {
                value = "1"
            } else {
                value = "0"
            }
        case time.Time:
            value = t.Format(date.DEFAULT_FORMAT)
        case sql.NullString:
            value = t.String
        case sql.NullInt64:
            if t.Int64 == 0 {
                value = ""
            } else {
                value = strconv.FormatInt(t.Int64, 10)
            }
        case sql.NullFloat64:
            value = strconv.FormatFloat(t.Float64, 'f', -1, 64)
        case sql.NullBool:
            if t.Bool {
                value = "1"
            } else {
                value = "0"
            }
        case date.NullTime:
            value = t.Time.Format(date.DEFAULT_FORMAT)
        }

        if value == "" {
            value = "null"
        } else {
            value = "\"" + strings.Replace(value, `"`, `\"`, -1) + "\""
        }

        return value
    }

    func (Object UserObj) Delete() error {
        query := "DELETE FROM User WHERE id = \"" + strconv.Itoa(Object.Id) + "\""

        con := connection.Get()
        _, err := con.Exec(query)
        if err != nil {
            return err
        }

        return nil
    }

    func ReadById(id int) UserObj {
        return ReadOneByQuery("SELECT * FROM User WHERE id = '" + strconv.Itoa(id) + "'")
    }

    func ReadAll(order string) []UserObj {
        return ReadByQuery("SELECT * FROM User", order)
    }

    func ReadByQuery(query string, order string) []UserObj {
        connection := connection.Get()
        objects := []UserObj{}
        if order != "" {
            query += " ORDER BY " + order
        }
        query = strings.Replace(query, "'", "\"", -1)
        rows, err := connection.Query(query)
        if err != nil {
            panic(err)
        } else {
            for rows.Next() {
                var object UserObj
                rows.Scan(&object.Id, &object.Name, &object.Email, &object.Income, &object.IsActive, &object.SignupDate)
                objects = append(objects, object)
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

        con := connection.Get()
        query = strings.Replace(query, "'", "\"", -1)
        err := con.QueryRow(query).Scan(&object.Id, &object.Name, &object.Email, &object.Income, &object.IsActive, &object.SignupDate)

        switch {
        case err == sql.ErrNoRows:
        //do something?
        case err != nil:
            panic(err)
        }

        return object
    }

# DAO_User.go - sample method to include

    func ReadAllActive(order string) []UserObj {
        return ReadByQuery("SELECT * FROM User WHERE IsActive = '1'", order)
    }
    
How to call:

    func main() {
        users := User.ReadAllActive("Name ASC")
        fmt.Println(users)
    }

# BO_User.go - sample method to include

    func (user UserObj) Terminate() {
        user.IsActive = false
        user.TerminationDate = time.Now()
        user.Save()
    }

How to call:

    func main() {
        users := User.ReadAllActive("Name ASC")
        for i := range users {
            user := users[i]
            user.Terminate()
        }
    }
