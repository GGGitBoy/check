package db

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
)

// CreateReport inserts a new report into the database.
func CreateReport(report *apis.Report) error {
	data, err := json.Marshal(report.Kubernetes)
	if err != nil {
		return fmt.Errorf("Error marshaling Kubernetes data: %v\n", err)
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("Error beginning transaction: %v\n", err)
	}

	stmt, err := tx.Prepare("INSERT INTO report(id, name, rating, report_time, data) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error preparing statement: %v\n", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(report.ID, report.Global.Name, report.Global.Rating, report.Global.ReportTime, string(data))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error executing statement: %v\n", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Error committing transaction: %v\n", err)
	}

	logrus.Infof("[DB] Report created successfully with ID: %s", report.ID)
	return nil
}

// GetReport retrieves a report from the database by ID.
func GetReport(reportID string) (*apis.Report, error) {
	row := DB.QueryRow("SELECT id, name, rating, report_time, data FROM report WHERE id = ? LIMIT 1", reportID)

	var id, name, rating, reportTime, data string
	report := apis.NewReport()
	err := row.Scan(&id, &name, &rating, &reportTime, &data)
	if err != nil {
		return nil, fmt.Errorf("Error scanning row: %v\n", err)
	}

	var dataKubernetes []*apis.Kubernetes
	err = json.Unmarshal([]byte(data), &dataKubernetes)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling Kubernetes data: %v\n", err)
	}

	report = &apis.Report{
		ID: id,
		Global: &apis.Global{
			Name:       name,
			Rating:     rating,
			ReportTime: reportTime,
		},
		Kubernetes: dataKubernetes,
	}

	logrus.Infof("[DB] Report get successfully with ID: %s", report.ID)
	return report, nil
}

// DeleteReport removes a report from the database by ID.
func DeleteReport(reportID string) error {
	result, err := DB.Exec("DELETE FROM report WHERE id = ?", reportID)
	if err != nil {
		return fmt.Errorf("Error executing delete statement: %v\n", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Error getting rows affected: %v\n", err)
	}

	if rowsAffected == 0 {
		logrus.Infof("[DB] No report found to delete with ID: %s", reportID)
	} else {
		logrus.Infof("[DB] Report deleted successfully with ID: %s", reportID)
	}

	return nil
}
