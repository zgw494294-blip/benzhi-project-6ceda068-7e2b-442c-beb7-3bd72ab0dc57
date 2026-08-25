package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type selfCheckTaskResponse struct {
	Task struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
		State   string `json:"state"`
	} `json:"task"`
}

func RunSelfCheck(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	if err := waitReady(ctx, client, baseURL+"/healthz"); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	create := map[string]any{
		"idempotencyKey": "selfcheck-create", "actor": "selfcheck-admin", "role": "DATA_ADMIN",
		"expectedVersion": 0,
		"reason":          "自检创建任务", "correlationId": "selfcheck-flow", "title": "自检观测任务",
		"instrumentCode": "SC-CCD", "observationStart": now.Add(-2 * time.Hour),
		"observationEnd": now.Add(-time.Hour), "owner": "selfcheck-owner",
	}
	var task selfCheckTaskResponse
	if err := selfCheckJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/archive-tasks", create, &task); err != nil {
		return fmt.Errorf("创建任务失败：%w", err)
	}
	taskID := task.Task.ID
	revision := commandBody("selfcheck-revision", task.Task.Version, "selfcheck-admin", "DATA_ADMIN", "登记自检数据")
	revision["logicalPath"] = "science/selfcheck.fits"
	revision["byteSize"] = 5760
	revision["mediaType"] = "application/fits"
	revision["sha256"] = strings.Repeat("a", 64)
	if err := selfCheckJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/archive-tasks/"+taskID+"/revisions", revision, &task); err != nil {
		return fmt.Errorf("登记修订失败：%w", err)
	}
	validation := commandBody("selfcheck-validation", task.Task.Version, "selfcheck-reviewer", "QUALITY_REVIEWER", "执行自检校验")
	if err := selfCheckJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/archive-tasks/"+taskID+"/validation", validation, &task); err != nil {
		return fmt.Errorf("执行校验失败：%w", err)
	}
	if task.Task.State != "REVIEW_PENDING" {
		return fmt.Errorf("校验后状态异常：%s", task.Task.State)
	}
	freeze := commandBody("selfcheck-freeze", task.Task.Version, "selfcheck-release", "RELEASE_LEAD", "冻结自检清单")
	if err := selfCheckJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/archive-tasks/"+taskID+"/freeze", freeze, &task); err != nil {
		return fmt.Errorf("冻结清单失败：%w", err)
	}
	issue := commandBody("selfcheck-issue", task.Task.Version, "selfcheck-release", "RELEASE_LEAD", "签发自检凭据")
	issue["purposeScope"] = "用于服务启动自检验证"
	if err := selfCheckJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/archive-tasks/"+taskID+"/release-credentials", issue, &task); err != nil {
		return fmt.Errorf("签发凭据失败：%w", err)
	}
	var verification struct {
		Valid bool `json:"valid"`
	}
	if err := selfCheckJSON(ctx, client, http.MethodGet, baseURL+"/api/v1/archive-tasks/"+taskID+"/release-verification", nil, &verification); err != nil {
		return fmt.Errorf("验证凭据失败：%w", err)
	}
	if !verification.Valid {
		return fmt.Errorf("释放凭据验证结果无效")
	}
	return nil
}

func commandBody(key string, version int64, actor, role, reason string) map[string]any {
	return map[string]any{
		"idempotencyKey": key, "expectedVersion": version, "actor": actor,
		"role": role, "reason": reason, "correlationId": "selfcheck-flow",
	}
}

func waitReady(ctx context.Context, client *http.Client, url string) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		response, err := client.Do(request)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待服务就绪超时：%w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func selfCheckJSON(ctx context.Context, client *http.Client, method, url string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Correlation-ID", "selfcheck-flow")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(encoded)))
	}
	if target != nil && len(encoded) > 0 {
		if err := json.Unmarshal(encoded, target); err != nil {
			return fmt.Errorf("响应 JSON 无效：%w", err)
		}
	}
	return nil
}
