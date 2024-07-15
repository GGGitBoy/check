package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/common"
	"time"
)

var (
	sqlTables = []string{
		`CREATE TABLE IF NOT EXISTS report (
            id VARCHAR(255) NOT NULL PRIMARY KEY,
            name TEXT,
            rating TEXT, 
            report_time TEXT,
            data LONGTEXT
        );`,
		`CREATE TABLE IF NOT EXISTS task (
            id VARCHAR(255) NOT NULL PRIMARY KEY,
            name TEXT,
            start_time TEXT,
            end_time TEXT,
            cron TEXT,
			state TEXT,
			rating TEXT, 
            report_id TEXT,
			template_id TEXT,
            notify_id TEXT,
			task_id TEXT, 
            mode TEXT, 
			err_message TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS template (
            id VARCHAR(255) NOT NULL PRIMARY KEY,
            name TEXT,
            data LONGTEXT
        );`,
		`CREATE TABLE IF NOT EXISTS notify (
            id VARCHAR(255) NOT NULL PRIMARY KEY,
            name TEXT,
            app_id TEXT,
            app_secret TEXT,
            webhook_url TEXT,
			secret TEXT
        );`,
	}

	DB *sql.DB
)

// Register creates tables if they do not exist
func Register() error {
	DB, err := GetDB()
	if err != nil {
		return fmt.Errorf("Error getting database connection: %v\n", err)
	}

	for _, table := range sqlTables {
		_, err = DB.Exec(table)
		if err != nil {
			return fmt.Errorf("Error executing table creation  SQL: %s, error: %v\n", err, table)
		}
	}

	return nil
}

// GetDB returns a database connection based on configuration
func GetDB() (*sql.DB, error) {
	var err error

	if common.MySQL == "true" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", common.MySQLUser, common.MySQLPassword, common.MySQLHost, common.MySQLPort, common.MySQLDB)
		DB, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("Error opening MySQL connection: %v, DSN: %s\n", err, dsn)
		}
		logrus.Infof("[DB] Connected to MySQL database successfully")
	} else {
		DB, err = sql.Open("sqlite3", common.SQLiteName)
		if err != nil {
			return nil, fmt.Errorf("Error opening SQLite connection: %v, DB Name: %s\n", err, common.SQLiteName)
		}
		logrus.Infof("[DB] Connected to SQLite database successfully")
	}

	DB.SetMaxOpenConns(25)                 // 最大打开的连接数
	DB.SetMaxIdleConns(25)                 // 最大空闲连接数
	DB.SetConnMaxLifetime(5 * time.Minute) // 连接最长存活时间

	if err = DB.Ping(); err != nil {
		return nil, fmt.Errorf("Error pinging database: %v\n", err)
	}
	logrus.Infof("[DB] Database connection successful")

	return DB, nil
}
