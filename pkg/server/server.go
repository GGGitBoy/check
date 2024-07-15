package server

import (
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/api"
	"inspection-server/pkg/core"
)

func Start() http.Handler {
	router := mux.NewRouter()
	router.UseEncodedPath()

	debugHandle(router)

	router.Methods(http.MethodGet).Path("/v1/reports/get/{id}").Handler(logMiddleware(api.GetReport()))
	router.Methods(http.MethodGet).Path("/v1/reports/print/{id}").Handler(logMiddleware(api.PrintReport()))

	router.Methods(http.MethodGet).Path("/v1/tasks/get/{id}").Handler(logMiddleware(api.GetTask()))
	router.Methods(http.MethodGet).Path("/v1/tasks/list").Handler(logMiddleware(api.ListTask()))
	router.Methods(http.MethodPost).Path("/v1/tasks/create").Handler(logMiddleware(api.CreateTask()))
	router.Methods(http.MethodDelete).Path("/v1/tasks/delete/{id}").Handler(logMiddleware(api.DeleteTask()))

	router.Methods(http.MethodGet).Path("/v1/templates/get/{id}").Handler(logMiddleware(api.GetTemplate()))
	router.Methods(http.MethodGet).Path("/v1/templates/list").Handler(logMiddleware(api.ListTemplate()))
	router.Methods(http.MethodPost).Path("/v1/templates/create").Handler(logMiddleware(api.CreateTemplate()))
	router.Methods(http.MethodPut).Path("/v1/templates/update").Handler(logMiddleware(api.UpdateTemplate()))
	router.Methods(http.MethodDelete).Path("/v1/templates/delete/{id}").Handler(logMiddleware(api.DeleteTemplate()))
	router.Methods(http.MethodGet).Path("/v1/templates/refresh/default").Handler(logMiddleware(api.RefreshDefaultTemplate()))

	router.Methods(http.MethodGet).Path("/v1/notify/get/{id}").Handler(logMiddleware(api.GetNotify()))
	router.Methods(http.MethodGet).Path("/v1/notify/list").Handler(logMiddleware(api.ListNotify()))
	router.Methods(http.MethodPost).Path("/v1/notify/create").Handler(logMiddleware(api.CreateNotify()))
	router.Methods(http.MethodPut).Path("/v1/notify/update").Handler(logMiddleware(api.UpdateNotify()))
	router.Methods(http.MethodDelete).Path("/v1/notify/delete/{id}").Handler(logMiddleware(api.DeleteNotify()))
	router.Methods(http.MethodPost).Path("/v1/notify/test").Handler(logMiddleware(api.TestNotify()))

	router.Methods(http.MethodGet).Path("/v1/clusters/list").Handler(logMiddleware(api.GetClusters()))
	router.Methods(http.MethodGet).Path("/v1/clusters/{id}/resource/list").Handler(logMiddleware(api.GetResource()))

	router.Methods(http.MethodGet).Path("/v1/agent/list").Handler(logMiddleware(api.ListAgent()))
	router.Methods(http.MethodDelete).Path("/v1/agent/delete/{id}").Handler(logMiddleware(api.DeleteAgent()))

	router.Methods(http.MethodGet).Path("/v1/grafana/get").Handler(logMiddleware(api.GetGrafanaClusterIP()))
	router.Methods(http.MethodGet).Path("/v1/grafana/alerting/get").Handler(logMiddleware(core.GetGrafanaAlerting()))

	return router
}

func debugHandle(router *mux.Router) {
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)

	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
}

// logMiddleware is a middleware function that logs each incoming HTTP request
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("Incoming request: Method=%s, URL=%s, RemoteAddr=%s", r.Method, r.URL, r.RemoteAddr)
		next.ServeHTTP(w, r)
		logrus.Debugf("Completed handling request: Method=%s, URL=%s, RemoteAddr=%s", r.Method, r.URL, r.RemoteAddr)
	})
}
