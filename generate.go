/*
A package with a struct of the table and several methods to handle common requests will be created in the $GOPATH/src/models/{table} directory. The files that are created, for a 'User' model (for example) would be:

- CRUX_User.go (containing the main CRUX methods and common methods such as ReadById, ReadAll, ReadOneByQuery, ReadByQuery, and Exec). This also validates any enum/set data type with the value passed to ensure it is one of the required fields

- DAO_User.go (this will hold any custom methods used to return User object(s))

- BO_User.go (this contains methods to be called on the User object itself)

- User_test.go to serve as a base for your unit testing

- examples_test.go with auto-generated example methods for godoc readability.

It will also generate a connection package to share connection(s) to prevent multiple open database connections.

# implementation

        go get github.com/go-sql-driver/mysql
        go get github.com/jonathankentstevens/gostruct

Create a generate.go file with the following contents (including your db username/password):

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

Then, run:

        go run generate.go -tables User -db main -host localhost
*/
package gostruct

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	//allows pulling from information_schema database
	_ "github.com/go-sql-driver/mysql"
)

//TableObj is the result set returned from the MySQL information_schema that
//contains all data for a specific table
type TableObj struct {
	Name       string
	IsNullable string
	Key        string
	DataType   string
	ColumnType string
	Default    sql.NullString
	Extra      sql.NullString
}

//Table houses the name of the table
type Table struct {
	Name string
}

type usedColumn struct {
	Name string
}

type uniqueValues struct {
	Value sql.NullString
}

//Globals variables
var (
	err              error
	con              *sql.DB
	tables           []string
	tablesDone       []string
	primaryKey       string
	GOPATH           string
	exampleIdStr     string
	exampleColumn    string
	exampleColumnStr string
	exampleOrderStr  string
)

//initialize global GOPATH
func init() {
	GOPATH = os.Getenv("GOPATH")
}

//Run generates a package for a single table
func (gs *Gostruct) Run(table string) error {
	//make sure models dir exists
	if !exists(GOPATH + "/src/models") {
		err = gs.CreateDirectory(GOPATH + "/src/models")
		if err != nil {
			return err
		}
	}

	err = gs.buildConnectionPackage()
	if err != nil {
		return err
	}

	//handle utils file
	err = gs.buildUtilsPackage()
	if err != nil {
		return err
	}

	err = gs.handleTable(table)
	if err != nil {
		return err
	}

	return nil
}

