package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	"inspection-server/pkg/send"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

func GetNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		notifyID := vars["id"]

		notify, err := db.GetNotify(notifyID)
		if err != nil {
			logrus.Errorf("Failed to get notify with ID %s: %v", notifyID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		notify.AppSecret = ""
		notify.Secret = ""
		jsonData, err := json.MarshalIndent(notify, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal notify with ID %s: %v", notifyID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write response for notify with ID %s: %v", notifyID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func ListNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		notifys, err := db.ListNotify()
		if err != nil {
			logrus.Errorf("Failed to list notifies: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(notifys, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal notifies: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write response for notifies: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func CreateNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		notify := apis.NewNotify()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, notify)
		if err != nil {
			logrus.Errorf("Failed to unmarshal request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		notifys, err := db.ListNotify()
		if err != nil {
			logrus.Errorf("Failed to list notifies: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, n := range notifys {
			if notify.Name == n.Name {
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该名称已存在"))
				return
			}
		}

		notify.ID = common.GetUUID()
		err = db.CreateNotify(notify)
		if err != nil {
			logrus.Errorf("Failed to create notify: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write([]byte("创建完成")); err != nil {
			logrus.Errorf("Failed to write creation response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}
	})
}

func UpdateNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		notify := apis.NewNotify()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, notify)
		if err != nil {
			logrus.Errorf("Failed to unmarshal request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		notifys, err := db.ListNotify()
		if err != nil {
			logrus.Errorf("Failed to list notifies: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, n := range notifys {
			if notify.Name == n.Name && notify.ID != n.ID {
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该名称已存在"))
				return
			}
		}

		err = db.UpdateNotify(notify)
		if err != nil {
			logrus.Errorf("Failed to update notify: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write([]byte("更新完成")); err != nil {
			logrus.Errorf("Failed to write update response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func DeleteNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		notifyID := vars["id"]

		tasks, err := db.ListTask()
		if err != nil {
			logrus.Errorf("Failed to list tasks: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, t := range tasks {
			if t.NotifyID == notifyID {
				logrus.Errorf("该通知在被巡检任务 %s 使用无法删除", t.Name)
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该通知在被巡检任务 %s 使用无法删除", t.Name))
				return
			}
		}

		err = db.DeleteNotify(notifyID)
		if err != nil {
			logrus.Errorf("Failed to delete notify with ID %s: %v", notifyID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write([]byte("删除完成")); err != nil {
			logrus.Errorf("Failed to write deletion response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func TestNotify() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		notify := apis.NewNotify()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, notify)
		if err != nil {
			logrus.Errorf("Failed to unmarshal request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		message := "测试成功"

		if notify.WebhookURL != "" && notify.Secret != "" {
			err = send.Webhook(notify.WebhookURL, notify.Secret, message, "Test")
			if err != nil {
				logrus.Errorf("Failed to send test notification: %v", err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		} else {
			err = send.Notify(notify.AppID, notify.AppSecret, common.SendTestPDFName, common.SendTestPDFPath, message, "Test")
			if err != nil {
				logrus.Errorf("Failed to send test notification: %v", err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		}

		if _, err := rw.Write([]byte("测试成功")); err != nil {
			logrus.Errorf("Failed to write test response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}
