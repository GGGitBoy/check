package common

import (
	"os"
)

var (
	ServerURL   = os.Getenv("SERVER_URL")
	BearerToken = os.Getenv("BEARER_TOKEN")

	MySQL         = os.Getenv("MY_SQL")
	MySQLUser     = os.Getenv("MY_SQL_USER")
	MySQLPassword = os.Getenv("MY_SQL_PASSWORD")
	MySQLHost     = os.Getenv("MY_SQL_HOST")
	MySQLPort     = os.Getenv("MY_SQL_PORT")
	MySQLDB       = os.Getenv("MY_SQL_DB")

	SQLiteName = "sqlite.db"

	PrintWaitSecond = os.Getenv("PRINT_WAIT_SECOND")

	LocalCluster        = "local"
	InspectionNamespace = "cattle-inspection-system"

	//WorkDir = "/Users/chenjiandao/jiandao/inspection-server/opt/"
	WorkDir = "/opt/"

	ConfigFilePath = WorkDir + "config/config.yml"

	PrintShotPath = WorkDir + "print/screenshot.png"
	PrintPDFPath  = WorkDir + "print/report.pdf"
	PrintPDFName  = "report.pdf"

	WriteKubeconfigPath = WorkDir + "kubeconfig/"

	SendTestPDFPath = WorkDir + SendTestPDFName
	SendTestPDFName = "test.pdf"

	AgentYamlPath = WorkDir + "yaml/"
)