//RunAll generates packages for all tables in a specific database and host
func (gs *Gostruct) RunAll() error {
	connection, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", gs.Username, gs.Password, gs.Host, gs.Port, gs.Database))
	if err != nil {
		panic(err)
	} else {
		rows, err := connection.Query("SELECT DISTINCT(TABLE_NAME) FROM `information_schema`.`COLUMNS` WHERE `TABLE_SCHEMA` LIKE ?", gs.Database)
		if err != nil {
			panic(err)
		} else {
			for rows.Next() {
				var table Table
				rows.Scan(&table.Name)

				err = gs.Run(table.Name)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

//CreateDirectory creates directory and sets permissions to 0777
func (gs *Gostruct) CreateDirectory(path string) error {
	err = os.Mkdir(path, 0777)
	if err != nil {
		return err
	}

	//give new directory full permissions
	err = os.Chmod(path, 0777)
	if err != nil {
		return err
	}

	return nil
}

//Main handler method for tables
func (gs *Gostruct) handleTable(table string) error {
	if inArray(table, tablesDone) {
		return nil
	}
	tablesDone = append(tablesDone, table)

	log.Println("Generating Models for: " + table)

	con, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", gs.Username, gs.Password, gs.Host, gs.Port, gs.Database))

	if err != nil {
		return err
	}

	rows1, err := con.Query("SELECT column_name, is_nullable, column_key, data_type, column_type, column_default, extra FROM information_schema.columns WHERE table_name = ? AND table_schema = ?", table, gs.Database)

	var object TableObj
	var objects []TableObj = make([]TableObj, 0)
	var columns []string

	if err != nil {
		return err
	}
	cntPK := 0
	for rows1.Next() {
		rows1.Scan(&object.Name, &object.IsNullable, &object.Key, &object.DataType, &object.ColumnType, &object.Default, &object.Extra)
		objects = append(objects, object)
		if object.Key == "PRI" {
			cntPK++
		}
		if cntPK > 1 && object.Key == "PRI" {
			continue
		}
		columns = append(columns, object.Name)
	}
	defer rows1.Close()

	primaryKey = ""
	if len(objects) == 0 {
		return errors.New("No results for table: " + table)
	}

	//get PrimaryKey
	for i := 0; i < len(objects); i++ {
		object := objects[i]
		if object.Key == "PRI" {
			primaryKey = object.Name
			break
		}
	}

	//create directory
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	if !exists(dir) {
		err := os.Mkdir(dir, 0777)
		if err != nil {
			return err
		}

		//give new directory full permissions
		err = os.Chmod(dir, 0777)
		if err != nil {
			return err
		}
	}

	//handle CRUX file
	err = gs.buildCruxFile(objects, table)
	if err != nil {
		return err
	}

	//handle DAO file
	err = gs.buildDaoFile(table)
	if err != nil {
		return err
	}

	//handle BO file
	err = gs.buildBoFile(table)
	if err != nil {
		return err
	}

	//handle Test file
	err = gs.buildTestFile(table)
	if err != nil {
		return err
	}

	//handle Example file
	err = gs.buildExamplesFile(table)
	if err != nil {
		return err
	}

	return nil
}

//Builds CRUX_{table}.go file with main struct and CRUD functionality
func (gs *Gostruct) buildCruxFile(objects []TableObj, table string) error {
	exampleIdStr = ""
	exampleColumn = ""
	exampleColumnStr = ""
	exampleOrderStr = ""

	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var usedColumns []usedColumn
	initialString := `//Package ` + uppercaseFirst(table) + ` serves as the base structure for the ` + table + ` table
//and contains base methods and CRUD functionality to
//interact with the ` + table + ` table in the ` + gs.Database + ` database
package ` + uppercaseFirst(table)
	importString := `

import (
	"database/sql"
	"strings"
	"connection"
	"reflect"
	"utils"`

	string1 := `

//` + uppercaseFirst(table) + `Obj is the structure of the ` + table + ` table
type ` + uppercaseFirst(table) + "Obj struct {"
	string2 := ""
	contents := ""

	primaryKeys := []string{}
	primaryKeyTypes := []string{}

	importTime := false
	importMysql := false

	var questionMarks []string

Loop:
	for i := 0; i < len(objects); i++ {
		object := objects[i]
		for c := 0; c < len(usedColumns); c++ {
			if usedColumns[c].Name == object.Name {
				continue Loop
			}
		}
		usedColumns = append(usedColumns, usedColumn{Name: object.Name})
		questionMarks = append(questionMarks, "?")

		isBool := true
		var dataType string
		switch object.DataType {
		case "int":
			isBool = false
			if object.IsNullable == "NO" {
				dataType = "int"
			} else {
				dataType = "sql.NullInt64"
			}
		case "tinyint":
			rows, err := con.Query("SELECT DISTINCT(`" + object.Name + "`) FROM " + gs.Database + "." + table)
			if err != nil {
				return err
			}
			for rows.Next() {
				var uObj uniqueValues
				rows.Scan(&uObj.Value)
				if uObj.Value.String != "0" && uObj.Value.String != "1" {
					isBool = false
				}
			}

			if isBool {
				if object.IsNullable == "NO" {
					dataType = "bool"
				} else {
					dataType = "sql.NullBool"
				}
			} else {
				if object.IsNullable == "NO" {
					dataType = "int"
				} else {
					dataType = "sql.NullInt64"
				}
			}
		case "bool", "boolean":
			if object.IsNullable == "NO" {
				dataType = "bool"
			} else {
				dataType = "sql.NullBool"
			}
		case "float", "decimal":
			isBool = false
			if object.IsNullable == "NO" {
				dataType = "float64"
			} else {
				dataType = "sql.NullFloat64"
			}
		case "date", "datetime", "timestamp":
			isBool = false
			if object.IsNullable == "NO" {
				importTime = true
				dataType = "time.Time"
			} else {
				importMysql = true
				dataType = "mysql.NullTime"
			}
		default:
			isBool = false
			if object.IsNullable == "NO" {
				dataType = "string"
				if i > 1 && exampleColumn == "" {
					exampleColumn = object.Name
					exampleColumnStr = uppercaseFirst(object.Name) + ` = "some string"`
				}
			} else {
				dataType = "sql.NullString"
				if i > 1 && exampleColumn == "" {
					exampleColumn = object.Name
					exampleColumnStr = uppercaseFirst(object.Name) + `.String = "some string"`
				}
			}
		}

		if object.Key == "PRI" {
			if isBool {
				if object.IsNullable == "NO" {
					dataType = "int"
				} else {
					dataType = "sql.NullInt64"
				}
			}
			if dataType == "string" || dataType == "sql.NullString" {
				exampleIdStr = `"12345"`
			} else if dataType == "int" || dataType == "sql.NullInt64" {
				exampleIdStr = `12345`
			}
			if exampleOrderStr == "" {
				exampleOrderStr = object.Name + ` DESC`
			}
			primaryKeys = append(primaryKeys, object.Name)
			primaryKeyTypes = append(primaryKeyTypes, dataType)
		}

		if i > 0 {
			string2 += ", &" + strings.ToLower(table) + "." + uppercaseFirst(object.Name)
		}
		defaultVal := ""
		if strings.ToLower(object.Default.String) != "null" {
			if object.Default.String == "0" && object.IsNullable == "YES" {
				defaultVal = ""
			} else {
				defaultVal = object.Default.String
			}
		}
		string1 += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\" default:\"" + defaultVal + "\" type:\"" + object.ColumnType + "\" key:\"" + object.Key + "\" extra:\"" + object.Extra.String + "\"`"
	}
	string1 += "\n}"

	if importTime {
		importString += `
		"time"`
	}
	if importMysql {
		importString += `
		"github.com/go-sql-driver/mysql"`
	}

	bs := "`"

	if len(primaryKeys) > 0 {
		string1 += `

//Save does just that. It will save if the object key exists, otherwise it will add the record
//by running INSERT ON DUPLICATE KEY UPDATE
func (` + strings.ToLower(table) + ` *` + uppercaseFirst(table) + `Obj) Save() (sql.Result, error) {
	v := reflect.ValueOf(` + strings.ToLower(table) + `).Elem()
	objType := v.Type()

	var columnArr []string
	var args []interface{}
	var q []string

	updateStr := ""
	query := "INSERT INTO ` + table + `"
	for i := 0; i < v.NumField(); i++ {
		val, err := utils.ValidateField(v.Field(i), objType.Field(i))
		if err != nil {
			return nil, err
		}
		args = append(args, val)
		column := string(objType.Field(i).Tag.Get("column"))
		columnArr = append(columnArr, "` + bs + `"+column+"` + bs + `")
		q = append(q, "?")
		if i > 0 && updateStr != "" {
			updateStr += ", "
		}
		updateStr += "` + bs + `" + column + "` + bs + ` = ?"
	}

	query += " (" + strings.Join(columnArr, ", ") + ") VALUES (" + strings.Join(q, ", ") + ") ON DUPLICATE KEY UPDATE " + updateStr
	newArgs := append(args, args...)`

		if len(primaryKeys) > 1 {
			string1 += `

	return Exec(query, newArgs...)`
		} else {
			var insertIdStr string
			switch primaryKeyTypes[0] {
			case "string":
				insertIdStr = "strconv.FormatInt(id, 10)"
				importString += `
				"strconv"`
			default:
				insertIdStr = `int(id)`
			}

			string1 += `
	newRecord := false
	if utils.Empty(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `) {
		newRecord = true
	}

	res, err := Exec(query, newArgs...)
	if err == nil && newRecord {
		id, _ := res.LastInsertId()
		` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + ` = ` + insertIdStr + `
	}
	return res, err`
		}

		string1 += `
}`

		whereStrQuery, whereStrQueryValues := "", ""
		for k := range primaryKeys {
			if k > 0 {
				whereStrQuery += " AND"
				whereStrQueryValues += ","
			}
			whereStrQuery += ` ` + primaryKeys[k] + ` = ?`
			whereStrQueryValues += ` ` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[k])
		}

		string1 += `

//Delete does just that. It removes that record from the database based on the primary key.
func (` + strings.ToLower(table) + ` *` + uppercaseFirst(table) + `Obj) Delete() (sql.Result, error) {
	return Exec("DELETE FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrQueryValues + `)
}
`
		paramStr, whereStrValues := "", ""
		for k := range primaryKeys {
			var param string
			if primaryKeys[k] == "type" {
				param = "objType"
			} else if primaryKeys[k] == "typeId" {
				param = "objTypeId"
			} else {
				param = primaryKeys[k]
			}

			var dataType string
			//var paramTypeStr string
			switch primaryKeyTypes[k] {
			case "int":
				dataType = "int"
				//paramTypeStr = "strconv.Itoa(" + param + ")"
			case "float64":
				dataType = "float64"
				//paramTypeStr = "strconv.FormatFloat(" + param + ", 'f', -1, 64)"
			default:
				dataType = "string"
				//paramTypeStr = param
			}

			paramStr += param + " " + dataType
			if k > 0 {
				//whereStr += " AND"
				whereStrValues += ","
			}
			if k != len(primaryKeys)-1 {
				//whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'"`
				//} else {
				paramStr += ", "
				//whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'`
			}
			whereStrValues += " " + param
		}

		string1 += `
//ReadById returns a pointer to a(n) ` + uppercaseFirst(table) + `Obj
func ReadById(` + paramStr + `) (*` + uppercaseFirst(table) + `Obj, error) {
	return ReadOneByQuery("SELECT * FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrValues + `)
}`
	}

	string1 += `

//ReadAll returns all records in the ` + uppercaseFirst(table) + ` table
func ReadAll(order string) ([]*` + uppercaseFirst(table) + `Obj, error) {
	query := "SELECT * FROM ` + table + `"
	if order != "" {
		query += " ORDER BY " + order
	}
	return ReadByQuery(query)
}`

	string1 += `

//ReadByQuery returns an array of ` + uppercaseFirst(table) + `Obj pointers
func ReadByQuery(query string, args ...interface{}) ([]*` + uppercaseFirst(table) + `Obj, error) {
	con := connection.Get("` + gs.Database + `")
	var objects []*` + uppercaseFirst(table) + `Obj
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.Query(query, args...)
	if err != nil {
		return objects, err
	} else if rows.Err() != nil {
		return objects, rows.Err()
	}

	defer rows.Close()
	for rows.Next() {
		var ` + strings.ToLower(table) + ` ` + uppercaseFirst(table) + `Obj
		err = rows.Scan(&` + strings.ToLower(table) + `.` + uppercaseFirst(objects[0].Name) + string2 + `)
		if err != nil {
			return objects, err
		}
		objects = append(objects, &` + strings.ToLower(table) + `)
	}

	return objects, nil
}

//ReadOneByQuery returns a pointer to a(n) ` + uppercaseFirst(table) + `Obj
func ReadOneByQuery(query string, args ...interface{}) (*` + uppercaseFirst(table) + `Obj, error) {
	var ` + strings.ToLower(table) + ` ` + uppercaseFirst(table) + `Obj

	con := connection.Get("` + gs.Database + `")
	query = strings.Replace(query, "'", "\"", -1)
	err := con.QueryRow(query, args...).Scan(&` + strings.ToLower(table) + `.` + uppercaseFirst(objects[0].Name) + string2 + `)

	return &` + strings.ToLower(table) + `, err
}

//Exec allows for executing queries
func Exec(query string, args ...interface{}) (sql.Result, error) {
	con := connection.Get("` + gs.Database + `")
	return con.Exec(query, args...)
}`

	importString += "\n)"
	contents = initialString + importString + string1

	cruxFilePath := dir + "CRUX_" + tableNaming + ".go"
	err = writeFile(cruxFilePath, contents, true)
	if err != nil {
		return err
	}

	_, err = runCommand("go fmt " + cruxFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds DAO_{table}.go file for custom Data Access Object methods
func (gs *Gostruct) buildDaoFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	daoFilePath := dir + "DAO_" + tableNaming + ".go"

	if !exists(daoFilePath) {
		contents := "package " + uppercaseFirst(table) + "\n\n//Methods Here"
		err = writeFile(daoFilePath, contents, false)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + daoFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds BO_{table}.go file for custom Business Object methods
func (gs *Gostruct) buildBoFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	boFilePath := dir + "BO_" + tableNaming + ".go"

	if !exists(boFilePath) {
		contents := "package " + uppercaseFirst(table) + "\n\n//Methods Here"
		err = writeFile(boFilePath, contents, false)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + boFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds {table}_test.go file
func (gs *Gostruct) buildTestFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	testFilePath := dir + tableNaming + "_test.go"

	if !exists(testFilePath) {
		contents := `package ` + tableNaming + `_test

		import (
			"testing"
		)

		func TestSomething(t *testing.T) {
			//test stuff here..
		}`
		err = writeFile(testFilePath, contents, false)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + testFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds {table}_test.go file
func (gs *Gostruct) buildExamplesFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	examplesFilePath := dir + "examples_test.go"

	if !exists(examplesFilePath) {
		contents := `package ` + tableNaming + `_test

import (
	"fmt"
	"models/` + tableNaming + `"
	"database/sql"
)

func Example` + tableNaming + `Obj_Save() {
	//existing ` + strings.ToLower(table) + `
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		` + strings.ToLower(table) + `.` + exampleColumnStr + `
		_, err = ` + strings.ToLower(table) + `.Save()
		if err != nil {
			//Save failed
		}
	}

	//new ` + strings.ToLower(table) + `
	` + strings.ToLower(table) + ` = new(` + tableNaming + `.` + tableNaming + `Obj)
	res, err := ` + strings.ToLower(table) + `.Save()
	if err != nil {
		//save failed
	} else {
		lastInsertId, err := res.LastInsertId()
		numRowsAffected, err := res.RowsAffected()
	}
}

func Example` + tableNaming + `Obj_Delete() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		_, err = ` + strings.ToLower(table) + `.Delete()
		if err != nil {
			//Delete failed
		}
	}
}

