package httpapi

import (
	"net/http"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

func (a *API) ProposeResolutionHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	findingID, err := pathID(request, "findingId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.ProposeResolutionCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	if command.FindingID != "" && command.FindingID != findingID {
		writeError(w, request, &requestError{status: http.StatusBadRequest, code: "FINDING_ID_MISMATCH", message: "路径与请求体的 findingId 不一致"})
		return
	}
	command.FindingID = findingID
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.ProposeResolution(request.Context(), taskID, command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ReviewResolutionHandler(w http.ResponseWriter, request *http.Request) {
	taskID, err := pathID(request, "taskId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	findingID, err := pathID(request, "findingId")
	if err != nil {
		writeError(w, request, err)
		return
	}
	var command archive.ReviewResolutionCommand
	if err := decodeJSON(w, request, &command); err != nil {
		writeError(w, request, err)
		return
	}
	if command.FindingID != "" && command.FindingID != findingID {
		writeError(w, request, &requestError{status: http.StatusBadRequest, code: "FINDING_ID_MISMATCH", message: "路径与请求体的 findingId 不一致"})
		return
	}
	command.FindingID = findingID
	defaultCorrelation(request, &command.CommandMeta)
	result, err := a.service.ReviewResolution(request.Context(), taskID, command)
	if err != nil {
		writeError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
