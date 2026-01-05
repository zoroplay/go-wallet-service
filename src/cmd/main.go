package main

import (
	"fmt"
	"path/filepath"
	"runtime"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
	"github.com/zoroplay/go-wallet-service/initializers"
	"github.com/zoroplay/go-wallet-service/routes"
)

func main() {
	initializers.LoadEnvVariables()

	//setup database
	dbInstance := initializers.DbInstance()

	driver, err := mysql.WithInstance(dbInstance, &mysql.Config{})
	if err != nil {

		logrus.Panic(err)
	}

	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file:///%s/migrations", GetRootPath()), "mysql", driver)
	if err != nil {

		logrus.Errorf("migration setup error %s ", err.Error())
	}

	err = m.Up() // or m.Step(2) if you want to explicitly set the number of migrations to run
	if err != nil {

		logrus.Errorf("migration error %s ", err.Error())
	}

	// setup consumers
	var a routes.App
	a.Initialize()
	go a.GRPC()
	a.Run()

}

func GetRootPath() string {

	_, b, _, _ := runtime.Caller(0)

	// Root folder of this project
	return filepath.Join(filepath.Dir(b), "./")
}