func ExampleReadAll() {
	` + strings.ToLower(table) + `s, err := ` + tableNaming + `.ReadAll("` + exampleOrderStr + `")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		for _, user := range ` + strings.ToLower(table) + `s {
			fmt.Println(` + strings.ToLower(table) + `)
		}
	}
}

func ExampleReadById() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		//handle ` + strings.ToLower(table) + ` object
		fmt.Println(` + strings.ToLower(table) + `)
	}
}

func ExampleReadByQuery() {
	` + strings.ToLower(table) + `s, err := ` + tableNaming + `.ReadByQuery("SELECT * FROM ` + table + ` WHERE ` + exampleColumn + ` = ? ORDER BY ` + exampleOrderStr + `", "some string")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		for _, user := range ` + strings.ToLower(table) + `s {
			fmt.Println(` + strings.ToLower(table) + `)
		}
	}
}

func ExampleReadOneByQuery() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadOneByQuery("SELECT * FROM ` + table + ` WHERE ` + exampleColumn + ` = ? ORDER BY ` + exampleOrderStr + `", "some string")
	if err != nil {
		if err == sql.ErrNoRows {
			//no results
		} else {
			//query or mysql error
		}
	} else {
		//handle ` + strings.ToLower(table) + ` object
		fmt.Println(` + strings.ToLower(table) + `)
	}
}

