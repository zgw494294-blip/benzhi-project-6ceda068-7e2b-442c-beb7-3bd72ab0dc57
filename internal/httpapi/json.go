package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maximumRequestBody = 1 << 20

type errorEnvelope struct {
	Error struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		CorrelationID string `json:"correlationId,omitempty"`
		Details       any    `json:"details,omitempty"`
	} `json:"error"`
}

func decodeJSON(w http.ResponseWriter, request *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &requestError{status: http.StatusUnsupportedMediaType, code: "CONTENT_TYPE_REQUIRED", message: "请求 Content-Type 必须为 application/json"}
	}
	request.Body = http.MaxBytesReader(w, request.Body, maximumRequestBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return &requestError{status: http.StatusRequestEntityTooLarge, code: "REQUEST_TOO_LARGE", message: "请求体超过 1 MiB 限制"}
		}
		return &requestError{status: http.StatusBadRequest, code: "INVALID_JSON", message: "JSON 请求体无效：" + err.Error()}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return &requestError{status: http.StatusBadRequest, code: "TRAILING_JSON", message: "请求体只能包含一个 JSON 对象"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, request *http.Request, err error) {
	status, code, message, details := mapError(err)
	envelope := errorEnvelope{}
	envelope.Error.Code = code
	envelope.Error.Message = message
	envelope.Error.Details = details
	envelope.Error.CorrelationID = correlationID(request)
	writeJSON(w, status, envelope)
}

func pathID(request *http.Request, name string) (string, error) {
	value := strings.TrimSpace(request.PathValue(name))
	if value == "" || len(value) > 200 || strings.ContainsAny(value, "/\\") {
		return "", &requestError{status: http.StatusBadRequest, code: "INVALID_PATH_ID", message: "路径标识无效"}
	}
	return value, nil
}
