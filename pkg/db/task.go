package db

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
)

// GetTask retrieves a task from the database by ID.
func GetTask(ID string) (*apis.Task, error) {
	row := DB.QueryRow("SELECT id, name, start_time, end_time, cron, state, rating, report_id, template_id, notify_id, task_id, mode, err_message FROM task WHERE id = ? LIMIT 1", ID)

	task := apis.NewTask()
	err := row.Scan(&task.ID, &task.Name, &task.StartTime, &task.EndTime, &task.Cron, &task.State, &task.Rating, &task.ReportID, &task.TemplateID, &task.NotifyID, &task.TaskID, &task.Mode, &task.ErrMessage)
	if err != nil {
		return nil, fmt.Errorf("Error scanning task row: %v\n", err)
	}

	logrus.Infof("[DB] Task get successfully with ID: %s", task.ID)
	return task, nil
}

// ListTask retrieves all tasks from the database.
func ListTask() ([]*apis.Task, error) {
	rows, err := DB.Query("SELECT id, name, start_time, end_time, cron, state, rating, report_id, template_id, notify_id, task_id, mode, err_message FROM task")
	if err != nil {
		return nil, fmt.Errorf("Error executing query: %v\n", err)
	}
	defer rows.Close()

	tasks := apis.NewTasks()
	for rows.Next() {
		var task apis.Task
		err = rows.Scan(&task.ID, &task.Name, &task.StartTime, &task.EndTime, &task.Cron, &task.State, &task.Rating, &task.ReportID, &task.TemplateID, &task.NotifyID, &task.TaskID, &task.Mode, &task.ErrMessage)
		if err != nil {
			return nil, fmt.Errorf("Error scanning task row: %v\n", err)
		}

		tasks = append(tasks, &task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("Error iterating over task rows: %v\n", err)
	}

	logrus.Infof("[DB] Tasks get successfully, total count: %d", len(tasks))
	return tasks, nil
}

// CreateTask inserts a new task into the database.
func CreateTask(task *apis.Task) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("Error beginning transaction: %v\n", err)
	}

	stmt, err := tx.Prepare("INSERT INTO task(id, name, start_time, end_time, cron, state, rating, report_id, template_id, notify_id, task_id, mode, err_message) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error preparing statement: %v\n", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(task.ID, task.Name, task.StartTime, task.EndTime, task.Cron, task.State, task.Rating, task.ReportID, task.TemplateID, task.NotifyID, task.TaskID, task.Mode, task.ErrMessage)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Error executing statement: %v\n", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Error committing transaction: %v\n", err)
	}

	logrus.Infof("[DB] Task created successfully with ID: %s", task.ID)
	return nil
}

// UpdateTask updates an existing task in the database.
func UpdateTask(task *apis.Task) error {
	_, err := DB.Exec("UPDATE task SET name = ?, start_time = ?, end_time = ?, cron = ?, state = ?, rating = ?, report_id = ?, template_id = ?, notify_id = ?, task_id = ?, mode = ?, err_message = ? WHERE id = ?", task.Name, task.StartTime, task.EndTime, task.Cron, task.State, task.Rating, task.ReportID, task.TemplateID, task.NotifyID, task.TaskID, task.Mode, task.ErrMessage, task.ID)
	if err != nil {
		return fmt.Errorf("Error updating task: %v\n", err)
	}

	logrus.Infof("[DB] Task updated successfully with ID: %s", task.ID)
	return nil
}

// DeleteTask removes a task from the database by ID.
func DeleteTask(ID string) error {
	result, err := DB.Exec("DELETE FROM task WHERE id = ?", ID)
	if err != nil {
		return fmt.Errorf("Error executing delete statement: %v\n", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Error getting rows affected: %v\n", err)
	}

	if rowsAffected == 0 {
		logrus.Infof("[DB] No task found to delete with ID: %s", ID)
	} else {
		logrus.Infof("[DB] Task deleted successfully with ID: %s", ID)
	}

	return nil
}
