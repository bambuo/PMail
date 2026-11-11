//go:build wasip1

// SpamBlock WASM 插件：把主题+正文送到 RETVec/TensorFlow Serving 模型分类，
// 诈骗置 status=3，广告置 status=5。
package main

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Jinnrry/pmail/hooks/wasm_plugins/internal/pluginabi"
)

const (
	hookReceiveParseAfter = 4
	maxModelResponse      = 64 * 1024
)

//go:wasmexport pmail_alloc
func pmailAlloc(size uint32) uint32 { return pluginabi.Alloc(size) }

//go:wasmexport pmail_free
func pmailFree(ptr, size uint32) { pluginabi.Free(ptr, size) }

//go:wasmexport pmail_hook
func pmailHook(hookID, reqPtr, reqLen uint32) uint32 {
	if hookID != hookReceiveParseAfter {
		return 0
	}
	var req hookRequest
	if err := json.Unmarshal(pluginabi.Guest(reqPtr, reqLen), &req); err != nil {
		pluginabi.Log(3, "spam_block: bad request: "+err.Error())
		return 0
	}
	if req.Email == nil {
		return 0
	}
	apiURL := req.Config["api_url"]
	if apiURL == "" {
		return 0
	}
	threshold := 0.20
	if v := req.Config["threshold"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}

	content := plainText(decodeText(req.Email.Text))
	if content == "" {
		content = plainText(decodeText(req.Email.HTML))
	}
	if content == "" {
		return 0
	}

	payload, err := json.Marshal(map[string]any{
		"instances": []any{
			map[string]any{"token": []string{req.Email.Subject + " " + content}},
		},
	})
	if err != nil {
		return 0
	}

	resp, code := pluginabi.HTTPPost(apiURL, payload, maxModelResponse)
	if code < 0 {
		pluginabi.Log(2, "spam_block: model request failed: "+strconv.Itoa(int(code)))
		return 0
	}
	var mr modelResponse
	if err := json.Unmarshal(resp, &mr); err != nil {
		pluginabi.Log(3, "spam_block: bad model response: "+err.Error())
		return 0
	}
	if len(mr.Predictions) == 0 || len(mr.Predictions[0]) != 3 {
		return 0
	}
	class, score := 0, 0.0
	for i, v := range mr.Predictions[0] {
		if i == 0 || v > score {
			class, score = i, v
		}
	}
	if class == 0 || score <= threshold {
		return 0
	}
	status := 5
	if class == 2 {
		status = 3
	}
	body, _ := json.Marshal(map[string]any{"email": map[string]any{"Status": status}})
	pluginabi.Emit(body)
	return 0
}

type hookRequest struct {
	Hook   string            `json:"hook"`
	Email  *email            `json:"email"`
	Config map[string]string `json:"config"`
}

type email struct {
	Subject string `json:"Subject"`
	Text    string `json:"Text"`
	HTML    string `json:"HTML"`
}

type modelResponse struct {
	Predictions [][]float64 `json:"predictions"`
}

func decodeText(b64 string) string {
	if b64 == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return string(b)
}

func plainText(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func main() {}
