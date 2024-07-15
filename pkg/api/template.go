package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/agent"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	"inspection-server/pkg/template"
	"io"
	"net/http"
)

func GetTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		templateID := vars["id"]
		logrus.Infof("[API] Received request to get template with ID: %s", templateID)

		template, err := db.GetTemplate(templateID)
		if err != nil {
			logrus.Errorf("Failed to get template with ID %s: %v", templateID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(template, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal template data for ID %s: %v", templateID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		_, err = rw.Write(jsonData)
		if err != nil {
			logrus.Errorf("Failed to write response for template ID %s: %v", templateID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		} else {
			logrus.Infof("[API] Successfully retrieved and sent template with ID: %s", templateID)
		}
	})
}

func ListTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to list all templates")

		templates, err := db.ListTemplate()
		if err != nil {
			logrus.Errorf("Failed to list templates: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		jsonData, err := json.MarshalIndent(templates, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal template list data: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		_, err = rw.Write(jsonData)
		if err != nil {
			logrus.Errorf("Failed to write response for template list: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		} else {
			logrus.Infof("[API] Successfully retrieved and sent template list")
		}
	})
}

func CreateTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to create a new template")

		template := apis.NewTemplate()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, template)
		if err != nil {
			logrus.Errorf("Failed to unmarshal template data: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		templates, err := db.ListTemplate()
		if err != nil {
			logrus.Errorf("Failed to list templates: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, t := range templates {
			if template.Name == t.Name {
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该名称已存在"))
				return
			}
		}

		template.ID = common.GetUUID()
		err = db.CreateTemplate(template)
		if err != nil {
			logrus.Errorf("Failed to create template with ID %s: %v", template.ID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		rw.Write([]byte("创建完成"))
		logrus.Infof("[API] Successfully created template with ID: %s", template.ID)
	})
}

func UpdateTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to update template")

		template := apis.NewTemplate()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = json.Unmarshal(body, template)
		if err != nil {
			logrus.Errorf("Failed to unmarshal template data: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if template.ID == "Default" {
			logrus.Errorf("Failed to update template %s", template.ID)
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Failed to update template %s\n", template.ID))
			return
		}

		templates, err := db.ListTemplate()
		if err != nil {
			logrus.Errorf("Failed to list templates: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, t := range templates {
			if template.Name == t.Name && template.ID != t.ID {
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该名称已存在"))
				return
			}
		}

		err = db.UpdateTemplate(template)
		if err != nil {
			logrus.Errorf("Failed to update template with ID %s: %v", template.ID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		rw.Write([]byte("更新完成"))
		logrus.Infof("[API] Successfully updated template with ID: %s", template.ID)
	})
}

func DeleteTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		templateID := vars["id"]
		logrus.Infof("[API] Received request to delete template with ID: %s", templateID)

		if templateID == "Default" {
			logrus.Errorf("Failed to delete template %s", templateID)
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Failed to delete template %s\n", templateID))
			return
		}

		tasks, err := db.ListTask()
		if err != nil {
			logrus.Errorf("Failed to list tasks for template deletion: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, t := range tasks {
			if t.TemplateID == templateID {
				logrus.Warnf("Template deletion failed: Template with ID %s is used in task %s", templateID, t.Name)
				common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("该通知在被巡检任务 %s 使用无法删除", t.Name))
				return
			}
		}

		err = db.DeleteTemplate(templateID)
		if err != nil {
			logrus.Errorf("Failed to delete template with ID %s: %v", templateID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		rw.Write([]byte("删除完成"))
		logrus.Infof("[API] Successfully deleted template with ID: %s", templateID)
	})
}

func RefreshDefaultTemplate() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to refresh template with Default")

		err := template.Register()
		if err != nil {
			logrus.Errorf("Failed to register template: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Failed to register template: %v\n", err))
			return
		}

		err = agent.Register()
		if err != nil {
			logrus.Errorf("Failed to register agent: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Failed to register agent: %v\n", err))
			return
		}

		rw.Write([]byte("更新完成"))
	})
}
