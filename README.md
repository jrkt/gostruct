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

    package Realtor

    import (
    	"database/sql"
    	"connection"
    	"reflect"
    	"strconv"
    	"errors"
    	"models/Company"
    	"models/Office"
    )
    
    type RealtorObj struct {
    	Realtorid		string
    	Type		string
    	Fname		sql.NullString
    	Lname		sql.NullString
    	Phone		sql.NullString
    	Cell		sql.NullString
    	Fax		sql.NullString
    	Email		string
    	Website		sql.NullString
    	Agency		sql.NullString
    	Office		sql.NullString
    	Associd		sql.NullString
    	Extid		string
    	Acct_status		string
    	Address		sql.NullString
    	City		sql.NullString
    	County		sql.NullString
    	State		sql.NullString
    	Zip		sql.NullString
    	Signupdate		sql.NullString
    	Notes		sql.NullString
    	Permit_email		string
    	Referral		sql.NullString
    	Refferedby		sql.NullString
    	Product		string
    	Credit		string
    	Bio		sql.NullString
    	HasLeadGen		string
    	EmailOptOut		string
    	VaRealtorStatus		string
    	VaClientId		sql.NullString
    	RealtorLogin		sql.NullString
    	RealtorPassword		sql.NullString
    	Acceptsms		string
    	MobileProvider		string
    	Testimonial		sql.NullString
    	StateLicenseNumber		sql.NullString
    	PersonalYouTubeId		sql.NullString
    	Md5Hash		sql.NullString
    	CacheExpire		sql.NullString
    	LeadbeePin		sql.NullString
    	LastLogin		sql.NullString
    	DefaultOrbitalRefNum		sql.NullString
    	FacebookURL		sql.NullString
    	TwitterURL		sql.NullString
    	YouTubeURL		sql.NullString
    	BlogURL		sql.NullString
    	EmailBounced		sql.NullString
    	PictureURL		sql.NullString
    	CompletedSetup		sql.NullString
    	TempCredit		sql.NullString
    	InfusionSoftTags		sql.NullString
    }
    
    var primaryKey = "realtorid"
    
    func Save(Object RealtorObj) {
    	v := reflect.ValueOf(&Object).Elem()
    	objType := v.Type()
    
    	firstValue := reflect.Value(v.Field(1)).String()
    	if firstValue == "<sql.NullString Value>" {
    		firstValue = "null"
    	} else {
    		firstValue = "'" + firstValue + "'"
    	}
    
    	query := "UPDATE realtor SET " + objType.Field(1).Name + " = " + firstValue
    
    	for i := 2; i < v.NumField(); i++ {
    		property := string(objType.Field(i).Name)
    		value := reflect.Value(v.Field(i)).String()
    		if value == "<sql.NullString Value>" {
    			value = "null"
    		} else {
    			value = "'" + value + "'"
    		}
    
    		query += ", " + property + " = " + value
    	}
    	query += " WHERE " + primaryKey + " = '" + Object.Realtorid + "'"
    
    	con := connection.GetConnection()
    	_, err := con.Exec(query)
    	if err != nil {
    		panic(err.Error())
    	}
    }
    
    func ReadById(id int) (RealtorObj, error) {
    	con := connection.GetConnection()
    
    	var realtor RealtorObj
    	err := con.QueryRow("SELECT * FROM realtor WHERE realtorid = ?", strconv.Itoa(id)).Scan(&realtor.Realtorid, &realtor.Type, &realtor.Fname, &realtor.Lname, &realtor.Phone, &realtor.Cell, &realtor.Fax, &realtor.Email, &realtor.Website, &realtor.Agency, &realtor.Office, &realtor.Associd, &realtor.Extid, &realtor.Acct_status, &realtor.Address, &realtor.City, &realtor.County, &realtor.State, &realtor.Zip, &realtor.Signupdate, &realtor.Notes, &realtor.Permit_email, &realtor.Referral, &realtor.Refferedby, &realtor.Product, &realtor.Credit, &realtor.Bio, &realtor.HasLeadGen, &realtor.EmailOptOut, &realtor.VaRealtorStatus, &realtor.VaClientId, &realtor.RealtorLogin, &realtor.RealtorPassword, &realtor.Acceptsms, &realtor.MobileProvider, &realtor.Testimonial, &realtor.StateLicenseNumber, &realtor.PersonalYouTubeId, &realtor.Md5Hash, &realtor.CacheExpire, &realtor.LeadbeePin, &realtor.LastLogin, &realtor.DefaultOrbitalRefNum, &realtor.FacebookURL, &realtor.TwitterURL, &realtor.YouTubeURL, &realtor.BlogURL, &realtor.EmailBounced, &realtor.PictureURL, &realtor.CompletedSetup, &realtor.TempCredit, &realtor.InfusionSoftTags)
    
    	switch {
    	case err == sql.ErrNoRows:
    		return realtor, errors.New("ERROR Realtor::ReadById - No result")
    	case err != nil:
    		return realtor, errors.New("ERROR Realtor::ReadById - " + err.Error())
    	default:
    		return realtor, nil
    	}
    
    	return realtor, nil
    }
    
    func GetCompany(Object RealtorObj) (Company.CompanyObj, error) {
    	con := connection.GetConnection()
    
    	var company Company.CompanyObj
    	err := con.QueryRow("SELECT Company.* FROM Company INNER JOIN realtor ON Company.CMPid = realtor.product WHERE realtor.product = ?", Object.Product).Scan(&company.CMPid, &company.CMPname, &company.CMPadmin, &company.CMPtype, &company.CMPnotes, &company.CMPbill, &company.CMPbillType, &company.CMPbillingLoc, &company.CMPlaunch, &company.CMPcontractStart, &company.CMPcontractEnd, &company.CMPguarantee, &company.CMPrep, &company.CMPcolorScheme, &company.CMPparticipate, &company.CMPstate, &company.CMPsalesRep, &company.CMPhasLeadGen, &company.CMPemailPromos, &company.CMPrelaunch, &company.CMPnumAgents, &company.CMProster, &company.PricingType, &company.VaCompanyStatus, &company.CompanyInformation, &company.CompanyPriceDetails, &company.NextPromoEmailDate, &company.PromoEmailFrequency, &company.ParentCMPid, &company.Extid, &company.CacheExpire, &company.IsLOYTAccount, &company.UrlLookUp, &company.WebExTrainingVideo, &company.WebExVideoHeader, &company.WebExVideoText, &company.Addr1, &company.Addr2, &company.City, &company.Zip, &company.Phone, &company.DisplayName, &company.ExternalSubdomain, &company.BillingRealtorId, &company.BillingProfileId, &company.GoogleAnalyticsCode, &company.Country, &company.StarMarketingStatus, &company.CompanyBrandID, &company.Credit, &company.IsFake)
    
    	switch {
    	case err == sql.ErrNoRows:
    		return company, errors.New("ERROR Realtor::GetCompany - No result")
    	case err != nil:
    		return company, errors.New("ERROR Realtor::GetCompany - " + err.Error())
    	default:
    		return company, nil
    	}
    
    	return company, nil
    }
    
    func GetOffice(Object RealtorObj) (Office.OfficeObj, error) {
    	con := connection.GetConnection()
    
    	var office Office.OfficeObj
    	err := con.QueryRow("SELECT Office.* FROM Office INNER JOIN realtor ON Office.OFFid = realtor.office WHERE realtor.office = ?", Object.Office).Scan(&office.OFFid, &office.OFF_COMPid, &office.OFFname, &office.OFFadmin, &office.OFFcode, &office.OFFexportOfficeId, &office.OFFimportOfficeId, &office.Addr1, &office.Addr2, &office.City, &office.State, &office.Zip, &office.Phone, &office.RealtorComMLSKey, &office.RealtorComMLSCode, &office.BillingProfile, &office.Credit)
    
    	switch {
    	case err == sql.ErrNoRows:
    		return office, errors.New("ERROR Realtor::GetOffice - No result")
    	case err != nil:
    		return office, errors.New("ERROR Realtor::GetOffice - " + err.Error())
    	default:
    		return office, nil
    	}
    
    	return office, nil
    }
