package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	pdfPrint "inspection-server/pkg/print"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func GetReport() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		reportID := vars["id"]

		report, err := db.GetReport(reportID)
		if err != nil {
			logrus.Errorf("Failed to get report with ID %s: %v", reportID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(report, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal report with ID %s: %v", reportID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write response for report with ID %s: %v", reportID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func PrintReport() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		reportID := vars["id"]

		report, err := db.GetReport(reportID)
		if err != nil {
			logrus.Errorf("Failed to get report with ID %s: %v", reportID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		filePath := filepath.Join(common.PrintPDFPath, common.GetReportFileName(report.Global.ReportTime))
		if !common.FileExists(filePath) {
			logrus.Infof("[API] Report file no exists at path: %s", filePath)
			p := pdfPrint.NewPrint()
			p.URL = "http://127.0.0.1/#/inspection/result-pdf-view/" + report.ID
			p.ReportTime = report.Global.ReportTime
			go pdfPrint.FullScreenshot(p, "Print")
			logrus.Errorf("Failed to get pdf for report with ID %s: %v", reportID, err)
			common.HandleError(rw, http.StatusNotFound, fmt.Errorf("Failed to get pdf for report with ID %s: %v\n", reportID, err))
			return
		}

		file, err := os.Open(filePath)
		if err != nil {
			logrus.Errorf("Failed to open file %s: %v", filePath, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				logrus.Warnf("Failed to close file %s: %v", filePath, err)
			}
		}()

		fileInfo, err := file.Stat()
		if err != nil {
			logrus.Errorf("Failed to get file info for %s: %v", filePath, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		rw.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Length", fmt.Sprint(fileInfo.Size()))

		if _, err := io.Copy(rw, file); err != nil {
			logrus.Errorf("Failed to copy file %s to response: %v", filePath, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}
	})
}
