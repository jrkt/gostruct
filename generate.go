package gostruct

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"errors"
	"os"
)

type TableObj struct {
	Name       string
	IsNullable string
	Key        string
	DataType   string
	ColumnType string
}

type KeyObj struct {
	TableName        string
	ColumnName       string
	ReferencedTable  sql.NullString
	ReferencedColumn sql.NullString
}

type UsedColumn struct {
	Name string
}

var err error
var con *sql.DB
var tablesDone []string
var GOPATH string
var primaryKey string

func Run(table string, database string, host string, port string) error {
	GOPATH = os.Getenv("GOPATH")

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
	}

	return nil
}

type UniqueValues struct {
	Value sql.NullString
}

func buildCruxFile(objects []TableObj, table string, database string) error {

	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var usedColumns []UsedColumn
	initialString := `package ` + uppercaseFirst(table)
	importString := `

import (
	"database/sql"
	"strings"
	"date"
	"connection"
	"reflect"
	"strconv"
	"time"`

	string1 := "\n\ntype " + uppercaseFirst(table) + "Obj struct {"
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
			} else {
				dataType = "sql.NullString"
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
			primaryKeys = append(primaryKeys, object.Name)
			primaryKeyTypes = append(primaryKeyTypes, dataType)
		}

		if i > 0 {
			string2 += ", &object." + uppercaseFirst(object.Name)
		}
		string1 += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\"`"
	}
	string1 += "\n}"

	bs := `\"`

	if len(primaryKeys) > 0 {
		string1 += `

func (Object ` + uppercaseFirst(table) + `Obj) Save() error {
	v := reflect.ValueOf(&Object).Elem()
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
				convertedVal = `strconv.Itoa(Object.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(Object.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = `Object.` + uppercaseFirst(primaryKeys[0])
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
				convertedVal = `strconv.Itoa(Object.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(Object.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = `Object.` + uppercaseFirst(primaryKeys[0])
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

		whereStr, whereStr2 := "", ""
		for k := range primaryKeys {
			if k > 0 {
				whereStr += " AND"
				whereStr2 += ","
			}
			var convertedVal string
			switch primaryKeyTypes[k] {
			case "int":
				convertedVal = `strconv.Itoa(Object.` + uppercaseFirst(primaryKeys[k]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(Object.` + uppercaseFirst(primaryKeys[k]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = `Object.` + uppercaseFirst(primaryKeys[k])
			}

			if k == len(primaryKeys) - 1 {
				whereStr += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\""`
				whereStr2 += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\""`
			} else {
				whereStr += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\"`
				whereStr2 += ` ` + primaryKeys[k] + ` = \"" + ` + convertedVal + ` + "\"`
			}
		}

		if len(primaryKeys) == 1 {
			var convertedVal string
			switch primaryKeyTypes[0] {
			case "int":
				convertedVal = `strconv.Itoa(Object.` + uppercaseFirst(primaryKeys[0]) + `)`
			case "float64":
				convertedVal = `strconv.FormatFloat(Object.` + uppercaseFirst(primaryKeys[0]) + `, 'f', -1, 64)`
			case "string":
				convertedVal = `Object.` + uppercaseFirst(primaryKeys[0])
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
                value = "\"" + strings.Replace(value, ` + "`\"`, " + "`" + bs + "`, -1) + " + `"\""
	}

	return value
}`

		string1 += `

func (Object ` + uppercaseFirst(table) + `Obj) Delete() error {
	query := "DELETE FROM ` + table + ` WHERE` + whereStr + `

	con := connection.Get()
	_, err := con.Exec(query)
	if err != nil {
		return err
	}

	return nil
}
`
		paramStr, whereStr := "", ""
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
			}
			if k == len(primaryKeys) - 1 {
				whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'"`
			} else {
				paramStr += ", "
				whereStr += ` ` + param + ` = '" + ` + paramTypeStr + ` + "'`
			}
		}

		//create ReadById method
		string1 += `
func ReadById(` + paramStr + `) ` + uppercaseFirst(table) + `Obj {
	return ReadOneByQuery("SELECT * FROM ` + table + ` WHERE` + whereStr + `)
}`
	}

	string1 += `

func ReadAll(order string) []` + uppercaseFirst(table) + `Obj {
	return ReadByQuery("SELECT * FROM ` + table + `", order)
}`

	string1 += `

func ReadByQuery(query string, order string) []` + uppercaseFirst(table) + `Obj {
	connection := connection.Get()
	objects := []` + uppercaseFirst(table) + `Obj{}
	if order != "" {
		query += " ORDER BY " + order
	}
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := connection.Query(query)
	if err != nil {
		panic(err)
	} else {
		for rows.Next() {
			var object ` + uppercaseFirst(table) + `Obj
			rows.Scan(&object.` + uppercaseFirst(objects[0].Name) + string2 + `)
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

func ReadOneByQuery(query string) ` + uppercaseFirst(table) + `Obj {
	var object ` + uppercaseFirst(table) + `Obj

	con := connection.Get()
	query = strings.Replace(query, "'", "\"", -1)
	err := con.QueryRow(query).Scan(&object.` + uppercaseFirst(objects[0].Name) + string2 + `)

	switch {
	case err == sql.ErrNoRows:
	//do something?
	case err != nil:
		panic(err)
	}

	return object
}`

	importString += "\n)"
	contents = initialString + importString + string1

	cruxFilePath := dir + "CRUX_" + tableNaming + ".go"
	err = writeFile(cruxFilePath, contents, true)
	if err != nil {
		return err
	}

	_, err = runCommand("go fmt " + cruxFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

func buildDaoFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	daoFilePath := dir + "DAO_" + tableNaming + ".go"

	if !exists(daoFilePath) {
		contents := "package " + uppercaseFirst(table) + "\n\n//Methods Here"
		err = writeFile(daoFilePath, contents, true)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + daoFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

func buildBoFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"
	boFilePath := dir + "BO_" + tableNaming + ".go"

	if !exists(boFilePath) {
		contents := "package " + uppercaseFirst(table) + "\n\n//Methods Here"
		err = writeFile(boFilePath, contents, true)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + boFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

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
		err = writeFile(testFilePath, contents, true)
		if err != nil {
			return err
		}
	}

	_, err := runCommand("go fmt " + testFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

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
		panic(err)
	}
	return connection
}`
	err = writeFile(conFilePath, contents, true)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + conFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

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
	err = writeFile(dateFilePath, contents, true)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + dateFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}

func buildLoggerPackage() error {
	if !exists(GOPATH + "/src/logger") {
		err = CreateDirectory(GOPATH + "/src/logger")
		if err != nil {
			return err
		}
	}

	contents := `package logger

import (
	"errors"
)

type Exception struct {
	Msg string
	File    string
	Line    int
}

func (e *Exception) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

func HandleError(e *interface{}) {
	switch e.(type) {
	case error:
		//do something
	case *Exception:
		//do something
	default:
		//unknown error
	}
}`

	loggerFilePath := GOPATH + "/src/logger/logger.go"
	err = writeFile(loggerFilePath, contents, true)
	if err != nil {
		return err
	}

	_, err := runCommand("go fmt " + loggerFilePath, true, false)
	if err != nil {
		return err
	}

	return nil
}