package launchcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/automation"
)

const automationPublicHookRoutePrefix = "/api/automation/hooks/"

func (s *LaunchServer) handleAutomationPublicHook(w http.ResponseWriter, r *http.Request) {
	hookPath, ok := parseAutomationPublicHookPath(r.URL.Path)
	if !ok {
		writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
		return
	}

	record, err := s.findAutomationPublicHookScript(hookPath)
	if err != nil {
		if err == errAutomationHookServiceUnavailable {
			writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
			return
		}
		if os.IsNotExist(err) {
			writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
			return
		}
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	if !record.PublicAPI.Enabled {
		writeAutomationAPIError(w, http.StatusNotFound, "not_found", "hook not found", "")
		return
	}

	if r.Method != record.PublicAPI.Method {
		writeAutomationAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
		return
	}

	runner, ok := s.starter.(AutomationScriptRunner)
	if !ok {
		writeAutomationAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "automation script api is unavailable", "")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body", "")
		return
	}

	input, err := buildAutomationPublicHookRunRequest(*record, r, body)
	if err != nil {
		writeAutomationAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), automationRequestErrorField(err))
		return
	}

	run, err := runner.AutomationScriptRunWithOptions(input)
	if err != nil {
		writeAutomationAPIError(w, http.StatusInternalServerError, "internal_error", err.Error(), "")
		return
	}

	writeAutomationPublicHookResponse(w, *record, run)
}

var errAutomationHookServiceUnavailable = automationRequestError("automation hook service unavailable")

func (s *LaunchServer) findAutomationPublicHookScript(hookPath string) (*automation.ScriptRecord, error) {
	lister, ok := s.starter.(AutomationScriptLister)
	if !ok {
		return nil, errAutomationHookServiceUnavailable
	}

	items, err := lister.AutomationScriptList()
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if normalizeAutomationPublicHookPath(item.PublicAPI.Path) != hookPath {
			continue
		}
		record := item
		return &record, nil
	}

	return nil, os.ErrNotExist
}

func parseAutomationPublicHookPath(urlPath string) (string, bool) {
	trimmed := strings.TrimSpace(urlPath)
	if !strings.HasPrefix(trimmed, automationPublicHookRoutePrefix) {
		return "", false
	}

	trimmed = strings.TrimPrefix(trimmed, automationPublicHookRoutePrefix)
	trimmed = normalizeAutomationPublicHookPath(trimmed)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func normalizeAutomationPublicHookPath(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}

	cleaned := strings.Trim(path.Clean("/"+value), "/")
	if cleaned == "" || cleaned == "." {
		return ""
	}
	return strings.ToLower(cleaned)
}

func buildAutomationPublicHookRunRequest(record automation.ScriptRecord, r *http.Request, body []byte) (automation.ScriptRunRequest, error) {
	if shouldApplyAutomationPublicHookVariables(record) {
		resolvedBody, err := resolveAutomationPublicHookRequestBody(record, body)
		if err != nil {
			return automation.ScriptRunRequest{}, err
		}
		body = resolvedBody
	}

	input, err := decodeAutomationPublicHookRequestBody(body)
	if err != nil {
		return automation.ScriptRunRequest{}, err
	}
	selectorText := ""
	useScriptSelector := true
	if strings.TrimSpace(input.Code) != "" {
		encodedSelectorText, err := encodeAutomationPublicHookJSONObject(map[string]interface{}{"code": strings.TrimSpace(input.Code)})
		if err != nil {
			return automation.ScriptRunRequest{}, badAutomationRequest("code is invalid")
		}
		selectorText = encodedSelectorText
		useScriptSelector = false
	}
	if err := validateAutomationTimeoutMs(input.TimeoutMs); err != nil {
		return automation.ScriptRunRequest{}, err
	}
	params := mergeAutomationPublicHookDefaultParamsObject(record, input.Params)
	paramsText, err := encodeAutomationPublicHookJSONObject(params)
	if err != nil {
		return automation.ScriptRunRequest{}, badAutomationRequest("params must be a JSON object")
	}

	return automation.ScriptRunRequest{
		ScriptID:          record.ID,
		SelectorText:      selectorText,
		ParamsText:        paramsText,
		UseScriptSelector: useScriptSelector,
		UseScriptParams:   false,
		TimeoutMs:         resolveAutomationPublicHookTimeout(r, input.TimeoutMs, record.PublicAPI.TimeoutMs),
	}, nil
}

type automationPublicHookRequestBody struct {
	Code      string                 `json:"code"`
	Params    map[string]interface{} `json:"params"`
	TimeoutMs int                    `json:"timeoutMs"`
}

