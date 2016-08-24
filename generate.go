package gostruct

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"errors"
	"os"
	"os/exec"
)

type TableObj struct {
	Name       string
	IsNullable string
	Key        string
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
var primaryKey string
var tables []string
var tablesDone []string
var foreignKeys []KeyObj
var GOPATH string

func Run(table string, database string, host string, port string) error {
	GOPATH = os.Getenv("GOPATH")

	if port == "" {
		port = "3306"
	}

	//make sure models dir exists
	if !exists(GOPATH + "/src/models") {
		err = os.Mkdir(GOPATH + "/src/models", 0777)
		if err != nil {
			return err
		}

		//give new directory full permissions
		err = os.Chmod(GOPATH + "/src/models", 0777)
		if err != nil {
			return err
		}
	}

	if !exists(GOPATH + "/src/connection") {
		//create connection package
		err = os.Mkdir(GOPATH + "/src/connection", 0777)
		if err != nil {
			return err
		}

		//give new directory full permissions
		err = os.Chmod(GOPATH + "/src/connection", 0777)
		if err != nil {
			return err
		}
	}

	err = buildConnectionFile(host, database)

	cnt := 0
	tables = append(tables, table)
	for {
		err = handleTable(tables[cnt], database, host, port)
		if err != nil {
			return err
		}

		if cnt == len(tables) - 1 {
			break
		}
		cnt++
	}

	return nil
}

func handleTable(table string, database string, host string, port string) error {
	if inStringArray(table, tablesDone) {
		return nil
	} else {
		tablesDone = append(tablesDone, table)
	}

	log.Println("Generating Models for: " + table)

	con, err = sql.Open("mysql", DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":" + port + ")/" + database)

	if err != nil {
		return err
	}

	rows1, err := con.Query("SELECT column_name, is_nullable, column_key FROM information_schema.columns WHERE table_name = ? AND table_schema = ?", table, database)

	var object TableObj
	var objects []TableObj = make([]TableObj, 0)
	var columns []string

	if err != nil {
		return err
	} else {
		cntPK := 0
		for rows1.Next() {
			rows1.Scan(&object.Name, &object.IsNullable, &object.Key)
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

		//get ForeignKeys
		rows2, err := con.Query("SELECT table_name, column_name, referenced_table_name, referenced_column_name FROM information_schema.key_column_usage WHERE table_name = ? AND table_schema = ?", table, database)

		var key = KeyObj{}
		foreignKeys = make([]KeyObj, 0)

		if err != nil {
			return err
		} else {
			for rows2.Next() {
				rows2.Scan(&key.TableName, &key.ColumnName, &key.ReferencedTable, &key.ReferencedColumn)
				if key.ColumnName != primaryKey && inStringArray(key.ColumnName, columns) && key.ReferencedTable.String != "" && key.TableName != key.ReferencedTable.String {
					foreignKeys = append(foreignKeys, key)
					tables = append(tables, key.ReferencedTable.String)
				}
			}
		}
		defer rows2.Close()

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

		//handle Crux file
		err = buildCruxFile(objects, table, database)

		//handle Normal file
		err = buildNormalFile(table)

		//handle Test file
		err = buildTestFile(table)
	}

	return nil
}

func buildConnectionFile(host string, database string) error {
	connectionFile, err := os.Create(GOPATH + "/src/connection/connection.go")
	defer connectionFile.Close()
	if err != nil {
		return err
	}
	contents := `package connection

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func GetConnection() *sql.DB {
	con, err := sql.Open("mysql", "` + DB_USERNAME + `:` + DB_PASSWORD + `@tcp(` + host + `:3306)/` + database + `")
	if err != nil {
		panic(err)
	}

	return con
}`
	_, err = connectionFile.WriteString(contents)
	if err != nil {
		return err
	}

	return nil
}

func buildCruxFile(objects []TableObj, table string, database string) error {

	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var usedColumns []UsedColumn
	initialString := `package ` + uppercaseFirst(table)
	importString := `

import (
	"database/sql"`

	string := "\n\ntype " + uppercaseFirst(table) + "Obj struct {"
	string2 := ""
	contents := ""

	cntPK := 0
	Loop:
	for i := 0; i < len(objects); i++ {
		object := objects[i]
		for c := 0; c < len(usedColumns); c++ {
			if usedColumns[c].Name == object.Name {
				continue Loop
			}
		}
		usedColumns = append(usedColumns, UsedColumn{Name: object.Name})
		if object.Key == "PRI" {
			cntPK++
			if cntPK == 1 {
				primaryKey = object.Name
			}
		}

		dataType := ""
		if object.IsNullable == "NO" {
			dataType = "string"
		} else {
			dataType = "sql.NullString"
		}
		if cntPK > 1 && object.Key == "PRI" {
			string2 += ""
		} else if i > 0 {
			string2 += ", &object." + uppercaseFirst(object.Name)
		}
		string += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType + "\t\t`column:\"" + object.Name + "\"`"
	}
	string += "\n}"

	if cntPK == 1 {
		importString += `
			"reflect"
			"db/` + database + `"`
		string += "\n\nvar primaryKey = \"" + primaryKey + "\"\n"

		string += `
func Create() *` + uppercaseFirst(table) + `Obj {
	return &` + uppercaseFirst(table) + `Obj{}
}

func (Object ` + uppercaseFirst(table) + `Obj) Save() {
	v := reflect.ValueOf(&Object).Elem()
	objType := v.Type()

	var firstValue string
	if v.Field(0).Type() == reflect.TypeOf(sql.NullString{}) {
		if reflect.Value(v.Field(0)).Field(0).String() == "" {
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

	if Object.` + uppercaseFirst(primaryKey) + ` == "" {
		query = "INSERT INTO ` + table + ` "
		columns += "("
		if firstValue != "null" {
			columns += string(objType.Field(1).Tag.Get("column")) + ","
			values += firstValue + ","
		}
	} else {
		query = "UPDATE ` + table + ` SET " + string(objType.Field(1).Tag.Get("column")) + " = " + firstValue
	}

	for i := 1; i < v.NumField(); i++ {
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

		if Object.` + uppercaseFirst(primaryKey) + ` == "" {
			if value != "null" {
				columns += string(objType.Field(i).Tag.Get("column")) + ","
				values += value + ","
			}
		} else {
			query += ", " + string(objType.Field(i).Tag.Get("column")) + " = " + value
		}
	}
	if Object.` + uppercaseFirst(primaryKey) + ` == "" {
		query += columns[:len(columns) - 1] + ") VALUES (" + values[:len(values) - 1] + ")"
	} else {
		query += " WHERE " + primaryKey + " = '" + Object.` + uppercaseFirst(primaryKey) + ` + "'"
	}

	con := db.GetConnection()
	_, err := con.Exec(query)
	if err != nil {
		panic(err.Error())
	}
}

func (Object ` + uppercaseFirst(table) + `Obj) Delete() {
	query := "DELETE FROM ` + table + ` WHERE ` + primaryKey + ` = '" + Object.` + uppercaseFirst(primaryKey) + ` + "'"

	con := db.GetConnection()
	_, err := con.Exec(query)
	if err != nil {
		panic(err.Error())
	}
}`

		//create ReadById method
		string += `

func ReadById(id string) ` + uppercaseFirst(table) + `Obj {
	var object ` + uppercaseFirst(table) + `Obj

	con := db.GetConnection()
	err := con.QueryRow("SELECT * FROM ` + table + ` WHERE ` + primaryKey + ` = ?", id).Scan(&object.` + uppercaseFirst(objects[0].Name) + string2 + `)

	switch {
	case err == sql.ErrNoRows:
		//do something?
	case err != nil:
		panic(err)
	}

	return object
}`

		//create foreign key methods
		for i := 0; i < len(foreignKeys); i++ {
			if foreignKeys[i].ReferencedTable.String == "DatabaseEnum" {
				continue
			}
			rows3, err := con.Query("SELECT column_name, is_nullable, column_key FROM information_schema.columns WHERE table_name = ? AND table_schema = ?", foreignKeys[i].ReferencedTable.String, database)
			defer rows3.Close()

			var object TableObj
			var objects2 []TableObj = make([]TableObj, 0)

			if err != nil {
				log.Fatalln(err.Error())
			} else {
				for rows3.Next() {
					rows3.Scan(&object.Name, &object.IsNullable, &object.Key)
					objects2 = append(objects2, object)
				}
			}

			if len(objects2) == 0 {
				log.Fatalln(errors.New("No results for table: " + foreignKeys[i].ReferencedTable.String))
			} else {
				if uppercaseFirst(foreignKeys[i].ReferencedTable.String) == table {
					continue
				}

				importString += `
					"models/` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `"`

				string += `

func (Object ` + uppercaseFirst(table) + `Obj) Get` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `() ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `Obj {
	var object ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `Obj

	con := db.GetConnection()
	err := con.QueryRow("SELECT ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `.* FROM ` + foreignKeys[i].ReferencedTable.String + ` INNER JOIN ` + foreignKeys[i].TableName + ` ON ` + foreignKeys[i].ReferencedTable.String + `.` + foreignKeys[i].ReferencedColumn.String + ` = ` + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName + ` WHERE ` + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName + ` = ?", Object.` + uppercaseFirst(foreignKeys[i].ColumnName) + `).Scan(&object.` + uppercaseFirst(objects2[0].Name)

				for o := 0; o < len(objects2); o++ {
					object2 := objects2[o]

					if object2.Key == "PRI" {
						primaryKey = object2.Name
					}
					if o > 0 {
						string += ", &object." + uppercaseFirst(object2.Name)
					}
				}

				string += `)

	switch {
	case err == sql.ErrNoRows:
		//do something?
	case err != nil:
		panic(err)
	}

	return object
}`
			}
		}
	}
	importString += "\n)"
	contents = initialString + importString + string

	cruxFilePath := dir + tableNaming + "_Crux.go"
	if exists(cruxFilePath) {
		err = os.Remove(cruxFilePath)
		if err != nil {
			return err
		}
	}

	cruxFile, err := os.Create(cruxFilePath)
	defer cruxFile.Close()
	if err != nil {
		return err
	}

	_, err = cruxFile.WriteString(contents)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "fmt", tableNaming + "/" + tableNaming + "_Crux.go")
	cmd.Run()

	return nil
}

func buildNormalFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var normalFile *os.File
	normalFilePath := dir + tableNaming + ".go"
	if !exists(normalFilePath) {
		normalFile, err = os.Create(normalFilePath)
		defer normalFile.Close()
		if err != nil {
			return err
		}

		contents := "package " + uppercaseFirst(table) + "\n\n//Methods Here"
		_, err = normalFile.WriteString(contents)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command("go", "fmt", tableNaming + "/" + tableNaming + ".go")
	cmd.Run()

	return nil
}

func buildTestFile(table string) error {
	tableNaming := uppercaseFirst(table)
	dir := GOPATH + "/src/models/" + uppercaseFirst(table) + "/"

	var testFile *os.File
	testFilePath := dir + tableNaming + "_test.go"
	if !exists(testFilePath) {
		testFile, err = os.Create(testFilePath)
		defer testFile.Close()
		if err != nil {
			return err
		}

		contents := `package ` + tableNaming + `_test

		import (
			"testing"
		)

		func TestSomething(t *testing.T) {
			//test stuff here..
		}`
		_, err = testFile.WriteString(contents)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command("go", "fmt", tableNaming + "/" + tableNaming + "_test.go")
	cmd.Run()

	return nil
}

func inStringArray(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}
