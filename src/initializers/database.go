package initializers

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func DbInstance() *sql.DB {

	username := os.Getenv("database_username")
	password := os.Getenv("database_password")
	dbname := os.Getenv("database_name")
	host := os.Getenv("database_host")
	port := os.Getenv("database_port")

	dbURI := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&multiStatements=true", username, password, host, port, dbname, "utf8")

	// fmt.Println("db connection string %s", dbURI)

	Db, err := sql.Open("mysql", dbURI)

	checkErr(err)

	idleConnection := os.Getenv("database_idle_connection")
	ic, err := strconv.Atoi(idleConnection)

	if err != nil {

		ic = 5
	}

	maxConnection := os.Getenv("database_max_connection")

	mx, err := strconv.Atoi(maxConnection)

	if err != nil {

		mx = 10
	}

	connectionLifetime := os.Getenv("database_connection_lifetime")

	cl, err := strconv.Atoi(connectionLifetime)

	if err != nil {

		cl = 60
	}

	Db.SetMaxIdleConns(ic)
	Db.SetConnMaxLifetime(time.Second * time.Duration(cl))
	Db.SetMaxOpenConns(mx)
	Db.SetConnMaxIdleTime(time.Second * time.Duration(cl))

	err = Db.Ping()
	checkErr(err)
	return Db
}

func checkErr(err error) {

	if err != nil {

		fmt.Println("db connection error", err)
		log.Printf("DB ERROR %s ", err.Error())
	}
}
