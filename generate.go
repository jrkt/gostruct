//Package gostruct is an ORM that generates a golang package based on a MySQL database table including a DAO, BO, CRUX, test, and example file
//
//The CRUX file provides all basic CRUD functionality to handle any objects of the table. The test file is a skeleton file
//ready to use for unit testing. THe example file is populated with sample methods aiding in clear godocs.
package gostruct

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"errors"
	"os"
	"strings"
)

//TableObj is the result set returned from the MySQL information_schema that
//contains all data for a specific table
type TableObj struct {
	Name       string
	IsNullable string
	Key        string
	DataType   string
	ColumnType string
}

type UsedColumn struct {
	Name string
}

type UniqueValues struct {
	Value sql.NullString
}

//Globals variables
var err error
var con *sql.DB
var tables []string
var tablesDone []string
var primaryKey string
var GOPATH string
var exampleIdStr string
var exampleColumn string
var exampleColumnStr string
var exampleOrderStr string

//initialize global GOPATH
func init() {
	GOPATH = os.Getenv("GOPATH")
}

//Generates a package for a single table
func Run(table string, database string, host string, port string) error {
	// if empty set port to MySQL default port
	if port == "" {
		port = "3306"
	}

	//make sure models dir exists
	if !exists(GOPATH + "/src/models") {
		err = CreateDirectory(GOPATH + "/src/models")
		if err != nil {
			return err
		}
	}

	err = buildConnectionPackage(host, database)
	if err != nil {
		return err
	}

	err = buildDatePackage()
	if err != nil {
		return err
	}

	err = buildLoggerPackage()
	if err != nil {
		return err
	}

	err = handleTable(table, database, host, port)
	if err != nil {
		return err
	}

	return nil
}

