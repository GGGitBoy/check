package db

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
)

// GetTemplate retrieves a template from the database by templateID.
func GetTemplate(templateID string) (*apis.Template, error) {
	row := DB.QueryRow("SELECT id, name, data FROM template WHERE id = ? LIMIT 1", templateID)

	var id, name, data string
	template := apis.NewTemplate()
	err := row.Scan(&id, &name, &data)
	if err != nil {
		return nil, fmt.Errorf("Error scanning template row: %v\n", err)
	}

	var dataKubernetesConfig []*apis.KubernetesConfig
	err = json.Unmarshal([]byte(data), &dataKubernetesConfig)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling KubernetesConfig: %v\n", err)
	}

	template = &apis.Template{
		ID:               id,
		Name:             name,
		KubernetesConfig: dataKubernetesConfig,
	}

	logrus.Infof("[DB] Template get successfully with ID: %s", template.ID)
	return template, nil
}

// ListTemplate retrieves all templates from the database.
func ListTemplate() ([]*apis.Template, error) {
	rows, err := DB.Query("SELECT id, name, data FROM template")
	if err != nil {
		return nil, fmt.Errorf("Error executing query: %v\n", err)
	}
	defer rows.Close()

	templates := apis.NewTemplates()
	for rows.Next() {
		var id, name, data string
		err = rows.Scan(&id, &name, &data)
		if err != nil {
			return nil, fmt.Errorf("Error scanning template row: %v\n", err)
		}

		var dataKubernetesConfig []*apis.KubernetesConfig
		err = json.Unmarshal([]byte(data), &dataKubernetesConfig)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling KubernetesConfig: %v\n", err)
		}

		templates = append(templates, &apis.Template{
			ID:               id,
			Name:             name,
			KubernetesConfig: dataKubernetesConfig,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("Error iterating over template rows: %v\n", err)
	}

	logrus.Infof("[DB] Templates get successfully, total count: %d", len(templates))
	return templates, nil
}

// CreateTemplate inserts a new template into the database.
func CreateTemplate(template *apis.Template) error {
	data, err := json.Marshal(template.KubernetesConfig)
	if err != nil {
		return fmt.Errorf("Error marshalling KubernetesConfig: %v\n", err)
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("Error beginning transaction: %v\n", err)
	}

	stmt, err := tx.Prepare("INSERT INTO template(id, name, data) VALUES(?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error preparing statement: %v\n", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(template.ID, template.Name, string(data))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error executing statement: %v\n", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Error committing transaction: %v\n", err)
	}

	logrus.Infof("[DB] Template created successfully with ID: %s", template.ID)
	return nil
}

// UpdateTemplate updates an existing template in the database.
func UpdateTemplate(template *apis.Template) error {
	data, err := json.Marshal(template.KubernetesConfig)
	if err != nil {
		return fmt.Errorf("Error marshalling KubernetesConfig: %v\n", err)
	}

	_, err = DB.Exec("UPDATE template SET name = ?, data = ? WHERE id = ?", template.Name, string(data), template.ID)
	if err != nil {
		return fmt.Errorf("Error updating template: %v\n", err)
	}

	logrus.Infof("[DB] Template updated successfully with ID: %s", template.ID)
	return nil
}

// DeleteTemplate removes a template from the database by templateID.
func DeleteTemplate(templateID string) error {
	result, err := DB.Exec("DELETE FROM template WHERE id = ?", templateID)
	if err != nil {
		return fmt.Errorf("Error executing delete statement: %v\n", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Error getting rows affected: %v\n", err)
	}

	if rowsAffected == 0 {
		logrus.Infof("[DB] No template found to delete with ID: %s", templateID)
	} else {
		logrus.Infof("[DB] Template deleted successfully with ID: %s", templateID)
	}

	return nil
}
