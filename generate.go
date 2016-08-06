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
                err = os.Chmod(GOPATH + "/src/connection", 0777)
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


	connectionFile, err := os.Create(GOPATH + "/src/connection/connection.go")
        defer connectionFile.Close()
                if err != nil {
                        return err
                }

                contents := "package connection\n\nimport (\n\t\"database/sql\"\n\t_ \"github.com/go-sql-driver/mysql\"\n\t\"log\"\n)\n\nfunc GetConnection() *sql.DB {\n\tcon, err := sql.Open(\"mysql\", " + DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":3306)/" + database + ")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\n\treturn con\n}"
                _, err = connectionFile.WriteString(contents)
                if err != nil {
                        return err
                }

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

	tableNaming := uppercaseFirst(table)
	log.Println("Generating Base Classes for: " + table)

	con, err = sql.Open("mysql", DB_USERNAME + ":" + DB_PASSWORD + "@tcp(" + host + ":3306)/" + database)

	if err != nil {
		return err
	}

	rows1, err := con.Query("SELECT column_name, is_nullable, column_key FROM information_schema.columns WHERE table_name = ?", table)
	defer rows1.Close()

	var object TableObj
	var objects []TableObj = make([]TableObj, 0)
	var columns []string

	if err != nil {
		return err
	} else {
		for rows1.Next() {
			rows1.Scan(&object.Name, &object.IsNullable, &object.Key)
			objects = append(objects, object)
			columns = append(columns, object.Name)
		}
	}

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
		if err != nil {
			return err
		}
		defer rows2.Close()

		var key = KeyObj{}
		foreignKeys = make([]KeyObj, 0)

		if err != nil {
			log.Fatalln(err.Error())
		} else {
			for rows2.Next() {
				rows2.Scan(&key.TableName, &key.ColumnName, &key.ReferencedTable, &key.ReferencedColumn)
				if key.ColumnName != primaryKey && inarray.InStringArray(key.ColumnName, columns) && key.ReferencedTable.String != "" {
					foreignKeys = append(foreignKeys, key)
					tables = append(tables, key.ReferencedTable.String)
				}
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

		contents := buildCruxFileContents(objects, table, database)
		_, err = cruxFile.WriteString(contents)
		if err != nil {
			return err
		}

		cmd := exec.Command("go", "fmt", tableNaming + "/" + tableNaming + "_Crux.go")
		cmd.Run()

		//handle Normal file
		var normalFile *os.File
		normalFilePath := dir + tableNaming + ".go"
		if !exists(normalFilePath) {
			normalFile, err = os.Create(normalFilePath)
			defer normalFile.Close()
			if err != nil {
				return err
			}

			contents = buildNormalFileContents(table)
			_, err = normalFile.WriteString(contents)
			if err != nil {
				return err
			}
		}
		cmd = exec.Command("go", "fmt", tableNaming + "/" + tableNaming + ".go")
		cmd.Run()
	}

	return nil
}

func buildCruxFileContents(objects []TableObj, table string, database string) string {

	var usedColumns []UsedColumn
	initialString := "package " + uppercaseFirst(table) + "\n\n"
	initialString += "import (\n\t\"database/sql\"\n\t\"db/" + database + "\"\n\t\"strconv\"\n\t\"errors\""

	string := "\n\ntype " + uppercaseFirst(table) + "Obj struct {"
	string2 := ""

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
			primaryKey = object.Name
		}

		dataType := ""
		if object.IsNullable == "NO" {
			dataType = "string"
		} else {
			dataType = "sql.NullString"
		}
		string += "\n\t" + uppercaseFirst(object.Name) + "\t\t" + dataType
		string2 = ", &" + strings.ToLower(table) + "." + uppercaseFirst(object.Name)
	}
	string += "\n}"

	//create ReadById method
	string += "\n\nfunc ReadById(id int) (" + uppercaseFirst(table) + "Obj, error) {\n\tcon := db.GetConnection()\n\n\tvar " + strings.ToLower(table) + " " + uppercaseFirst(table) + "Obj"
	string += "\n\terr := con.QueryRow(\"SELECT * FROM " + table + " WHERE " + primaryKey + " = ?\", strconv.Itoa(id)).Scan(&" + strings.ToLower(table) + "." + uppercaseFirst(objects[0].Name)
	string += string2
	string += ")\n\n\tswitch {\n\tcase err == sql.ErrNoRows:\n\t\treturn " + strings.ToLower(table) + ", errors.New(\"ERROR " + uppercaseFirst(table) + "::ReadById - No result\")"
	string += "\n\tcase err != nil:\n\t\treturn " + strings.ToLower(table) + ", errors.New(\"ERROR " + uppercaseFirst(table) + "::ReadById - \" + err.Error())"
	string += "\n\tdefault:\n\t\treturn " + strings.ToLower(table) + ", nil\n\t}\n\n\treturn " + strings.ToLower(table) + ", nil\n}"

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
			string += "\n\nfunc Get" + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "(Object " + uppercaseFirst(foreignKeys[i].TableName) + "Obj) (" + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "Obj, error) {"
			string += "\n\tcon := db.GetConnection()\n\n\tvar " + strings.ToLower(foreignKeys[i].ReferencedTable.String) + " " + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + "Obj"
			string += "\n\terr := con.QueryRow(\"SELECT * FROM " + foreignKeys[i].ReferencedTable.String + " INNER JOIN " + foreignKeys[i].TableName + " ON " + foreignKeys[i].ReferencedTable.String
			string += "." + foreignKeys[i].ReferencedColumn.String + " = " + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName + " WHERE " + foreignKeys[i].TableName + "." + foreignKeys[i].ColumnName
			string += " = ?\", Object." + uppercaseFirst(foreignKeys[i].ColumnName) + ").Scan(&" + strings.ToLower(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(objects2[0].Name)
			for o := 1; o < len(objects2); o++ {
				object2 := objects2[o]

				if object2.Key == "PRI" {
					primaryKey = object2.Name
				}
				string += ", &" + strings.ToLower(foreignKeys[i].ReferencedTable.String) + "." + uppercaseFirst(object2.Name)
			}
			string += ")\n\n\tswitch {\n\tcase err == sql.ErrNoRows:\n\t\treturn " + strings.ToLower(foreignKeys[i].ReferencedTable.String) + ", errors.New(\"ERROR " + uppercaseFirst(table) + "::Get" + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + " - No result\")"
			string += "\n\tcase err != nil:\n\t\treturn " + strings.ToLower(foreignKeys[i].ReferencedTable.String) + ", errors.New(\"ERROR " + uppercaseFirst(table) + "::Get" + uppercaseFirst(foreignKeys[i].ReferencedTable.String) + " - \" + err.Error())"
			string += "\n\tdefault:\n\t\treturn " + strings.ToLower(foreignKeys[i].ReferencedTable.String) + ", nil\n\t}\n\n\treturn " + strings.ToLower(foreignKeys[i].ReferencedTable.String) + ", nil\n}"
		}
	}
	initialString += "\n)"
	contents := initialString + string

	return contents
}

func buildNormalFileContents(table string) string {
	string := "package " + uppercaseFirst(table) + "\n\n//Methods Here"

	return string
}
