package gostruct

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"errors"
	"os"
	"strings"
	"os/exec"
	"utils/inarray"
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

func Run(table string, database string, host string) error {
	GOPATH = os.Getenv("GOPATH")

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
		err = handleTable(tables[cnt], database, host)
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

func handleTable(table string, database string, host string) error {
	if inarray.InStringArray(table, tablesDone) {
		return nil
	} else {
		tablesDone = append(tablesDone, table)
	}

	log.Println("Generating Base Classes for: " + table)

	con, err = sql.Open("mysql", DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":3306)/" + database)

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
		rows2, err := con.Query("SELECT table_name, column_name, referenced_table_name, referenced_column_name FROM information_schema.key_column_usage WHERE table_name = ?", table)

		var key = KeyObj{}
		foreignKeys = make([]KeyObj, 0)

		if err != nil {
			return err
		} else {
			for rows2.Next() {
				rows2.Scan(&key.TableName, &key.ColumnName, &key.ReferencedTable, &key.ReferencedColumn)
				if key.ColumnName != primaryKey && inarray.InStringArray(key.ColumnName, columns) && key.ReferencedTable.String != "" && key.TableName != key.ReferencedTable.String {
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
	initialString := "package " + uppercaseFirst(table) + "\n\n"
	initialString += "import (\n\t\"database/sql\"\n\t\"connection\"\n\t\"reflect\"\n\t\"strconv\"\n\t\"errors\""

	string := "\n\ntype " + uppercaseFirst(table) + "Obj struct {"
	string2 := ""

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
			string += ""
		} else {
			string += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType
		}

		if cntPK > 1 && object.Key == "PRI" {
			string2 += ""
		} else if i > 0 {
			string2 += ", &" + strings.ToLower(table) + "." + uppercaseFirst(object.Name)
		}
	}
	string += "\n}\n\nvar primaryKey = \"" + primaryKey + "\"\n"

	string += `
func Save(Object ` + uppercaseFirst(table) + `Obj) {
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

	query := "UPDATE ` + table + ` SET " + objType.Field(1).Name + " = " + firstValue

	for i := 2; i < v.NumField(); i++ {
		propType := v.Field(i).Type()
		property := string(objType.Field(i).Name)
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

		query += ", " + property + " = " + value
	}
	query += " WHERE " + primaryKey + " = '" + Object.` + uppercaseFirst(primaryKey) + ` + "'"

	con := connection.GetConnection()
	_, err := con.Exec(query)
	if err != nil {
		panic(err.Error())
	}
}`

	//create ReadById method
	string += `

func ReadById(id int) ` + uppercaseFirst(table) + `Obj {
	var ` + strings.ToLower(table) + ` ` + uppercaseFirst(table) + `Obj

	con := connection.GetConnection()
	err := con.QueryRow("SELECT * FROM ` + table + ` WHERE ` + primaryKey + ` = ?", strconv.Itoa(id)).Scan(&` + strings.ToLower(table) + "." + uppercaseFirst(objects[0].Name) + string2 + `)

	switch {
	case err == sql.ErrNoRows:
		println("No result for Id: " + strconv.Itoa(id))
	case err != nil:
		panic(errors.New("ERROR ` + uppercaseFirst(table) + `::ReadById - " + err.Error()))
	default:
		return ` + strings.ToLower(table) + `
	}

	return ` + strings.ToLower(table) + `
}`

	//create foreign key methods
	for i := 0; i < len(foreignKeys); i++ {
		rows3, err := con.Query("SELECT column_name, is_nullable, column_key FROM information_schema.columns WHERE table_name = ?", foreignKeys[i].ReferencedTable.String)
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

			initialString += "\n\t\"models/" + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "\""

			string += `

func Get` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `(Object ` + uppercaseFirst(foreignKeys[i].TableName) + `Obj) ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `Obj {
	var ` + strings.ToLower(foreignKeys[i].ReferencedTable.String) + ` ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `Obj

	con := connection.GetConnection()
	err := con.QueryRow("SELECT ` + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + `.* FROM ` + foreignKeys[i].ReferencedTable.String + ` INNER JOIN ` + foreignKeys[i].TableName + ` ON ` + foreignKeys[i].ReferencedTable.String + `.` + foreignKeys[i].ReferencedColumn.String + ` = ` + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName + ` WHERE ` + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName + ` = ?", Object.` + uppercaseFirst(foreignKeys[i].ColumnName) + `).Scan(&` + strings.ToLower(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(objects2[0].Name)

			for o := 1; o < len(objects2); o++ {
				object2 := objects2[o]

				if object2.Key == "PRI" {
					primaryKey = object2.Name
				}
				string += ", &" + strings.ToLower(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(object2.Name)
			}

			string += `)

	switch {
	case err == sql.ErrNoRows:
		println("No result")
	case err != nil:
		panic(errors.New("ERROR Realtor::GetCompany - " + err.Error()))
	default:
		return ` + strings.ToLower(foreignKeys[i].ReferencedTable.String) + `
	}

	return ` + strings.ToLower(foreignKeys[i].ReferencedTable.String) + `
}`
		}
	}
	initialString += "\n)"
	contents := initialString + string

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
