# gostruct
This is a library to auto-generate models with packages, structs, and basic methods of accessibility for a given MySQL database table and all other tables related through all foreign key relationships. 

# flags 

table
    
    MySQL database table
    
database
    
    Name of the MySQL database
    
host
    
    Hostname or server of where the database is located

# usage - test.go

Replace the {username} and {password} constants in gostruct.go to the credentials of your database.

    package main

    import (
    	_ "github.com/go-sql-driver/mysql"
    	"github.com/jonathankentstevens/gostruct"
    	"log"
    )

    func main() {
    	err := gostruct.Generate()
    	if err != nil {
    	        panic(err)
    	}
    }

# run it

    go run test.go -table user -database main -host localhost
    
A package with a struct and a method to read by the primary key will be created in the $GOPATH/src/models/ directory. It will also build packages for any other tables that have foreign keys of the table give.

# sample file - sample.go

    package Home

    import (
            "database/sql"
            "db/circlepix"
            "errors"
            "strconv"
    )
    
    type HomeObj struct {
            Homeid                string
            Mlsnum                sql.NullString
            Altmlsnum             sql.NullString
            Rooms                 sql.NullString
            Baths                 sql.NullString
            Sqft                  sql.NullString
            Comments              sql.NullString
            Updated               sql.NullString
            Listdate              sql.NullString
            Expiredate            sql.NullString
            Askingprice           string
            Addr                  string
            City                  string
            County                sql.NullString
            State                 sql.NullString
            Zip                   string
            Country               string
            Date_posted           string
            Status                string
            Id                    string
            PropertyType          sql.NullString
            PropertySubType       sql.NullString
            FullBaths             sql.NullString
            HalfBaths             sql.NullString
            ThreeQuarterBaths     sql.NullString
            QuarterBaths          sql.NullString
            SellType              sql.NullString
            Neighborhood          sql.NullString
    }
    
    func ReadById(id int) (HomeObj, error) {
            con := db.GetConnection()
    
            var home HomeObj
            err := con.QueryRow("SELECT * FROM home WHERE id = ?", strconv.Itoa(id)).Scan(&home.Homeid, &home.Mlsnum, &home.Altmlsnum, &home.Rooms, &home.Baths, &home.Sqft, &home.Comments, &home.Updated, &home.Listdate, &home.Expiredate, &home.Askingprice, &home.Addr, &home.City, &home.County, &home.State, &home.Zip, &home.Country, &home.Date_posted, &home.Status, &home.Id, &home.Photographerid, &home.Date_linked, &home.Contact_notes, &home.LeadGen, &home.Archived, &home.ExclusiveLeads, &home.Directory, &home.NeedsLinking, &home.VaPropertyId, &home.VaHomeStatus, &home.MlsName, &home.ExternalURL, &home.OfficeCode, &home.ExternalImagesUpdated, &home.DateMediaUpdated, &home.SellingPrice, &home.UpdateLOYTImages, &home.CorrectedCity, &home.PropertyType, &home.PropertySubType, &home.FullBaths, &home.HalfBaths, &home.ThreeQuarterBaths, &home.QuarterBaths, &home.SellType, &home.Neighborhood, &home.CustomVideoTitle, &home.IsWaterfront, &home.IsInForeclosure, &home.IsShortSale, &home.IsREO, &home.YearBuilt, &home.SourceImportScriptId, &home.MlsArea, &home.ListingStatus, &home.FrontImageLabel, &home.LastActiveInFeed, &home.DateFeedShowsUpdated)
    
            switch {
            case err == sql.ErrNoRows:
                    return home, errors.New("ERROR Home::ReadById - No result")
            case err != nil:
                    return home, errors.New("ERROR Home::ReadById - " + err.Error())
            default:
                    return home, nil
            }
    
            return home, nil
    }
