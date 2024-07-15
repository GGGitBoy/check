package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	"inspection-server/pkg/schedule"
	"io"
	"net/http"
	"path/filepath"
)

func GetTask() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		taskID := vars["id"]
		logrus.Infof("[API] Received request to get task with ID: %s", taskID)

		task, err := db.GetTask(taskID)
		if err != nil {
			logrus.Errorf("Failed to get task with ID %s: %v", taskID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(task, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal task data for task ID %s: %v", taskID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		_, err = rw.Write(jsonData)
		if err != nil {
			logrus.Errorf("Failed to write response for task ID %s: %v", taskID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		} else {
			logrus.Infof("[API] Successfully retrieved and sent task with ID: %s", taskID)
		}
	})
}

func ListTask() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to list all tasks")

		tasks, err := db.ListTask()
		if err != nil {
			logrus.Errorf("Failed to list tasks: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(tasks, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal task list data: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		_, err = rw.Write(jsonData)
		if err != nil {
			logrus.Errorf("Failed to write response for task list: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		} else {
			logrus.Infof("[API] Successfully retrieved and sent task list")
		}
	})
}

func CreateTask() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to create a new task")

		task := apis.NewTask()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, task)
		if err != nil {
			logrus.Errorf("Failed to unmarshal task data: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		tasks, err := db.ListTask()
		if err != nil {
			logrus.Errorf("Failed to list tasks: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, t := range tasks {
			if task.Name == t.Name {
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该名称已存在"))
				return
			}
		}

		task.ID = common.GetUUID()
		task.State = "计划中"
		if task.TemplateID == "" {
			logrus.Warnf("Task creation failed: No template associated with the task")
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该计划没有对应的模版"))
			return
		}

		if task.Cron != "" {
			_, err := cron.ParseStandard(task.Cron)
			if err != nil {
				logrus.Errorf("Invalid cron expression: %v\n", err)
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Invalid cron expression: %v\n", err))
				return
			}
		}

		err = db.CreateTask(task)
		if err != nil {
			logrus.Errorf("Failed to create task with ID %s: %v", task.ID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = schedule.AddSchedule(task)
		if err != nil {
			logrus.Errorf("Failed to schedule task with ID %s: %v", task.ID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		rw.Write([]byte("创建完成"))
		logrus.Infof("[API] Successfully created and scheduled task with ID: %s", task.ID)
	})
}

func DeleteTask() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		taskID := vars["id"]
		logrus.Infof("[API] Received request to delete task with ID: %s", taskID)

		task, err := db.GetTask(taskID)
		if err != nil {
			logrus.Errorf("Failed to get task with ID %s: %v", taskID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if task.Mode == "周期任务" && task.Cron != "" && task.TaskID == "" {
			tasks, err := db.ListTask()
			if err != nil {
				logrus.Errorf("Failed to list tasks: %v", err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}

			for _, t := range tasks {
				if task.ID == t.TaskID {
					err = schedule.RemoveSchedule(t)
					if err != nil {
						logrus.Errorf("Failed to remove schedule for task with ID %s: %v", t.ID, err)
						continue
					}

					err = db.DeleteTask(t.ID)
					if err != nil {
						logrus.Errorf("Failed to delete task with ID %s: %v", t.ID, err)
						continue
					}

					report, err := db.GetReport(t.ReportID)
					if err != nil {
						logrus.Errorf("Failed to get report with ID %s: %v", t.ReportID, err)
						continue
					}

					filePath := filepath.Join(common.PrintPDFPath, common.GetReportFileName(report.Global.ReportTime))
					err = common.DeleteFile(filePath)
					if err != nil {
						logrus.Errorf("Failed to delete report pdf file %s: %v", t.ReportID, err)
						continue
					}

					err = db.DeleteReport(t.ReportID)
					if err != nil {
						logrus.Errorf("Failed to delete report for task ID %s: %v", t.ID, err)
						continue
					}
				}
			}

			err = schedule.RemoveSchedule(task)
			if err != nil {
				logrus.Errorf("Failed to remove schedule for task with ID %s: %v", taskID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}

			err = db.DeleteTask(task.ID)
			if err != nil {
				logrus.Errorf("Failed to delete task with ID %s: %v", taskID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		} else if task.Mode == "计划任务" || (task.Mode == "周期任务" && task.TaskID != "") {
			if task.State == "巡检中" {
				logrus.Warnf("Task deletion failed: Task with ID %s is currently in progress", taskID)
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("巡检中的计划不能删除"))
				return
			}

			err = schedule.RemoveSchedule(task)
			if err != nil {
				logrus.Errorf("Failed to remove schedule for task with ID %s: %v", taskID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}

			err = db.DeleteTask(task.ID)
			if err != nil {
				logrus.Errorf("Failed to delete task with ID %s: %v", taskID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}

			report, err := db.GetReport(task.ReportID)
			if err != nil {
				logrus.Errorf("Failed to get report with ID %s: %v", task.ReportID, err)
				return
			}

			filePath := filepath.Join(common.PrintPDFPath, common.GetReportFileName(report.Global.ReportTime))
			err = common.DeleteFile(filePath)
			if err != nil {
				logrus.Errorf("Failed to delete report pdf file %s: %v", task.ReportID, err)
				return
			}

			err = db.DeleteReport(task.ReportID)
			if err != nil {
				logrus.Errorf("Failed to delete report for task ID %s: %v", taskID, err)
				return
			}
		}

		rw.Write([]byte("删除完成"))
		logrus.Infof("[API] Successfully deleted task with ID: %s", taskID)
	})
}