func ExampleExec() {
	res, err := ` + tableNaming + `.Exec("UPDATE ` + table + ` SET ` + exampleColumn + ` = ? WHERE id = ?", "some string", ` + exampleIdStr + `)
	if err != nil {
		//save failed
	} else {
		lastInsertId, err := res.LastInsertId()
		numRowsAffected, err := res.RowsAffected()
	}
}`
		err = writeFile(examplesFilePath, contents, false)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + examplesFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds utils file
func (gs *Gostruct) buildUtilsPackage() error {
	filePath := GOPATH + "/src/utils/utils.go"
	if !exists(GOPATH + "/src/utils") {
		err = gs.CreateDirectory(GOPATH + "/src/utils")
		if err != nil {
			return err
		}
	}

	if !exists(filePath) {
		contents := `package utils

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"errors"
	"reflect"
	"strings"
	"time"
)

//Determine whether or not an object is empty
func Empty(val interface{}) bool {
	empty := true
	switch val.(type) {
	case string, int, int64, float64, bool, time.Time:
		empty = isEmpty(val)
	default:
		v := reflect.ValueOf(val).Elem()
		if v.String() == "<invalid Value>" {
			return true
		}
		for i := 0; i < v.NumField(); i++ {
			var value interface{}
			field := reflect.Value(v.Field(i))

			switch field.Interface().(type) {
			case string:
				value = field.String()
			case int, int64:
				value = field.Int()
			case float64:
				value = field.Float()
			case bool:
				value = field.Bool()
			case time.Time:
				value = field.Interface()
			case sql.NullString:
				value = field.Field(0).String()
			case sql.NullInt64:
				value = field.Field(0).Int()
			case sql.NullFloat64:
				value = field.Field(0).Float()
			case sql.NullBool:
				value = field.Field(0).Bool()
			case mysql.NullTime:
				value = field.Field(0).Interface()
			default:
				value = field.Interface()
			}

			if !isEmpty(value) {
				empty = false
				break
			}
		}
	}
	return empty
}

