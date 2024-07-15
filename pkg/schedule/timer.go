package schedule

import (
	"fmt"
	"inspection-server/pkg/apis"
	"time"

	"github.com/sirupsen/logrus"
)

func AddTimeTask(task *apis.Task) error {
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", task.StartTime, GetLoc())
	if err != nil {
		logrus.Errorf("Error parsing start time for task %s: %v", task.ID, err)
		return fmt.Errorf("error parsing start time for schedule %s: %v", task.ID, err)
	}

	duration := time.Until(startTime)
	logrus.Infof("[Schedule] Task %s will execute after %.2f minutes", task.ID, duration.Minutes())
	if duration <= 0 {
		go ExecuteTask(task)
		logrus.Warnf("Task %s is scheduled with a past time. Executing immediately.", task.ID)
		return nil
	}

	TaskMutex.Lock()
	defer TaskMutex.Unlock()

	timer := time.AfterFunc(duration, func() {
		logrus.Infof("[Schedule] Executing task %s", task.ID)
		go ExecuteTask(task)
	})

	TaskMap[task.ID] = &Schedule{
		Timer: timer,
	}

	logrus.Infof("[Schedule] Scheduled task %s to execute at %s", task.ID, task.StartTime)
	return nil
}

func RemoveTimetask(taskID string) error {
	TaskMutex.Lock()
	defer TaskMutex.Unlock()

	if s, exists := TaskMap[taskID]; exists {
		if !s.Timer.Stop() {
			logrus.Warnf("Timer for task %s has already executed or stopped", taskID)
		}
		delete(TaskMap, taskID)
		logrus.Infof("[Schedule] Deleted scheduled task %s", taskID)
	} else {
		logrus.Warnf("[Schedule] No scheduled task found with ID %s", taskID)
	}

	return nil
}