//Generates packages for all tables in a specific database and host
func RunAll(database string, host string, port string) error {
	var portStr string
	if port == "" {
		portStr = "3306"
	} else {
		portStr = host
	}
	connection, err := sql.Open("mysql", DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":" + portStr + ")/" + database)
	if err != nil {
		panic(err)
	} else {
		rows, err := connection.Query("SELECT DISTINCT(TABLE_NAME) FROM `information_schema`.`COLUMNS` WHERE `TABLE_SCHEMA` LIKE ?", database)
		if err != nil {
			panic(err)
		} else {
			for rows.Next() {
				var table Table
				rows.Scan(&table.Name)

				err = Run(table.Name, database, host, port)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

//Creates directory and sets permissions to 0777
func CreateDirectory(path string) error {
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

//Main hanlder method for tables
func handleTable(table string, database string, host string, port string) error {
	if inArray(table, tablesDone) {
		return nil
	} else {
		tablesDone = append(tablesDone, table)
	}

	log.Println("Generating Models for: " + table)

	con, err = sql.Open("mysql", DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":" + port + ")/" + database)

	if err != nil {
		return err
	}

	rows1, err := con.Query("SELECT column_name, is_nullable, column_key, data_type, column_type FROM information_schema.columns WHERE table_name = ? AND table_schema = ?", table, database)

	var object TableObj
	var objects []TableObj = make([]TableObj, 0)
	var columns []string

	if err != nil {
		return err
	} else {
		cntPK := 0
		for rows1.Next() {
			rows1.Scan(&object.Name, &object.IsNullable, &object.Key, &object.DataType, &object.ColumnType)
			objects = append(objects, object)
			if object.Key == "PRI" {
				cntPK++
			}
			if cntPK > 1 && object.Key == "PRI" {
				continue
			}
			columns = append(columns, object.Name)
		}
	}
	defer rows1.Close()

	primaryKey = ""
	if len(objects) == 0 {
		return errors.New("No results for table: " + table)
	} else {
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
		err = buildCruxFile(objects, table, database)
		if err != nil {
			return err
		}

		//handle DAO file
		err = buildDaoFile(table)
		if err != nil {
			return err
		}

		//handle BO file
		err = buildBoFile(table)
		if err != nil {
			return err
		}

		//handle Test file
		err = buildTestFile(table)
		if err != nil {
			return err
		}

		//handle Example file
		err = buildExamplesFile(table)
		if err != nil {
			return err
		}
	}

	return nil
}

//Builds CRUX_{table}.go file with main struct and CRUD functionality
func buildCruxFile(objects []TableObj, table string, database string) error {
	exampleIdStr = ""
	exampleColumn = ""
	exampleColumnStr = ""
	exampleOrderStr = ""

	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var usedColumns []UsedColumn
	initialString := `//The ` + uppercaseFirst(table) + ` package serves as the base structure for the ` + table + ` table
//
//Package ` + uppercaseFirst(table) + ` contains base methods and CRUD functionality to
//interact with the ` + table + ` table in the ` + database + ` database
package ` + uppercaseFirst(table)
	importString := `

import (
	"database/sql"
	"strings"
	"date"
	"connection"
	"logger"
	"reflect"
	"strconv"
	"time"`

	string1 := `

//` + uppercaseFirst(table) + `Obj is the structure of the home table
//
//This contains all columns that exist in the database
type ` + uppercaseFirst(table) + "Obj struct {"
	string2 := ""
	contents := ""

	primaryKeys := []string{}
	primaryKeyTypes := []string{}

	Loop:
	for i := 0; i < len(objects); i++ {
		object := objects[i]
		for c := 0; c < len(usedColumns); c++ {
			if usedColumns[c].Name == object.Name {
				continue Loop
			}
		}
		usedColumns = append(usedColumns, UsedColumn{Name: object.Name})

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
			rows, err := con.Query("SELECT DISTINCT(`" + object.Name + "`) FROM " + database + "." + table)
			if err != nil {
				return err
			} else {
				for rows.Next() {
					var uObj UniqueValues
					rows.Scan(&uObj.Value)
					if uObj.Value.String != "0" && uObj.Value.String != "1" {
						isBool = false
					}
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
				dataType = "time.Time"
			} else {
				dataType = "date.NullTime"
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
		string1 += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\"`"
	}
	string1 += "\n}"

	bs := `\"`
	if len(primaryKeys) > 0 {
		string1 += `

//Save accepts a ` + uppercaseFirst(table) + `Obj pointer and
//applies any updates needed to the record in the database
func (` + strings.ToLower(table) + ` *` + uppercaseFirst(table) + `Obj) Save() (sql.Result, error) {
	v := reflect.ValueOf(` + strings.ToLower(table) + `).Elem()
	objType := v.Type()`

		string1 += `

	values := ""
	columns := ""`

		if len(primaryKeys) > 1 {
			string1 += `

	query := "INSERT INTO ` + table + `"`
		} else {
			var convertedVal string
			switch primaryKeyTypes[0] {
			case "int":
				convertedVal = `strconv.Itoa(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0])
			}

			string1 += `

	var query string

	if ` + convertedVal + ` == "" {
		query = "INSERT INTO ` + table + ` "
		firstValue := getFieldValue(v.Field(0))
		if firstValue != "null" {
			columns += string(objType.Field(0).Tag.Get("column"))
			values += firstValue
		}
	} else {
		query = "UPDATE ` + table + ` SET "
	}`
		}
		if len(primaryKeys) == 1 {
			string1 += `

	for i := 1; i < v.NumField(); i++ {`
		} else {
			string1 += `

	for i := 0; i < v.NumField(); i++ {`
		}
		string1 += `
		value := getFieldValue(v.Field(i))`

		if len(primaryKeys) == 1 {
			var convertedVal string
			switch primaryKeyTypes[0] {
			case "int":
				convertedVal = `strconv.Itoa(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0])
			}

			string1 += `

		if ` + convertedVal + ` == "" {
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
	}`
		} else {
			string1 += `

		if i > 0 {
			columns += ", "
			values += ", "
		}
		`
			string1 += "columns += \"`\" + string(objType.Field(i).Tag.Get(\"column\")) + \"`\""
			string1 += `
		values += value
	}`
		}

		whereStr, whereStr2, whereStrQuery, whereStrQueryValues := "", "", "", ""
		for k := range primaryKeys {
			if k > 0 {
				whereStr += " AND"
				whereStrQuery += " AND"
				whereStr2 += ","
				whereStrQueryValues += ","
			}
			var convertedVal string
			switch primaryKeyTypes[k] {
			case "int":
				convertedVal = `strconv.Itoa(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[k]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[k]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[k])
			}

			if k == len(primaryKeys) - 1 {
				whereStr += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\""`
				whereStr2 += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\""`
			} else {
				whereStr += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\"`
				whereStr2 += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\"`
			}
			whereStrQuery += ` ` + primaryKeys[k] + ` = ?`
			whereStrQueryValues += ` ` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[k])
		}

		if len(primaryKeys) == 1 {
			var convertedVal string
			switch primaryKeyTypes[0] {
			case "int":
				convertedVal = `strconv.Itoa(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(` + strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = strings.ToLower(table) + `.` + uppercaseFirst(primaryKeys[0])
			}

			string1 += `
	if ` + convertedVal + ` == "" {
		query += "(" + columns + ") VALUES (" + values + ")"
	} else {
		query += " WHERE` + whereStr + `
	}`
		} else {
			string1 += `
	query += " (" + columns + ") VALUES(" + values + ") ON DUPLICATE KEY UPDATE` + whereStr2
		}

		string1 += `

	con := connection.Get()
	result, err := con.Exec(query)
	if err != nil {
		logger.HandleError(err)
	}

	return result, err
}

//Serves as a global 'toString()' function getting each property's string
//representation so we can include it in the database query
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
                value = "\"" + strings.Replace(value, ` + "`\"`, " + "`" + bs + "`, -1) + " + `"\""
	}

	return value
}`

		string1 += `

//Deletes record from database
func (` + strings.ToLower(table) + ` *` + uppercaseFirst(table) + `Obj) Delete() (sql.Result, error) {
	con := connection.Get()
	result, err := con.Exec("DELETE FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrQueryValues + `)
	if err != nil {
		logger.HandleError(err)
	}

	return result, err
}
`
		paramStr, whereStr, whereStrValues := "", "", ""
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
			var paramTypeStr string
			switch primaryKeyTypes[k] {
			case "int":
				dataType = "int"
				paramTypeStr = "strconv.Itoa(" + param + ")"
			case "float64":
				dataType = "float64"
				paramTypeStr = "strconv.FormatFloat(" + param + ", 'f', -1, 64)"
			default:
				dataType = "string"
				paramTypeStr = param
			}

			paramStr += param + " " + dataType
			if k > 0 {
				whereStr += " AND"
				whereStrValues += ","
			}
			if k == len(primaryKeys) - 1 {
				whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'"`
			} else {
				paramStr += ", "
				whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'`
			}
			whereStrValues += " " + param
		}

		//create ReadById method
		string1 += `
//Returns a single object as pointer
func ReadById(` + paramStr + `) (*` + uppercaseFirst(table) + `Obj, error) {
	return ReadOneByQuery("SELECT * FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrValues + `)
}`
	}

	string1 += `

//Returns all records in the table as a slice of ` + uppercaseFirst(table) + `Obj pointers
func ReadAll(order string) ([]*` + uppercaseFirst(table) + `Obj, error) {
	query := "SELECT * FROM ` + table + `"
	if order != "" {
		query += " ORDER BY " + order
	}
	return ReadByQuery(query)
}`

	string1 += `

//Returns a slice of ` + uppercaseFirst(table) + `Obj pointers
//
//Accepts a query string, and an order string
func ReadByQuery(query string, args ...interface{}) ([]*` + uppercaseFirst(table) + `Obj, error) {
	con := connection.Get()
	objects := make([]*` + uppercaseFirst(table) + `Obj, 0)
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.Query(query, args...)
	if err != nil {
		logger.HandleError(err)
		return objects, err
	} else {
		for rows.Next() {
			var ` + strings.ToLower(table) + ` ` + uppercaseFirst(table) + `Obj
			rows.Scan(&` + strings.ToLower(table) + `.` + uppercaseFirst(objects[0].Name) + string2 + `)
			objects = append(objects, &` + strings.ToLower(table) + `)
		}
		err = rows.Err()
		if err != nil {
			logger.HandleError(err)
			return objects, err
		} else if len(objects) == 0 {
			return objects, sql.ErrNoRows
		}
		rows.Close()
	}

	return objects, nil
}

//Returns a single object as pointer
//
//Serves as the LIMIT 1
func ReadOneByQuery(query string, args ...interface{}) (*` + uppercaseFirst(table) + `Obj, error) {
	var ` + strings.ToLower(table) + ` ` + uppercaseFirst(table) + `Obj
	con := connection.Get()
	query = strings.Replace(query, "'", "\"", -1)
	err := con.QueryRow(query, args...).Scan(&` + strings.ToLower(table) + `.` + uppercaseFirst(objects[0].Name) + string2 + `)
	if err != nil {
		logger.HandleError(err)
	}

	return &` + strings.ToLower(table) + `, err
}

//Method for executing UPDATE queries
func Exec(query string, args ...interface{}) (sql.Result, error) {
	con := connection.Get()
	result, err := con.Exec(query, args...)
	if err != nil {
		logger.HandleError(err)
	}

	return result, err
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
func buildDaoFile(table string) error {
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
func buildBoFile(table string) error {
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
func buildTestFile(table string) error {
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
func buildExamplesFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	examplesFilePath := dir + "examples_test.go"

	if !exists(examplesFilePath) {
		contents := `package ` + tableNaming + `_test

import (
	"fmt"
	"models/` + tableNaming + `"
)

func Example` + tableNaming + `Obj_Save() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	if err == nil {
		` + strings.ToLower(table) + `.` + exampleColumnStr + `
		` + strings.ToLower(table) + `.Save()
	}
}

func Example` + tableNaming + `Obj_Delete() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	if err == nil {
		` + strings.ToLower(table) + `.Delete()
	}
}

func ExampleReadAll() {
	` + strings.ToLower(table) + `s, err := ` + tableNaming + `.ReadAll("` + exampleOrderStr + `")
	if err == nil {
		for i := range ` + strings.ToLower(table) + `s {
			` + strings.ToLower(table) + ` := ` + strings.ToLower(table) + `s[i]
			fmt.Println(` + strings.ToLower(table) + `)
		}
	}
}

func ExampleReadById() {
	` + strings.ToLower(table) + `, _ := ` + tableNaming + `.ReadById(` + exampleIdStr + `)
	fmt.Println(` + strings.ToLower(table) + `)
}

func ExampleReadByQuery() {
	` + strings.ToLower(table) + `s, err := ` + tableNaming + `.ReadByQuery("SELECT * FROM ` + table + ` WHERE ` + exampleColumn + ` = 'some string'", "` + exampleOrderStr + `")
	if err == nil {
		for i := range ` + strings.ToLower(table) + `s {
			` + strings.ToLower(table) + ` := ` + strings.ToLower(table) + `s[i]
			fmt.Println(` + strings.ToLower(table) + `)
		}
	}
}

func ExampleReadOneByQuery() {
	` + strings.ToLower(table) + `, _ := ` + tableNaming + `.ReadOneByQuery("SELECT * FROM ` + table + ` WHERE ` + exampleColumn + ` = 'some string' ORDER BY ` + exampleOrderStr + `")
	fmt.Println(` + strings.ToLower(table) + `)
}

func ExampleExec() {
	_, err := ` + tableNaming + `.Exec(fmt.Sprintf("UPDATE ` + table + ` SET ` + exampleColumn + ` = 'some string' WHERE id = '%d'", ` + exampleIdStr + `))
	if err != nil {
		//Exec failed
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

//Builds main connection package for serving up all database connections
//with a shared connection pool
func buildConnectionPackage(host string, database string) error {
	if !exists(GOPATH + "/src/connection") {
		err = CreateDirectory(GOPATH + "/src/connection")
		if err != nil {
			return err
		}
	}

	conFilePath := GOPATH + "/src/connection/connection.go"
	contents := `package connection
import (
	"logger"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

var connection *sql.DB
var err error

func Get() *sql.DB {
	if connection != nil {
		//determine whether connection is still alive
		err = connection.Ping()
		if err == nil {
			return connection
		}
	}

	connection, err = sql.Open("mysql", "` + DB_USERNAME + `:` + DB_PASSWORD + `@tcp(` + host + `:3306)/` + database + `?parseTime=true")
	if err != nil {
		logger.HandleError(err)
	}

	return connection
}`
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

//Builds date package to provide date.NullTime type
func buildDatePackage() error {
	if !exists(GOPATH + "/src/date") {
		err = CreateDirectory(GOPATH + "/src/date")
		if err != nil {
			return err
		}
	}

	dateFilePath := GOPATH + "/src/date/date.go"
	contents := `package date
import (
	"time"
	"database/sql/driver"
)

const DEFAULT_FORMAT = "2006-01-02 15:04:05"

// In place of a sql.NullTime struct
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}`
	err = writeFile(dateFilePath, contents, false)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + dateFilePath)
	if err != nil {
		return err
	}

	return nil
}

//Builds base logging package that
func buildLoggerPackage() error {
	if !exists(GOPATH + "/src/logger") {
		err = CreateDirectory(GOPATH + "/src/logger")
		if err != nil {
			return err
		}
	}

	contents := `package logger

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
)

var datetime string
var hostname string
var ip string
var line int
var pc uintptr
var class string
var method string
var file string

//Method to set all variables used in all functions of logging
func setVars() {
	t := time.Now()
	datetime = t.Format("2006-01-02 15:04:05")
	hostname, _ = os.Hostname()
	ipArr, _ := net.LookupHost(hostname)
	if len(ipArr) == 1 {
		ip = ipArr[0]
	}
	pc, file, line, _ = runtime.Caller(3)
	path := runtime.FuncForPC(pc).Name()
	pathArgs := strings.Split(path, ".")
	class = pathArgs[0]
	method = pathArgs[1]
}

func HandleError(e interface{}) {
	setVars()

	if e == sql.ErrNoRows {
		//handle queries with no results
	} else {
		errorStr := fmt.Sprintf("%s %s(%s.%s):%d - %s", datetime, file, class, method, line, e.Error())
		log.Fatalln(errorStr)
	}
}`

	loggerFilePath := GOPATH + "/src/logger/logger.go"
	err = writeFile(loggerFilePath, contents, false)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + loggerFilePath)
	if err != nil {
		return err
	}

	return nil
}