func isEmpty(val interface{}) bool {
	empty := false
	switch v := val.(type) {
	case string:
		if v == "" {
			empty = true
		}
	case int:
		if v == 0 {
			empty = true
		}
	case int64:
		if int(int64(v)) == 0 {
			empty = true
		}
	case float64:
		if int(float64(v)) == 0 {
			empty = true
		}
	case bool:
		if v == false {
			empty = true
		}
	case time.Time:
		if v.String() == "0001-01-01 00:00:00 +0000 UTC" {
			empty = true
		}
	}
	return empty
}

//Validate field value
func ValidateField(val reflect.Value, field reflect.StructField) (interface{}, error) {
	if strings.Contains(string(field.Tag.Get("type")), "enum") {
		var s string
		switch t := val.Interface().(type) {
		case string:
			s = t
		case sql.NullString:
			s = t.String
		}
		vals := Between(string(field.Tag.Get("type")), "enum('", "')")
		arr := strings.Split(vals, "','")
		if !InArray(s, arr) {
			return nil, errors.New("Invalid value: '" + s + "' for column: " + string(field.Tag.Get("column")) + ". Possible values are: " + strings.Join(arr, ", "))
		}
	}

	return GetFieldValue(val, field.Tag.Get("default")), nil
}

//Returns string between two specified characters/strings
func Between(initial string, beginning string, end string) string {
	return strings.TrimLeft(strings.TrimRight(initial, end), beginning)
}

