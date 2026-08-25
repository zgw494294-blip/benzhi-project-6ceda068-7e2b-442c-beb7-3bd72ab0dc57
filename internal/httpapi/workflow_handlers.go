package httpapi

import (
	"net/http"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

func (a *API) RegisterRevisionHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.RegisterRevisionCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.RegisterRevision(request.Context(), taskID, command)
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

func (a *API) ValidateTaskHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.ValidateCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.Validate(request.Context(), taskID, command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func defaultCorrelation(request *http.Request, meta *archive.CommandMeta) {
	if meta.CorrelationID == "" {
		meta.CorrelationID = correlationID(request)
	}
}