func decodeAutomationPublicHookRequestBody(body []byte) (automationPublicHookRequestBody, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return automationPublicHookRequestBody{}, nil
	}

	var input automationPublicHookRequestBody
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return automationPublicHookRequestBody{}, badAutomationRequest("invalid request body")
	}
	if input.Params == nil {
		input.Params = map[string]interface{}{}
	}
	return input, nil
}

func encodeAutomationPublicHookJSONObject(obj map[string]interface{}) (string, error) {
	encoded, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func shouldApplyAutomationPublicHookVariables(record automation.ScriptRecord) bool {
	if strings.TrimSpace(record.PublicAPI.RequestBodyText) == "" {
		return false
	}
	for _, variable := range record.PublicAPI.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		if strings.Contains(record.PublicAPI.RequestBodyText, "{{"+name+"}}") || strings.Contains(record.PublicAPI.RequestBodyText, "${"+name+"}") {
			return true
		}
	}
	return false
}

func replaceAutomationPublicHookPlaceholderValue(bodyText string, name string, rawValue interface{}) string {
	value := strings.TrimSpace(formatAutomationPublicHookVariableValue(rawValue))
	escapedValue := escapeAutomationPublicHookJSONString(value)
	for _, placeholder := range []string{"{{" + name + "}}", "${" + name + "}"} {
		bodyText = strings.ReplaceAll(bodyText, placeholder, escapedValue)
	}
	return bodyText
}

func resolveAutomationPublicHookRequestBody(record automation.ScriptRecord, body []byte) ([]byte, error) {
	config := record.PublicAPI
	input, err := decodeAutomationPublicHookRequestBody(body)
	if err != nil {
		return nil, err
	}
	values := input.Params

	bodyText := replaceAutomationPublicHookPlaceholderValue(config.RequestBodyText, "code", input.Code)
	for _, variable := range config.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" {
			continue
		}
		placeholders := []string{"{{" + name + "}}", "${" + name + "}"}
		used := false
		for _, placeholder := range placeholders {
			if strings.Contains(bodyText, placeholder) {
				used = true
				break
			}
		}
		if !used {
			continue
		}

		rawValue := interface{}(variable.DefaultValue)
		if incomingValue, ok := values[name]; ok {
			rawValue = incomingValue
		}
		value := strings.TrimSpace(formatAutomationPublicHookVariableValue(rawValue))
		if variable.Required && value == "" {
			return nil, badAutomationRequest("missing required variable: " + name)
		}
		escapedValue := escapeAutomationPublicHookJSONString(value)
		for _, placeholder := range placeholders {
			bodyText = strings.ReplaceAll(bodyText, placeholder, escapedValue)
		}
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(bodyText), &decoded); err != nil {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	decodedBody, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	mergedBody := mergeAutomationPublicHookDefaultParams(record, decodedBody)
	encoded, err := json.Marshal(mergedBody)
	if err != nil {
		return nil, badAutomationRequest("resolved request body must be a JSON object")
	}
	return encoded, nil
}

func mergeAutomationPublicHookDefaultParams(record automation.ScriptRecord, body map[string]interface{}) map[string]interface{} {
	defaultParams, ok := parseAutomationPublicHookJSONObject(record.ParamsText)
	if !ok || len(defaultParams) == 0 {
		return body
	}

	if record.PublicAPI.RequestMode == "params-only" {
		return mergeAutomationPublicHookJSONObjects(defaultParams, body)
	}

	rawParams, ok := body["params"].(map[string]interface{})
	if !ok {
		return body
	}

	nextBody := make(map[string]interface{}, len(body))
	for key, value := range body {
		nextBody[key] = value
	}
	nextBody["params"] = mergeAutomationPublicHookJSONObjects(defaultParams, rawParams)
	return nextBody
}

func mergeAutomationPublicHookDefaultParamsObject(record automation.ScriptRecord, param map[string]interface{}) map[string]interface{} {
	if param == nil {
		param = map[string]interface{}{}
	}
	defaultParams, ok := parseAutomationPublicHookJSONObject(record.ParamsText)
	if !ok || len(defaultParams) == 0 {
		return param
	}
	return mergeAutomationPublicHookJSONObjects(defaultParams, param)
}

func parseAutomationPublicHookJSONObject(text string) (map[string]interface{}, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false
	}
	var value interface{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]interface{})
	return object, ok
}

func mergeAutomationPublicHookJSONObjects(base map[string]interface{}, patch map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(patch))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range patch {
		baseObject, baseOK := merged[key].(map[string]interface{})
		patchObject, patchOK := value.(map[string]interface{})
		if baseOK && patchOK {
			merged[key] = mergeAutomationPublicHookJSONObjects(baseObject, patchObject)
			continue
		}
		merged[key] = value
	}
	return merged
}

func formatAutomationPublicHookVariableValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64, bool, int, int64, json.Number:
		return strings.TrimSpace(strings.Trim(fmt.Sprint(typed), "\""))
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func escapeAutomationPublicHookJSONString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	text := string(encoded)
	if len(text) >= 2 {
		return text[1 : len(text)-1]
	}
	return text
}

func decodeAutomationRunAPIRequestBody(body []byte) (automationScriptRunAPIRequest, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return automationScriptRunAPIRequest{}, nil
	}

	var req automationScriptRunAPIRequest
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return automationScriptRunAPIRequest{}, badAutomationRequest("invalid request body")
	}
	return req, nil
}

func decodeJSONObjectBody(body []byte, fieldName string) (map[string]interface{}, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}

	var value interface{}
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, badAutomationRequest(fieldName + " must be a JSON object")
	}
	return obj, true, nil
}

func resolveAutomationPublicHookTimeout(r *http.Request, requestTimeout int, fallback int) int {
	if requestTimeout > 0 {
		return requestTimeout
	}

	if r != nil {
		if raw := strings.TrimSpace(r.URL.Query().Get("timeoutMs")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				return parsed
			}
		}
	}

	return fallback
}

func writeAutomationPublicHookResponse(w http.ResponseWriter, record automation.ScriptRecord, run *automation.ScriptRunRecord) {
	_ = record
	parsedPayload, resultPayload, hasResult := decodeAutomationRunPayloadValue(run.ResultText)
	if run.Status != "success" {
		writeJSON(w, http.StatusOK, compactAutomationPublicHookFailure(run))
		return
	}

	response := map[string]interface{}{
		"ok":      true,
		"status":  run.Status,
		"summary": run.Summary,
		"message": run.Summary,
		"data":    map[string]interface{}{},
		"result":  map[string]interface{}{},
	}

	if hasResult {
		data := compactAutomationPublicHookData(resultPayload, run)
		response["data"] = data
		response["result"] = data
	} else if parsedPayload != nil {
		data := compactAutomationPublicHookData(parsedPayload, run)
		response["data"] = data
		response["result"] = data
	}

	writeJSON(w, http.StatusOK, response)
}

func compactAutomationPublicHookFailure(run *automation.ScriptRunRecord) map[string]interface{} {
	response := map[string]interface{}{
		"ok":      false,
		"status":  run.Status,
		"summary": run.Summary,
		"message": run.Summary,
		"data":    map[string]interface{}{},
		"result":  map[string]interface{}{},
	}
	if strings.TrimSpace(run.Error) != "" {
		response["error"] = run.Error
	}
	return response
}

func compactAutomationPublicHookData(payload interface{}, run *automation.ScriptRunRecord) interface{} {
	data := compactAutomationPublicHookResult(payload, run)
	delete(data, "ok")
	delete(data, "summary")
	return data
}

func compactAutomationPublicHookResult(payload interface{}, run *automation.ScriptRunRecord) map[string]interface{} {
	obj, ok := payload.(map[string]interface{})
	if !ok {
		result := map[string]interface{}{"ok": true}
		if strings.TrimSpace(run.Summary) != "" {
			result["summary"] = run.Summary
		}
		if payload != nil {
			result["result"] = payload
		}
		return result
	}

	if !hasAutomationPublicHookDownloadField(obj) {
		result := make(map[string]interface{}, len(obj)+1)
		result["ok"] = true
		for key, value := range obj {
			if key != "ok" && value != nil {
				result[key] = value
			}
		}
		if _, exists := result["summary"]; !exists && strings.TrimSpace(run.Summary) != "" {
			result["summary"] = run.Summary
		}
		return result
	}

	result := map[string]interface{}{"ok": true}

	for _, key := range []string{
		"downloadAddress",
		"downloadPath",
		"outputPath",
		"sourceImageUrl",
		"sourceDownloadUrl",
		"screenshotPath",
		"pageScreenshotPath",
		"contentType",
		"imageWidth",
		"imageHeight",
		"status",
		"summary",
		"error",
	} {
		if value, exists := obj[key]; exists && value != nil {
			result[key] = value
		}
	}
	return result
}

func hasAutomationPublicHookDownloadField(obj map[string]interface{}) bool {
	for _, key := range []string{"downloadAddress", "downloadPath", "outputPath"} {
		if value, exists := obj[key]; exists && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return true
		}
	}
	return false
}

func decodeAutomationRunPayloadValue(raw string) (interface{}, interface{}, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil, false
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, nil, false
	}

	if obj, ok := payload.(map[string]interface{}); ok {
		result, exists := obj["result"]
		return payload, result, exists
	}
	return payload, nil, false
}
