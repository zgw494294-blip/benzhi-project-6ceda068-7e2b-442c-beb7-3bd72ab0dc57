package httpapi

import (
	"net/http"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

func (a *API) HealthHandler(w http.ResponseWriter, request *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "service": "astronomy-release-governance", "startedAt": a.startedAt,
	})
}

func (a *API) CreateTaskHandler(w http.ResponseWriter, request *http.Request) {
	var command archive.CreateTaskCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	if command.CorrelationID == "" {
		command.CorrelationID = correlationID(request)
	}
	result, err := a.service.CreateTask(request.Context(), command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	status := http.StatusCreated
	if result.Replay {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (a *API) ListTasksHandler(w http.ResponseWriter, request *http.Request) {
	items, err := a.service.ListTasks(request.Context())
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *API) GetTaskHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	detail, err := a.service.GetTask(request.Context(), taskID)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}
