/*
Package gostruct is an ORM that builds a package for a specific MySQL database table.

A package with the underlying struct of the table will be created in the $GOPATH/src/models/{table} directory along with several methods to handle common requests. The files that are created in the package, for a 'User' model (for example) would be:

User_base.go - CRUD operations and common ReadBy functions. It also validates any enum/set data type with the value passed to ensure it is one of the required fields

User_extended.go - Custom functions & methods

User_test.go - Serves as a base for your unit testing

examples_test.go - Includes auto-generated example methods based on the auto-generated methods in the CRUX file

It will also generate a connection package to share connection(s) to prevent multiple open database connections.

Dependencies:

        go get github.com/go-sql-driver/mysql
        go get github.com/pkg/errors

Installation:

        go get github.com/jonathankentstevens/gostruct

Create a generate.go file with the following contents (including your db username/password):

	package main

	import (
		"github.com/jonathankentstevens/gostruct"
	)

	func main() {
		gs := new(gostruct.Gostruct)
		gs.Username = "{username}"
		gs.Password = "{password}"
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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	// imported to allow mysql driver to be used
	_ "github.com/go-sql-driver/mysql"
)

// Gostruct is the main holding object for connection information
type Gostruct struct {
	Database  string
	Host      string
	Port      string
	Username  string
	Password  string
	NameFuncs bool
	add       chan int
	totalChan chan int
	errorChan chan error
	processed int
	errored   int
	errors    []error
	total     int
}

// tableObj is the result set returned from the MySQL information_schema that
// contains all data for a specific table
type tableObj struct {
	Name       string
	IsNullable string
	Key        string
	DataType   string
	ColumnType string
	Default    sql.NullString
	Extra      sql.NullString
}

type table struct {
	Name string
}

type usedColumn struct {
	Name string
}

type uniqueValues struct {
	Value sql.NullString
}

// Globals variables
var (
	GOPATH string
	wg     sync.WaitGroup
)

// initialize global GOPATH
func init() {
	GOPATH = os.Getenv("GOPATH")
	if last := len(GOPATH) - 1; last >= 0 && GOPATH[last] == '/' {
		GOPATH = GOPATH[:last]
	}

	err := buildConnectionPkg()
	if err != nil {
		panic(err)
	}

	// handle extract file
	err = buildExtractPkg()
	if err != nil {
		panic(err)
	}
}

// Generate serves as the main method to build package
func (g *Gostruct) Generate() error {

	tbls := flag.String("tables", "", "Comma separated list of tables")
	db := flag.String("db", "", "Database")
	host := flag.String("host", "", "DB Host")
	port := flag.String("port", "3306", "DB Port (MySQL 3306 is default)")
	all := flag.Bool("all", false, "Run for All Tables")
	nameFuncs := flag.Bool("nameFuncs", false, "Whether to include the struct name in the function signature")
	flag.Parse()

	g.Database = *db
	g.Host = *host
	g.NameFuncs = *nameFuncs
	g.Port = *port

	g.add = make(chan int, 1)
	g.errorChan = make(chan error, 1)
	g.totalChan = make(chan int, 1)
	work := make(chan string, 1)

	go g.handler()

	for i := 0; i < 50; i++ {
		go g.worker(work)
	}

	stop := startTimer(g)
	defer stop()

	if *all {
		err := g.RunAll(work)
		if err != nil {
			return err
		}
	} else {
		if (*tbls == "" && !*all) || *db == "" || *host == "" {
			return errors.New("You must include the 'table', 'database', and 'host' flag")
		}
		t := strings.Replace(*tbls, " ", "", -1)
		tables := strings.Split(t, ",")
		g.total = len(tables)
		for _, tbl := range tables {
			wg.Add(1)
			work <- tbl
		}
	}
	time.Sleep(1 * time.Second)
	log.Println("Waiting for goroutines to finish work...")
	wg.Wait()

	return nil
}

// handler provides a safe way to perform all concurrent tasks
func (g *Gostruct) handler() {
	for {
		select {
		case cnt := <-g.add:
			g.processed += cnt
			wg.Done()
			showProgress(*g)
		case cnt := <-g.totalChan:
			g.total += cnt
		case err := <-g.errorChan:
			g.errored++
			g.errors = append(g.errors, err)
			println("ERROR:", err.Error())
		}
	}
}

// worker loops through & runs all jobs in the work queue
func (g Gostruct) worker(work <-chan string) {
	for table := range work {
		g.Run(table)
	}
}

// RunAll generates packages for all tables in a specific database and host
func (g *Gostruct) RunAll(work chan<- string) error {
	connection, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", g.Username, g.Password, g.Host, g.Port, g.Database))
	if err != nil {
		return err
	}

	type Count struct {
		cnt int
	}
	var count Count
	err = connection.QueryRow("SELECT COUNT(DISTINCT(TABLE_NAME)) FROM `information_schema`.`COLUMNS` WHERE `TABLE_SCHEMA` LIKE ?", g.Database).Scan(&count.cnt)
	if err != nil {
		return err
	}
	g.totalChan <- count.cnt

	rows, err := connection.Query("SELECT DISTINCT(TABLE_NAME) FROM `information_schema`.`COLUMNS` WHERE `TABLE_SCHEMA` LIKE ?", g.Database)
	if err != nil {
		return err
	}

	for rows.Next() {
		wg.Add(1)
		var tbl table
		rows.Scan(&tbl.Name)
		work <- tbl.Name
	}

	return nil
}

// Run handles the run for a single table
func (g Gostruct) Run(table string) {
	// make sure models dir exists
	if !exists(GOPATH + "/src/models") {
		err := createDirectory(GOPATH + "/src/models")
		if err != nil {
			g.errorChan <- err
			return
		}
	}

	log.Println("Building package:", table)

	con, err := getConnection(g)
	if err != nil {
		g.errorChan <- err
		return
	}

	rows1, err := con.Query("SELECT column_name, is_nullable, column_key, data_type, column_type, column_default, extra FROM information_schema.columns WHERE table_name = ? AND table_schema = ?", table, g.Database)
	if err != nil {
		g.errorChan <- err
		return
	}

	var object tableObj
	var objects []tableObj
	var columns []string

	tableNaming := uppercaseFirst(table)

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

	if len(objects) == 0 {
		g.errorChan <- errors.New("No results for table: " + table)
		return
	}

	// create directory if needed
	dir := GOPATH + "/src/models/" + tableNaming + "/"
	if !exists(dir) {
		err := os.Mkdir(dir, 0777)
		if err != nil {
			g.errorChan <- err
			return
		}

		// give new directory full permissions
		err = os.Chmod(dir, 0777)
		if err != nil {
			g.errorChan <- err
			return
		}
	}

	// handle base file
	err = g.buildBase(objects, table)
	if err != nil {
		g.errorChan <- err
		return
	}

	// handle extended file
	g.buildExtended(table)

	// handle Test file
	g.buildTest(table)

	g.add <- 1
}

// buildBase builds the {table}_base.go file with main struct and CRUD functionality
func (g Gostruct) buildBase(objects []tableObj, table string) error {
	tableNaming := uppercaseFirst(table)
	lowerTable := strings.ToLower(table)

	dir := GOPATH + "/src/models/" + tableNaming + "/"
	importTime, importMysql := false, false

	var usedColumns []usedColumn
	var scanStr, scanStr2, nilExtension, funcName string
	var primaryKeys, primaryKeyTypes, questionMarks []string

	if g.NameFuncs {
		funcName = tableNaming
	}
	initialString := `// Package ` + tableNaming + ` contains base methods and CRUD functionality to
// interact with the ` + table + ` table in the ` + g.Database + ` database
package ` + tableNaming
	importString := `

import (
	"connection"
	"database/sql"
	"reflect"
	"strings"`

	nilStruct := `
	// ` + lowerTable + ` is the nilable structure of the home table
type ` + lowerTable + " struct {"

	string1 := `

// ` + tableNaming + ` is the structure of the home table
type ` + tableNaming + " struct {"

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
				nilDataType = "sql.NullInt64"
				nilExtension = ".Int64"
			} else {
				dataType = "int64"
				nilDataType = "int64"
			}
		case "tinyint", "smallint":
			con, err := getConnection(g)
			if err != nil {
				return err
			}
			rows, err := con.Query("SELECT DISTINCT(`" + object.Name + "`) FROM " + g.Database + "." + table)
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
					nilDataType = "sql.NullBool"
					nilExtension = ".Bool"
				} else {
					dataType = "bool"
					nilDataType = "bool"
				}
			} else {
				if object.IsNullable == "YES" {
					dataType = "*int64"
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
				scanStr2 += ", &obj." + uppercaseFirst(object.Name) + nilExtension
			} else {
				scanStr2 += ", obj." + uppercaseFirst(object.Name)
			}
			scanStr += ", &obj." + uppercaseFirst(object.Name)
		}

		var defaultVal string
		if strings.ToLower(object.Default.String) != "null" {
			defaultVal = object.Default.String
		}
		string1 += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\" default:\"" + defaultVal + "\" type:\"" + object.ColumnType + "\" key:\"" + object.Key + "\" null:\"" + object.IsNullable + "\" extra:\"" + object.Extra.String + "\"`"
		nilStruct += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + nilDataType
	}
	string1 += "\n}" + nilStruct + "\n}\n"

	if importTime {
		importString += `
		"time"`
	}
	importString += `
	`
	if importMysql {
		importString += `
		"github.com/go-sql-driver/mysql"`
	} else {
		importString += `
		_ "github.com/go-sql-driver/mysql"`
	}
	importString += `
		"github.com/pkg/errors"
		"golang.org/x/net/context"`

	if len(primaryKeys) == 1 {
		string1 += `

// TableName returns the name of the mysql table
func (obj *` + tableNaming + `) TableName() string {
	return "` + table + `"
}

// PrimaryKeyInfo returns the string value of the primary key column and the corresponding value for the receiver
func (obj *` + tableNaming + `) PrimaryKeyInfo() (string, interface{}) {
	val := reflect.ValueOf(obj).Elem()
	var objTypeId interface{}
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		column := val.Type().Field(i).Tag.Get("column")
		if "` + primaryKeys[0] + `" == column {
			switch valueField.Kind() {
			case reflect.Int:
				objTypeId = valueField.Interface().(int)
			case reflect.Int64:
				objTypeId = valueField.Interface().(int64)
			case reflect.String:
				objTypeId = valueField.Interface().(string)
			}
		}
	}

	return "` + primaryKeys[0] + `", objTypeId
}

// TypeInfo implements mysql.Info interface to allow for retrieving type/typeId for any db model
func (obj *` + tableNaming + `) TypeInfo() (string, interface{}) {
	_, pkVal := obj.PrimaryKeyInfo()
	return "` + table + `", pkVal
}`
	}

	if len(primaryKeys) > 0 {
		string1 += `

// Save runs an INSERT..UPDATE ON DUPLICATE KEY and validates each value being saved
func (obj *` + tableNaming + `) ` + funcName + `Save(ctx context.Context) (sql.Result, error) {
	v := reflect.ValueOf(obj).Elem()
	args, columns, q, updateStr, err := connection.BuildQuery(v, v.Type())
	if err != nil {
		return nil, errors.Wrap(err, "field validation error")
	}
	query := "INSERT INTO ` + table + ` (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(q, ", ") + ") ON DUPLICATE KEY UPDATE " + updateStr
	newArgs := append(args, args...)`

		if len(primaryKeys) > 1 {
			string1 += `

	return ` + funcName + `Exec(ctx, query, newArgs...)`
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
			if obj.` + uppercaseFirst(primaryKeys[0]) + ` == 0 {
				newRecord = true
			}

			res, err := ` + funcName + `Exec(ctx, query, newArgs...)
			if err == nil && newRecord {
				id, _ := res.LastInsertId()
				obj.` + uppercaseFirst(primaryKeys[0]) + ` = ` + insertIdStr + `
			}
			if err != nil {
				err = errors.Wrap(err, "save failed for ` + table + `")
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

// Delete removes a record from the database according to the primary key
func (obj *` + tableNaming + `) ` + funcName + `Delete(ctx context.Context) (sql.Result, error) {
	return ` + funcName + `Exec(ctx, "DELETE FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrQueryValues + `)
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

		// create ReadByKey method
		string1 += `
// ReadByKey returns a single pointer to a(n) ` + tableNaming + `
func Read` + funcName + `ByKey(ctx context.Context, ` + paramStr + `) (*` + tableNaming + `, error) {
	return ReadOne` + funcName + `ByQuery(ctx, "SELECT * FROM ` + table + ` WHERE` + whereStrQuery + `", ` + whereStrValues + `)
}`
	}

	string1 += `

// ReadAll returns all records in the table
func ReadAll` + funcName + `(ctx context.Context, options ...connection.QueryOptions) ([]*` + tableNaming + `, error) {
	return Read` + funcName + `ByQuery(ctx, "SELECT * FROM ` + table + `", options)
}

// ReadByQuery returns an array of ` + tableNaming + ` pointers
func Read` + funcName + `ByQuery(ctx context.Context, query string, args ...interface{}) ([]*` + tableNaming + `, error) {
	var objects []*` + tableNaming + `

	con, err := connection.Get("` + g.Database + `")
	if err != nil {
		return objects, errors.Wrap(err, "connection failed")
	}

	newArgs := connection.ApplyQueryOptions(&query, args)
	query = strings.Replace(query, "'", "\"", -1)
	rows, err := con.QueryContext(ctx, query, newArgs...)
	if err != nil {
		return objects, errors.Wrap(err, "query error")
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return objects, errors.Wrap(err, "rows error")
	}

	defer rows.Close()
	for rows.Next() {
		var obj ` + lowerTable + `
		err = rows.Scan(&obj.` + uppercaseFirst(objects[0].Name) + scanStr + `)
		if err != nil {
			return objects, errors.Wrap(err, "scan error")
		}
		objects = append(objects, &` + tableNaming + `{obj.` + uppercaseFirst(objects[0].Name) + scanStr2 + `})
	}

	if len(objects) == 0 {
		err = errors.Wrap(sql.ErrNoRows, "no records found")
	}

	return objects, err
}

// ReadOneByQuery returns a single pointer to a(n) ` + tableNaming + `
func ReadOne` + funcName + `ByQuery(ctx context.Context, query string, args ...interface{}) (*` + tableNaming + `, error) {
	var obj ` + lowerTable + `

	con, err := connection.Get("` + g.Database + `")
	if err != nil {
		return nil, errors.Wrap(err, "connection failed")
	}

	query = strings.Replace(query, "'", "\"", -1)
	err = con.QueryRowContext(ctx, query, args...).Scan(&obj.` + uppercaseFirst(objects[0].Name) + scanStr + `)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "query/scan error")
	}

	return &` + tableNaming + `{obj.` + uppercaseFirst(objects[0].Name) + scanStr2 + `}, nil
}

// Exec allows for update queries
func ` + funcName + `Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	con, err := connection.Get("` + g.Database + `")
	if err != nil {
		return nil, errors.Wrap(err, "connection failed")
	}
	return con.ExecContext(ctx, query, args...)
}`

	importString += "\n)"

	autoGenFile := dir + tableNaming + "_base.go"
	// g.write <- filewrite{autoGenFile, initialString + importString + string1, true}
	err := writeFile(autoGenFile, initialString+importString+string1, true)
	if err != nil {
		g.errorChan <- err
	}

	_, err = runCommand("go fmt " + autoGenFile)
	if err != nil {
		g.errorChan <- err
	}

	return nil
}

// buildExtended builds the {table}_extends.go file for custom functions & methods
func (g Gostruct) buildExtended(table string) {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + tableNaming + "/"
	extendedFilePath := dir + tableNaming + "_extended.go"

	if !exists(extendedFilePath) {
		contents := "package " + tableNaming + "\n\n// Methods Here"
		// g.write <- filewrite{daoFilePath, contents, false}
		err := writeFile(extendedFilePath, contents, false)
		if err != nil {
			g.errorChan <- err
		}
		_, err = runCommand("go fmt " + extendedFilePath)
		if err != nil {
			g.errorChan <- err
		}
	}
}

// buildTest builds the skeleton {table}_test.go file to hold all unit tests
func (g Gostruct) buildTest(table string) {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + tableNaming + "/"
	testFilePath := dir + tableNaming + "_test.go"

	if !exists(testFilePath) {
		contents := `package ` + tableNaming + `_test

		import (
			"testing"
		)

		func TestSomething(t *testing.T) {
			// test stuff here..
		}`
		// g.write <- filewrite{testFilePath, contents, false}
		err := writeFile(testFilePath, contents, false)
		if err != nil {
			g.errorChan <- err
		}
		_, err = runCommand("go fmt " + testFilePath)
		if err != nil {
			g.errorChan <- err
		}
	}
}

// buildExtractPkg.. well, builds the extract package
func buildExtractPkg() error {
	filePath := GOPATH + "/src/utils/extract/extract.go"
	if !exists(GOPATH + "/src/utils/extract") {
		err := createDirectory(GOPATH + "/src/utils/extract")
		if err != nil {
			return err
		}
	}

	if !exists(filePath) {
		contents := `package extract

import (
	"database/sql"
	"github.com/pkg/errors"
	"reflect"
	"strings"
	"time"
)

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
		if v == int64(0) {
			empty = true
		}
	case float64:
		if v == float64(0) {
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
	case *string, *int, *int64, *float64, *bool, *time.Time:
		if v == nil {
			empty = true
		}
	}
	return empty
}

func GetValue(val reflect.Value, field reflect.StructField) (interface{}, error) {
	var value interface{}

	column := field.Tag.Get("column")
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
			return nil, errors.New("Invalid value: '" + s + "' for column: " + column + ". Possible values are: " + strings.Join(arr, ", "))
		}
	}

	if val.Kind() == reflect.Interface && !val.IsNil() {
		elm := val.Elem()
		if elm.Kind() == reflect.Ptr && !elm.IsNil() && elm.Elem().Kind() == reflect.Ptr {
			val = elm
		}
	}

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			value = nil
		} else {
			value = val.Elem().Interface()
		}
	} else {
		value = val.Interface()
	}

	if isEmpty(value) && field.Tag.Get("key") != "PRI" {
		value = field.Tag.Get("default")
		if value == "" && field.Tag.Get("null") == "NO" {
			return nil, errors.New("you must provide a value for column: " + column)
		}
	}

	return value, nil
}

// Returns string between two specified characters/strings
func Between(initial string, beginning string, end string) string {
	return strings.TrimLeft(strings.TrimRight(initial, end), beginning)
}

// Determine whether or not a string is in array
func InArray(char string, strings []string) bool {
	for _, a := range strings {
		if a == char {
			return true
		}
	}
	return false
}

`
		err := writeFile(filePath, contents, false)
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

// buildConnectionPkg builds the main connection package for serving up all database connections
// with a shared connection pool
func buildConnectionPkg() error {
	if !exists(GOPATH + "/src/connection") {
		err := createDirectory(GOPATH + "/src/connection")
		if err != nil {
			return err
		}
	} else if exists(GOPATH + "/src/connection/connection.go") {
		return nil
	}

	bs := "`"
	conFilePath := GOPATH + "/src/connection/connection.go"
	contents := `// Package connection handles all connections to the MySQL database(s)
package connection

import (
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"sync"
	"utils/extract"

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

// Connections holds the list of database connections
type Connections struct {
	list map[string]*sql.DB
	sync.Mutex
}

// QueryOptions allows for passing optional parameters for queries
type QueryOptions struct {
	OrderBy string
	Limit   int
}

// Get returns a connection to a specific database. If the connection exists in the connections list AND is
// still active, it will just return that connection. Otherwise, it will open a new connection to
// the specified database and add it to the connections list.
func Get(db string) (*sql.DB, error) {
	connections.Lock()
	defer connections.Unlock()

	connection := connections.list[db]
	if connection != nil {
		// determine if connection is still active
		err = connection.Ping()
		if err == nil {
			return connection, err
		}
	}

	con, err := sql.Open("mysql", fmt.Sprintf("root:asdfjklasdf@tcp(localhost:3306)/%s?parseTime=true", db))
	if err != nil {
		// do whatever tickles your fancy here
		log.Fatalln("Connection Error to DB [", db, "]", err.Error())
	}
	con.SetMaxIdleConns(0)
	con.SetMaxOpenConns(50)

	connections.list[db] = con

	return con, nil
}

// ApplyQueryOptions takes in a slice of interfaces from a query and applies the QueryOptions struct
func ApplyQueryOptions(query *string, args []interface{}) []interface{} {
	var newArgs []interface{}
	for _, arg := range args {
		switch t := arg.(type) {
		case []QueryOptions:
			if len(t) > 0 {
				options := t[0]
				orderBy := options.OrderBy
				if orderBy != "" {
					*query += fmt.Sprintf(" ORDER BY %s", orderBy)
				}
				limit := options.Limit
				if limit != 0 {
					*query += fmt.Sprintf(" LIMIT %d", limit)
				}
			}
		case QueryOptions:
			orderBy := t.OrderBy
			if orderBy != "" {
				*query += fmt.Sprintf(" ORDER BY %s", orderBy)
			}
			limit := t.Limit
			if limit != 0 {
				*query += fmt.Sprintf(" LIMIT %d", limit)
			}
		default:
			newArgs = append(newArgs, t)
		}
	}

	return newArgs
}

// BuildQuery returns all necessary arguments for the Save method of a type
func BuildQuery(v reflect.Value, valType reflect.Type) ([]interface{}, []string, []string, string, error) {
	var columns []string
	var q []string
	var updateStr string
	var args []interface{}

	for i := 0; i < v.NumField(); i++ {
		val, err := extract.GetValue(v.Field(i), valType.Field(i))
		if err != nil {
			return nil, columns, q, "", err
		}
		args = append(args, val)
		column := string(valType.Field(i).Tag.Get("column"))
		columns = append(columns, "` + bs + `"+column+"` + bs + `")
		q = append(q, "?")
		if i > 0 && updateStr != "" {
			updateStr += ", "
		}
		updateStr += "` + bs + `" + column + "` + bs + ` = ?"
	}

	return args, columns, q, updateStr, nil
}

`
	err := writeFile(conFilePath, contents, false)
	if err != nil {
		return err
	}

	_, err = runCommand("go fmt " + conFilePath)
	if err != nil {
		return err
	}

	return nil
}

// startTimer keeps a timer of the duration of the process
func startTimer(g *Gostruct) func() {
	t := time.Now()
	return func() {
		d := time.Now().Sub(t)
		printNoSpace("\n\n======= Results =======\n")
		fmt.Println("Processed:", g.processed)
		fmt.Println("Duration:", d)

		if g.errored > 0 {
			printNoSpace("\n\n======= Errors: ", g.errored, "/", g.processed, " =======\n")
			for i, err := range g.errors {
				fmt.Println(i+1, ":", err.Error())
			}
		}
	}
}

// getConnection is a helper to return a connection & an error
func getConnection(gs Gostruct) (*sql.DB, error) {
	return sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", gs.Username, gs.Password, gs.Host, gs.Port, gs.Database))
}

// showProgress prints how many tables have been processed compared to the total number
func showProgress(g Gostruct) {
	printNoSpace("Progress.. ", g.processed, "/", g.total, "\r")
}

// printNoSpace is a println implementation without automatically putting a space between args
func printNoSpace(args ...interface{}) {
	var s string
	for _, arg := range args {
		s += fmt.Sprint(arg)
	}

	fmt.Print(s)
}
