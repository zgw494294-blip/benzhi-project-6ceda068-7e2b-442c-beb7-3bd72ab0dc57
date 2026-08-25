package httpapi

import (
	"net/http"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

type API struct {
	service   *archive.Service
	startedAt time.Time
	mux       *http.ServeMux
}

func New(service *archive.Service) *API {
	api := &API{service: service, startedAt: time.Now().UTC(), mux: http.NewServeMux()}
	api.routes()
	return api
}

func (a *API) Handler() http.Handler {
	return a.recoverPanic(a.requestMetadata(a.mux))
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.HealthHandler)
	a.mux.HandleFunc("GET /api/v1/archive-tasks", a.ListTasksHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks", a.CreateTaskHandler)
	a.mux.HandleFunc("GET /api/v1/archive-tasks/{taskId}", a.GetTaskHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/revisions", a.RegisterRevisionHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/validation", a.ValidateTaskHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/findings/{findingId}/resolution", a.ProposeResolutionHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/findings/{findingId}/review", a.ReviewResolutionHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/freeze", a.FreezeManifestHandler)
	a.mux.HandleFunc("POST /api/v1/archive-tasks/{taskId}/release-credentials", a.IssueCredentialHandler)
	a.mux.HandleFunc("GET /api/v1/archive-tasks/{taskId}/timeline", a.TimelineHandler)
	a.mux.HandleFunc("GET /api/v1/archive-tasks/{taskId}/release-verification", a.VerifyReleaseHandler)
}
