package db

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
)

// GetNotify retrieves a notification by its ID from the database.
func GetNotify(notifyID string) (*apis.Notify, error) {
	row := DB.QueryRow("SELECT id, name, app_id, app_secret, webhook_url, secret FROM notify WHERE id = ? LIMIT 1", notifyID)

	var id, name, appID, appSecret, webhookURL, secret string
	notify := apis.NewNotify()
	err := row.Scan(&id, &name, &appID, &appSecret, &webhookURL, &secret)
	if err != nil {
		return nil, fmt.Errorf("Error scanning row: %v\n", err)
	}

	notify = &apis.Notify{
		ID:         id,
		Name:       name,
		AppID:      appID,
		AppSecret:  appSecret,
		WebhookURL: webhookURL,
		Secret:     secret,
	}

	logrus.Infof("[DB] Notify get successfully with ID: %s", notify.ID)
	return notify, nil
}

// ListNotify retrieves all notifications from the database.
func ListNotify() ([]*apis.Notify, error) {
	rows, err := DB.Query("SELECT id, name, app_id, app_secret, webhook_url, secret FROM notify")
	if err != nil {
		return nil, fmt.Errorf("Error querying database: %v\n", err)
	}
	defer rows.Close()

	notifys := apis.NewNotifys()
	for rows.Next() {
		var id, name, appID, appSecret, webhookURL, secret string
		err = rows.Scan(&id, &name, &appID, &appSecret, &webhookURL, &secret)
		if err != nil {
			return nil, fmt.Errorf("Error scanning row: %v\n", err)
		}

		notifys = append(notifys, &apis.Notify{
			ID:         id,
			Name:       name,
			AppID:      appID,
			WebhookURL: webhookURL,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("Error iterating over rows: %v\n", err)
	}

	logrus.Infof("[DB] Notifys get successfully, total count: %d", len(notifys))
	return notifys, nil
}

// CreateNotify inserts a new notification into the database.
func CreateNotify(notify *apis.Notify) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("Error starting transaction: %v\n", err)
	}

	stmt, err := tx.Prepare("INSERT INTO notify(id, name, app_id, app_secret, webhook_url, secret) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error preparing statement: %v\n", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(notify.ID, notify.Name, notify.AppID, notify.AppSecret, notify.WebhookURL, notify.Secret)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error executing statement: %v\n", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Error committing transaction: %v\n", err)
	}

	logrus.Infof("[DB] Notify created successfully with ID: %s", notify.ID)
	return nil
}

// UpdateNotify updates an existing notification in the database.
func UpdateNotify(notify *apis.Notify) error {
	_, err := DB.Exec("UPDATE notify SET name = ?, app_id = ?, app_secret = ?, webhook_url = ?, secret = ? WHERE id = ?", notify.Name, notify.AppID, notify.AppSecret, notify.WebhookURL, notify.Secret, notify.ID)
	if err != nil {
		return fmt.Errorf("Error updating notification with ID %s: %v\n", notify.ID, err)
	}

	logrus.Infof("[DB] Notify updated successfully with ID: %s", notify.ID)
	return nil
}

// DeleteNotify removes a notification from the database by its ID.
func DeleteNotify(ID string) error {
	result, err := DB.Exec("DELETE FROM notify WHERE id = ?", ID)
	if err != nil {
		return fmt.Errorf("Error deleting notification with ID %s: %v\n", ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Error getting rows affected: %v\n", err)
	}

	if rowsAffected == 0 {
		logrus.Infof("[DB] No notify found to delete with ID: %s", ID)
	} else {
		logrus.Infof("[DB] Notify deleted successfully with ID: %s", ID)
	}

	return nil
}
