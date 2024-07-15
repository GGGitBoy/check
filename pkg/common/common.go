package common

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
)

var (
	ServerURL   = getEnv("SERVER_URL", "")
	BearerToken = getEnv("BEARER_TOKEN", "")

	MySQL                 = getEnv("MY_SQL", "")
	MySQLUser             = getEnv("MY_SQL_USER", "")
	MySQLPassword         = getEnv("MY_SQL_PASSWORD", "")
	MySQLHost             = getEnv("MY_SQL_HOST", "")
	MySQLPort             = getEnv("MY_SQL_PORT", "")
	MySQLDB               = getEnv("MY_SQL_DB", "")
	SystemDefaultRegistry = getEnv("SYSTEM_DEFAULT_REGISTRY", "")

	SQLiteName          = "sqlite.db"
	AgentName           = "inspection-agent"
	AgentScriptName     = "inspection-agent-sh"
	InspectionNamespace = "cattle-inspection-system"

	PrintWaitSecond = getEnv("PRINT_WAIT_SECOND", "")

	LocalCluster = "local"

	WorkDir = "/opt/"
	//WorkDir = "/Users/chenjiandao/jiandao/inspection-server/opt/"

	PrintShotPath       = WorkDir + "db/print/screenshot.png"
	PrintPDFPath        = WorkDir + "db/print/"
	WriteKubeconfigPath = WorkDir + "db/kubeconfig/"

	SendTestPDFPath = WorkDir + SendTestPDFName
	SendTestPDFName = "test.pdf"

	AgentYamlPath = WorkDir + "yaml/"
)

// getEnv retrieves the environment variable or returns the default value if not set.
func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		logrus.Infof("Environment variable %s not set, using default value: %s", key, defaultValue)
		return defaultValue
	}
	return value
}

// GetReportFileName generates the report file name using the provided time string.
func GetReportFileName(time string) string {
	fileName := fmt.Sprintf("Report(%s).pdf", time)
	logrus.Debugf("Generated report file name: %s", fileName)
	return fileName
}