//Determine whether or not a string is in array
func InArray(char string, strings []string) bool {
	for _, a := range strings {
		if a == char {
			return true
		}
	}
	return false
}

//Returns the value from the struct field value as an interface
func GetFieldValue(field reflect.Value, defaultVal string) interface{} {
	var val interface{}

	switch t := field.Interface().(type) {
	case string:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case int:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case int64:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case float64:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case bool:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case time.Time:
		if !Empty(t) {
			val = t
		} else {
			val = defaultVal
		}
	case sql.NullString:
		val = NewNullString(t.String)
	case sql.NullInt64:
		val = NewNullInt(t.Int64)
	case sql.NullFloat64:
		val = NewNullFloat(t.Float64)
	case sql.NullBool:
		val = NewNullBool(t.Bool)
	case mysql.NullTime:
		val = NewNullTime(t.Time)
	}

	return val
}

func NewNullString(s string) sql.NullString {
	if Empty(s) {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func NewNullInt(i int64) sql.NullInt64 {
	if Empty(i) {
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}

func NewNullFloat(f float64) sql.NullFloat64 {
	if Empty(f) {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{
		Float64: f,
		Valid:   true,
	}
}

func NewNullBool(b bool) sql.NullBool {
	if Empty(b) {
		return sql.NullBool{}
	}
	return sql.NullBool{
		Bool:  b,
		Valid: true,
	}
}

func NewNullTime(t time.Time) mysql.NullTime {
	if Empty(t) {
		return mysql.NullTime{}
	}
	return mysql.NullTime{
		Time:  t,
		Valid: true,
	}
}
`

		err = writeFile(filePath, contents, false)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + filePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds main connection package for serving up all database connections
//with a shared connection pool
func (gs *Gostruct) buildConnectionPackage() error {
	if !exists(GOPATH + "/src/connection") {
		err = gs.CreateDirectory(GOPATH + "/src/connection")
		if err != nil {
			return err
		}
	}

	conFilePath := GOPATH + "/src/connection/connection.go"
	contents := `//Package connection handles all connections to the MySQL database(s)
package connection

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"sync"
)

var (
	err         error
	connections *Connections
)

func init() {
	connections = &Connections{
		list: make(map[string]*sql.DB),
	}
}

//Connections holds the list of database connections
type Connections struct {
	list map[string]*sql.DB
	sync.Mutex
}

//Get returns a connection to a specific database. If the connection exists in the connections list AND is
//still active, it will just return that connection. Otherwise, it will open a new connection to
//the specified database and add it to the connections list.
func Get(db string) *sql.DB {

	connection := connections.list[db]
	if connection != nil {
		//determine if connection is still active
		err = connection.Ping()
		if err == nil {
			return connection
		}
	}

	con, err := sql.Open("mysql", fmt.Sprintf("root:Jstevens120)@tcp(localhost:3306)/%s?parseTime=true", db))
	sql.Open("mysql", fmt.Sprintf("` + gs.Username + `:` + gs.Password + `@tcp(` + gs.Host + `:3306)/%s?parseTime=true", db))
	if err != nil {
		//do whatever tickles your fancy here
		log.Fatalln("Connection Error to DB [", db, "]", err.Error())
	}
	con.SetMaxIdleConns(10)
	con.SetMaxOpenConns(500)

	connections.Lock()
	connections.list[db] = con
	connections.Unlock()

	return con
}
`
	err = writeFile(conFilePath, contents, false)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + conFilePath)
	if err != nil {
		return err
	}

	return nil
}
