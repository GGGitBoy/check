package schedule

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	"time"
)

func AddCornTask(task *apis.Task) error {
	TaskMutex.Lock()
	defer TaskMutex.Unlock()

	entryID, err := CronClient.AddFunc(task.Cron, func() {
		now := time.Now().Format("2006-01-02 15:04:05")
		newTask := &apis.Task{
			ID:         common.GetUUID(),
			Name:       fmt.Sprintf("%s(%s)", task.Name, now),
			StartTime:  now,
			EndTime:    "",
			Cron:       task.Cron,
			State:      "巡检中",
			Rating:     "",
			ReportID:   "",
			TemplateID: task.TemplateID,
			NotifyID:   task.NotifyID,
			TaskID:     task.ID,
			Mode:       task.Mode,
		}

		err := db.CreateTask(newTask)
		if err != nil {
			logrus.Errorf("Failed to create task in DB for scheduled schedule %s: %v", newTask.ID, err)
			return
		}

		logrus.Infof("[Schedule] Executing task: %s", newTask.ID)
		go ExecuteTask(newTask)
		logrus.Infof("[Schedule] Task %s is executing", newTask.ID)
	})
	if err != nil {
		return fmt.Errorf("Error adding cron job: %v\n", err)
	}

	TaskMap[task.ID] = &Schedule{
		Cron: entryID,
	}

	logrus.Infof("[Schedule] Scheduled task %s to execute at %s", task.ID, task.Cron)

	return nil
}

func RemoveCorntask(taskID string) error {
	TaskMutex.Lock()
	defer TaskMutex.Unlock()

	if s, exists := TaskMap[taskID]; exists {
		CronClient.Remove(s.Cron)
		delete(TaskMap, taskID)
		logrus.Infof("[Schedule] Deleted scheduled task with ID %s", taskID)
	} else {
		logrus.Warnf("[Schedule] No scheduled task found with ID %s", taskID)
	}

	logrus.Infof("[Schedule] Removed task with ID: %s", taskID)

	return nil
}
