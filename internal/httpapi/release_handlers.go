package httpapi

import (
	"net/http"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

func (a *API) FreezeManifestHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.FreezeCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.Freeze(request.Context(), taskID, command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) IssueCredentialHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.IssueCredentialCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.IssueCredential(request.Context(), taskID, command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) TimelineHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	result, err := a.service.Timeline(request.Context(), taskID)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) VerifyReleaseHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	result, err := a.service.Verify(request.Context(), taskID)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
