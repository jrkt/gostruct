/*
Package gostruct is an ORM that builds a package for a specific MySQL database table.

A package with the underlying struct of the table will be created in the $GOPATH/src/models/{table} directory along with several methods to handle common requests. The files that are created in the package, for a 'User' model (for example) would be:

User_base.go - CRUD operations and common ReadBy functions. It also validates any enum/set data type with the value passed to ensure it is one of the required fields

User_extended.go - Custom functions & methods

User_test.go - Serves as a base for your unit testing

examples_test.go - Includes auto-generated example methods based on the auto-generated methods in the CRUX file

It will also generate a connection package to share connection(s) to prevent multiple open database connections.

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

	//imported to allow mysql driver to be used
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

	tableNaming := uppercaseFirst(table)

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
	dir := GOPATH + "/src/models/" + tableNaming + "/"
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
	err = gs.buildBase(objects, table)
	if err != nil {
		return err
	}

	//handle DAO file
	err = gs.buildExtended(table)
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

//Builds {table}_base.go file with main struct and CRUD functionality
func (gs *Gostruct) buildBase(objects []TableObj, table string) error {
	exampleIdStr = ""
	exampleColumn = ""
	exampleColumnStr = ""
	exampleOrderStr = ""

	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + tableNaming + "/"

	var usedColumns []usedColumn
	var funcName string
	if gs.NameFuncs {
		funcName = tableNaming
	}
	initialString := `//The ` + tableNaming + ` package serves as the base structure for the ` + table + ` table
//
//Package ` + tableNaming + ` contains base methods and CRUD functionality to
//interact with the ` + table + ` table in the ` + gs.Database + ` database

package ` + tableNaming
	importString := `

import (
	"database/sql"
	"strings"
	"errors"
	"fmt"
	"connection"
	"reflect"
	"utils"`

	nilStruct := `
	//` + strings.ToLower(table) + ` is the nilable structure of the home table
type ` + strings.ToLower(table) + " struct {"

	string1 := `

//` + tableNaming + ` is the structure of the home table
//
//This contains all columns that exist in the database
type ` + tableNaming + " struct {"
	string2, nilString2, scanStr2, nilExtension := "", "", "", ""
	contents := ""

	var primaryKeys []string
	var primaryKeyTypes []string
	var questionMarks []string
	var nullableDeclarations, nullableHandlers string

	importTime, importMysql := false, false
	nullableCnt := 0

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
		var dataType, nilDataType string
		switch object.DataType {
		case "int", "mediumint":
			isBool = false
			if object.IsNullable == "YES" {
				dataType = "*int64"
				nullableCnt++
				nilDataType = "sql.NullInt64"
				nilExtension = ".Int64"
			} else {
				dataType = "int64"
				nilDataType = "int64"
			}
		case "tinyint", "smallint":
			rows, err := con.Query("SELECT DISTINCT(`" + object.Name + "`) FROM " + gs.Database + "." + table)
			if err != nil {
				return err
			}
			for rows.Next() {
				var uObj uniqueValues
				rows.Scan(&uObj.Value)
				if uObj.Value.String != "0" && uObj.Value.String != "1" && uObj.Value.String != "" {
					isBool = false
				}
			}
			if isBool {
				if object.IsNullable == "YES" {
					dataType = "*bool"
					nullableCnt++
					nilDataType = "sql.NullBool"
					nilExtension = ".Bool"
				} else {
					dataType = "bool"
					nilDataType = "bool"
				}
			} else {
				if object.IsNullable == "YES" {
					dataType = "*int64"
					nullableCnt++
					nilDataType = "sql.NullInt64"
					nilExtension = ".Int64"
				} else {
					dataType = "int64"
					nilDataType = "int64"
				}
			}
		case "float", "decimal":
			isBool = false
			if object.IsNullable == "YES" {
				dataType = "*float64"
				nullableCnt++
				nilDataType = "sql.NullFloat64"
				nilExtension = ".Float64"
			} else {
				dataType = "float64"
				nilDataType = "float64"
			}
		case "date", "datetime", "timestamp":
			isBool = false
			importTime = true
			if object.IsNullable == "YES" {
				dataType = "*time.Time"
				importMysql = true
				nullableCnt++
				nilDataType = "mysql.NullTime"
				nilExtension = ".Time"
			} else {
				dataType = "time.Time"
				nilDataType = "time.Time"
			}
		default:
			isBool = false
			if object.IsNullable == "YES" {
				dataType = "*string"
				nullableCnt++
				nilDataType = "sql.NullString"
				nilExtension = ".String"
			} else {
				dataType = "string"
				nilDataType = "string"
			}
		}

		if object.Key == "PRI" {
			if isBool {
				dataType = "int64"
			}
			primaryKeys = append(primaryKeys, object.Name)
			primaryKeyTypes = append(primaryKeyTypes, dataType)
		}

		if i > 0 {
			if object.IsNullable == "YES" {
				name := strings.ToLower(object.Name)
				if name == "type" {
					name = "objType"
				}
				nullableDeclarations += `
					var ` + name + " " + dataType
				nullableHandlers += `
				if ` + name + ` != nil {
					` + "obj." + uppercaseFirst(object.Name) + ` = ` + name + `
				}`
				nilString2 += ", &" + name
				scanStr2 += ", &obj." + uppercaseFirst(object.Name) + nilExtension
			} else {
				nilString2 += ", &obj." + uppercaseFirst(object.Name)
				scanStr2 += ", obj." + uppercaseFirst(object.Name)
			}
			string2 += ", &obj." + uppercaseFirst(object.Name)
		}

		defaultVal := ""
		if strings.ToLower(object.Default.String) != "null" {
			if object.Default.String == "0" && object.IsNullable == "YES" {
				defaultVal = ""
			} else {
				defaultVal = object.Default.String
			}
		}
		string1 += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\" default:\"" + defaultVal + "\" type:\"" + object.ColumnType + "\" key:\"" + object.Key + "\" null:\"" + object.IsNullable + "\" extra:\"" + object.Extra.String + "\"`"
		nilStruct += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + nilDataType
	}
	string1 += "\n}"
	nilStruct += "\n}\n"

	optionThreshold := 3

	if nullableCnt > optionThreshold {
		string1 += nilStruct
	}

	if importTime {
		importString += `
		"time"`
	}
	if importMysql && nullableCnt > optionThreshold {
		importString += `
		"github.com/go-sql-driver/mysql"`
	}

	var scanStr string
	if nullableCnt <= optionThreshold {
		scanStr = nilString2
	} else {
		scanStr = string2
	}

	bs := "`"

	if len(primaryKeys) > 0 {
		string1 += `

//Save runs an INSERT..UPDATE ON DUPLICATE KEY and validates each value being saved
func (obj *` + tableNaming + `) ` + funcName + `Save() (sql.Result, error) {
	v := reflect.ValueOf(obj).Elem()
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

	return ` + funcName + `Exec(query, newArgs...)`
		} else {
			var insertIdStr string
			switch primaryKeyTypes[0] {
			case "string":
				insertIdStr = "strconv.FormatInt(id, 10)"
				importString += `
				"strconv"`
			default:
				insertIdStr = `id`
			}

			string1 += `
			newRecord := false
			if utils.Empty(obj.` + uppercaseFirst(primaryKeys[0]) + `) {
				newRecord = true
			}

			res, err := ` + funcName + `Exec(query, newArgs...)
			if err == nil && newRecord {
				id, _ := res.LastInsertId()
				obj.` + uppercaseFirst(primaryKeys[0]) + ` = ` + insertIdStr + `
			}
			return res, err`
		}

		whereStrQuery, whereStrQueryValues := "", ""
		for k := range primaryKeys {
			if k > 0 {
				whereStrQuery += " AND"
				whereStrQueryValues += ","
			}
			whereStrQuery += ` ` + primaryKeys[k] + ` = ?`
			whereStrQueryValues += ` obj.` + uppercaseFirst(primaryKeys[k])
		}

		string1 += `
}

//Delete removes a record from the database according to the primary key
func (obj *` + tableNaming + `) ` + funcName + `Delete() (sql.Result, error) {
	return ` + funcName + `Exec("DELETE FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrQueryValues + `)
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
			switch primaryKeyTypes[k] {
			case "int64":
				dataType = "int64"
			case "float64":
				dataType = "float64"
			default:
				dataType = "string"
			}

			paramStr += param + " " + dataType
			if k > 0 {
				whereStrValues += ","
			}
			if k != len(primaryKeys)-1 {
				paramStr += ", "
			}
			whereStrValues += " " + param
		}

		//create ReadByKey method
		string1 += `
//ReadByKey returns a single pointer to a(n) ` + tableNaming + `
func Read` + funcName + `ByKey(` + paramStr + `) (*` + tableNaming + `, error) {
	return ReadOne` + funcName + `ByQuery("SELECT * FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrValues + `)
}`
	}

	string1 += `

//ReadAll returns all records in the table
func ReadAll` + funcName + `(options ...connection.QueryOptions) ([]*` + tableNaming + `, error) {
	return Read` + funcName + `ByQuery("SELECT * FROM ` + table + `", options)
}`

	string1 += `

//ReadByQuery returns an array of ` + tableNaming + ` pointers
func Read` + funcName + `ByQuery(query string, args ...interface{}) ([]*` + tableNaming + `, error) {
	var objects []*` + tableNaming + `
	var err error
	var argss []interface{}
	for _, arg := range args {
		switch t := arg.(type) {
		case []connection.QueryOptions:
			if len(t) > 0 {
				options := t[0]
				orderBy := options.OrderBy
				if orderBy != "" {
					query += fmt.Sprintf(" ORDER BY %s", orderBy)
				}
				limit := options.Limit
				if limit != 0 {
					query += fmt.Sprintf(" LIMIT %d", limit)
				}
			}
		default:
			argss = append(argss, t)
		}
	}

	con, err := connection.Get("` + gs.Database + `")
	if err != nil {
		return objects, errors.New("connection failed")
	}
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.Query(query, argss...)
	if err != nil {
		return objects, err
	} else {
		rowsErr := rows.Err()
		if rowsErr != nil {
			return objects, err
		}

		defer rows.Close()
		for rows.Next() {`
	if nullableCnt <= optionThreshold {
		string1 += "var obj " + tableNaming
		string1 += nullableDeclarations
	} else {
		string1 += "var obj " + strings.ToLower(table)
	}
	string1 += `
			err = rows.Scan(&obj.` + uppercaseFirst(objects[0].Name) + scanStr + `)
			if err != nil {
				return objects, err
			}`
	if nullableCnt <= optionThreshold {
		string1 += nullableHandlers
		string1 += `
		objects = append(objects, &obj)`
	} else {
		string1 += `
		objects = append(objects, &` + tableNaming + `{obj.` + uppercaseFirst(objects[0].Name) + scanStr2 + `})`
	}

	string1 += `
		}
	}

	if len(objects) == 0 {
		err = sql.ErrNoRows
	}

	return objects, err
}

//ReadOneByQuery returns a single pointer to a(n) ` + tableNaming + `
func ReadOne` + funcName + `ByQuery(query string, args ...interface{}) (*` + tableNaming + `, error) {`
	if nullableCnt <= optionThreshold {
		string1 += "var obj " + tableNaming
	} else {
		string1 += "var obj " + strings.ToLower(table)
	}

	string1 += `

	con, err := connection.Get("` + gs.Database + `")
	if err != nil {
		return &` + tableNaming + `{}, errors.New("connection failed")
	}
	query = strings.Replace(query, "'", "\"", -1)`
	if nullableCnt <= optionThreshold {
		string1 += nullableDeclarations
	}
	string1 += `
	err = con.QueryRow(query, args...).Scan(&obj.` + uppercaseFirst(objects[0].Name) + scanStr + `)
	if err != nil && err != sql.ErrNoRows {
		return &` + tableNaming + `{}, err
	}`
	if nullableCnt <= optionThreshold {
		string1 += nullableHandlers
		string1 += `

		return &obj, err`
	} else {
		string1 += `

		return &` + tableNaming + `{obj.` + uppercaseFirst(objects[0].Name) + scanStr2 + `}, nil
		`
	}
	string1 += `
}

//Exec allows for update queries
func ` + funcName + `Exec(query string, args ...interface{}) (sql.Result, error) {
	con, err := connection.Get("` + gs.Database + `")
	if err != nil {
		var result sql.Result
		return result, errors.New("connection failed")
	}
	return con.Exec(query, args...)
}`

	importString += "\n)"
	contents = initialString + importString + string1

	autoGenFile := dir + tableNaming + "_base.go"
	err = writeFile(autoGenFile, contents, true)
	if err != nil {
		return err
	}

	_, err = runCommand("go fmt " + autoGenFile)
	if err != nil {
		return err
	}

	return nil
}

//Builds {table}_extends.go file for custom Data Access Object methods
func (gs *Gostruct) buildExtended(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + tableNaming + "/"
	daoFilePath := dir + tableNaming + "_extended.go"

	if !exists(daoFilePath) {
		contents := "package " + tableNaming + "\n\n//Methods Here"
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

//Builds {table}_test.go file
func (gs *Gostruct) buildTestFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + tableNaming + "/"
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
	dir := GOPATH + "/src/models/" + tableNaming + "/"
	examplesFilePath := dir + "examples_test.go"

	if !exists(examplesFilePath) {
		contents := `package ` + tableNaming + `_test

import (
	"fmt"
	"models/` + tableNaming + `"
	"database/sql"
)

func Example` + tableNaming + `_Save() {
	//existing ` + strings.ToLower(table) + `
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadByKey(` + exampleIdStr + `)
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
	` + strings.ToLower(table) + ` = new(` + tableNaming + `.` + tableNaming + `)
	res, err := ` + strings.ToLower(table) + `.Save()
	if err != nil {
		//save failed
	} else {
		lastInsertId, err := res.LastInsertId()
		numRowsAffected, err := res.RowsAffected()
	}
}

func Example` + tableNaming + `_Delete() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadByKey(` + exampleIdStr + `)
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

func ExampleReadByKey() {
	` + strings.ToLower(table) + `, err := ` + tableNaming + `.ReadByKey(` + exampleIdStr + `)
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
		if v.IsZero() {
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
	}

	return val
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
	"log"
	"sync"

	_ "github.com/go-sql-driver/mysql"
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

//QueryOptions allows for passing optional parameters for queries
type QueryOptions struct {
	OrderBy string
	Limit   int
}

//Get returns a connection to a specific database. If the connection exists in the connections list AND is
//still active, it will just return that connection. Otherwise, it will open a new connection to
//the specified database and add it to the connections list.
func Get(db string) (*sql.DB, error) {

	connection := connections.list[db]
	if connection != nil {
		//determine if connection is still active
		err = connection.Ping()
		if err == nil {
			return connection, err
		}
	}

	con, err := sql.Open("mysql", fmt.Sprintf("root:Jstevens120)@tcp(localhost:3306)/%s?parseTime=true", db))
	if err != nil {
		//do whatever tickles your fancy here
		log.Fatalln("Connection Error to DB [", db, "]", err.Error())
	}
	con.SetMaxIdleConns(10)
	con.SetMaxOpenConns(500)

	connections.Lock()
	connections.list[db] = con
	connections.Unlock()

	return con, nil
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
