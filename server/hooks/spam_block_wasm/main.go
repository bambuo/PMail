//go:build wasip1

// SpamBlock WASM 插件：把主题+正文送到 RETVec/TensorFlow Serving 模型分类，
// 诈骗置 status=3，广告置 status=5。构建：
//
//	GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build -buildmode=c-shared -o plugin.wasm .
package main

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"unsafe"
)

//go:wasmimport env pmail_emit
func pmailEmit(ptr, size uint32) uint32

//go:wasmimport env pmail_log
func pmailLog(level, ptr, size uint32)

//go:wasmimport env pmail_http_post
func pmailHTTPPost(urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap uint32) int32

const (
	hookReceiveParseAfter = 4 // wasmabi.HookReceiveParseAfter
	maxModelResponse      = 64 * 1024
)

var buffers = map[uint32][]byte{}

//go:wasmexport pmail_alloc
func pmailAlloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

//go:wasmexport pmail_free
func pmailFree(ptr, size uint32) {
	delete(buffers, ptr)
}

//go:wasmexport pmail_hook
func pmailHook(hookID, reqPtr, reqLen uint32) uint32 {
	if hookID != hookReceiveParseAfter {
		return 0
	}
	var req hookRequest
	if err := json.Unmarshal(guestSlice(reqPtr, reqLen), &req); err != nil {
		logf(3, "spam_block: bad request: "+err.Error())
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

	urlBytes := []byte(apiURL)
	urlPtr := keep(urlBytes)
	defer release(urlPtr)
	bodyPtr := keep(payload)
	defer release(bodyPtr)
	outPtr := pmailAlloc(maxModelResponse)
	if outPtr == 0 {
		return 0
	}
	defer pmailFree(outPtr, maxModelResponse)

	n := pmailHTTPPost(urlPtr, uint32(len(urlBytes)), bodyPtr, uint32(len(payload)), outPtr, maxModelResponse)
	if n <= 0 {
		logf(2, "spam_block: model request failed: "+strconv.Itoa(int(n)))
		return 0
	}

	var mr modelResponse
	if err := json.Unmarshal(guestSlice(outPtr, uint32(n)), &mr); err != nil {
		logf(3, "spam_block: bad model response: "+err.Error())
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
	emitStatus(status)
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

func emitStatus(status int) {
	body, _ := json.Marshal(map[string]any{"email": map[string]any{"Status": status}})
	ptr := pmailAlloc(uint32(len(body)))
	if ptr == 0 {
		return
	}
	copy(guestSlice(ptr, uint32(len(body))), body)
	pmailEmit(ptr, uint32(len(body)))
	pmailFree(ptr, uint32(len(body)))
}

func guestSlice(ptr, size uint32) []byte {
	if size == 0 || ptr == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

func keep(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

func release(ptr uint32) {
	delete(buffers, ptr)
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

func logf(level uint32, msg string) {
	b := []byte(msg)
	ptr := keep(b)
	defer release(ptr)
	pmailLog(level, ptr, uint32(len(b)))
}

func main() {}
