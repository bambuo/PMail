package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/hooks/wasmabi"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// WasmHook 把 WASM 插件适配为 contract.EmailHook。
type WasmHook struct {
	id            string
	manifest      *WasmManifest
	grantedHooks  map[string]bool
	grantedCaps   map[string]bool
	exec          *wasmExecutor
	serverVersion string
	counters      *pluginCounters
	draining      atomic.Bool
	inflight      sync.WaitGroup
}

var _ contract.EmailHook = (*WasmHook)(nil)

func (h *WasmHook) Name() string { return h.id }

func (h *WasmHook) GetName(ctx *pcontext.Context) string {
	resp, err := h.invoke(ctx, wasmabi.HookGetName, &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "GetName",
		ServerVersion: h.serverVersion,
	}, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil {
		return ""
	}
	var r nameResponse
	if json.Unmarshal(resp, &r) != nil {
		return ""
	}
	name := strings.ReplaceAll(r.Name, " ", "")
	if name == "" {
		return h.manifest.Name
	}
	return name
}

func (h *WasmHook) SettingsHtml(ctx *pcontext.Context, url, requestData string) string {
	if !h.grantedHooks["SettingsHtml"] {
		return ""
	}
	resp, err := h.invoke(ctx, wasmabi.HookSettingsHTML, &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "SettingsHtml",
		ServerVersion: h.serverVersion,
		URL:           url,
		RequestData:   requestData,
	}, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil {
		return ""
	}
	var r htmlResponse
	if json.Unmarshal(resp, &r) != nil {
		return ""
	}
	return r.HTML
}

func (h *WasmHook) SendBefore(ctx *pcontext.Context, email *parsemail.Email) {
	if !h.grantedHooks["SendBefore"] {
		return
	}
	req := h.request(wasmabi.HookSendBefore, "SendBefore", email)
	resp, err := h.invoke(ctx, wasmabi.HookSendBefore, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err == nil && h.grantedCaps[capEmailWrite] {
		h.applyEmailResponse(email, resp)
	}
}

func (h *WasmHook) SendAfter(ctx *pcontext.Context, email *parsemail.Email, errs map[string]error) {
	if !h.grantedHooks["SendAfter"] {
		return
	}
	req := h.request(wasmabi.HookSendAfter, "SendAfter", email)
	req.Errors = map[string]string{}
	for host, err := range errs {
		if err != nil {
			req.Errors[host] = err.Error()
		}
	}
	_, err := h.invoke(ctx, wasmabi.HookSendAfter, req, time.Duration(h.manifest.Limits.AsyncTimeoutMS)*time.Millisecond)
	h.report(ctx, err)
}

func (h *WasmHook) ReceiveParseBefore(ctx *pcontext.Context, raw *[]byte) {
	if !h.grantedHooks["ReceiveParseBefore"] {
		return
	}
	if raw == nil {
		return
	}
	if len(*raw) > maxRawBytes {
		h.report(ctx, fmt.Errorf("raw exceeds %d bytes", maxRawBytes))
		return
	}
	req := &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "ReceiveParseBefore",
		ServerVersion: h.serverVersion,
	}
	if h.grantedCaps[capEmailRead] {
		req.Raw = base64.StdEncoding.EncodeToString(*raw)
	}
	resp, err := h.invoke(ctx, wasmabi.HookReceiveParseBefore, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil || !h.grantedCaps[capEmailWrite] {
		return
	}
	var r rawResponse
	if json.Unmarshal(resp, &r) != nil || r.Raw == "" {
		return
	}
	decoded, derr := base64.StdEncoding.DecodeString(r.Raw)
	if derr != nil || len(decoded) > maxRawBytes {
		return
	}
	*raw = decoded
}

func (h *WasmHook) ReceiveParseAfter(ctx *pcontext.Context, email *parsemail.Email) {
	if !h.grantedHooks["ReceiveParseAfter"] {
		return
	}
	req := h.request(wasmabi.HookReceiveParseAfter, "ReceiveParseAfter", email)
	resp, err := h.invoke(ctx, wasmabi.HookReceiveParseAfter, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err == nil && h.grantedCaps[capEmailWrite] {
		h.applyEmailResponse(email, resp)
	}
}

func (h *WasmHook) ReceiveSaveAfter(ctx *pcontext.Context, email *parsemail.Email, ue []*contract.EmailOwner) {
	if !h.grantedHooks["ReceiveSaveAfter"] {
		return
	}
	req := h.request(wasmabi.HookReceiveSaveAfter, "ReceiveSaveAfter", email)
	req.UserEmail = ue
	_, err := h.invoke(ctx, wasmabi.HookReceiveSaveAfter, req, time.Duration(h.manifest.Limits.AsyncTimeoutMS)*time.Millisecond)
	h.report(ctx, err)
}

func (h *WasmHook) request(hookID uint32, hookName string, email *parsemail.Email) *hookRequest {
	req := &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          hookName,
		ServerVersion: h.serverVersion,
	}
	if email == nil {
		return req
	}
	if h.grantedCaps[capEmailRead] {
		req.Email = email
	} else {
		req.Email = redactEmail(email)
	}
	return req
}

func (h *WasmHook) invoke(ctx *pcontext.Context, hookID uint32, req *hookRequest, timeout time.Duration) ([]byte, error) {
	if h.draining.Load() {
		return nil, errors.New("plugin is draining")
	}
	h.inflight.Add(1)
	defer h.inflight.Done()
	if req.Config == nil && len(h.manifest.Config) > 0 {
		req.Config = h.manifest.Config
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	base := context.Background()
	if ctx != nil && ctx.Context != nil {
		base = ctx.Context
	}
	callCtx, cancel := context.WithTimeout(base, timeout)
	defer cancel()
	return h.exec.Call(callCtx, hookID, body)
}

func (h *WasmHook) applyEmailResponse(email *parsemail.Email, resp []byte) {
	if email == nil {
		return
	}
	var r emailResponse
	if json.Unmarshal(resp, &r) != nil || r.Email == nil {
		return
	}
	applyEmail(email, r.Email)
}

func (h *WasmHook) report(ctx *pcontext.Context, err error) {
	if h.counters != nil {
		h.counters.record(err)
	}
	if err == nil {
		return
	}
	entry := log.WithFields(log.Fields{
		"plugin_id": h.id,
		"plugin":    h.manifest.Name,
	})
	if ctx != nil {
		entry = entry.WithContext(ctx)
	}
	entry.Errorf("wasm plugin call failed: %v", err)
}

func (h *WasmHook) drain(timeout time.Duration) {
	h.draining.Store(true)
	done := make(chan struct{})
	go func() {
		h.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

type hookRequest struct {
	ABI           string                 `json:"abi"`
	Hook          string                 `json:"hook"`
	ServerVersion string                 `json:"serverVersion"`
	Email         *parsemail.Email       `json:"email,omitempty"`
	Errors        map[string]string      `json:"errors,omitempty"`
	Raw           string                 `json:"raw,omitempty"`
	UserEmail     []*contract.EmailOwner `json:"userEmail,omitempty"`
	URL           string                 `json:"url,omitempty"`
	RequestData   string                 `json:"requestData,omitempty"`
	Config        map[string]string      `json:"config,omitempty"`
}

type emailPatch struct {
	Subject     *string               `json:"Subject"`
	Text        *[]byte               `json:"Text"`
	HTML        *[]byte               `json:"HTML"`
	Status      *int                  `json:"Status"`
	From        *parsemail.User       `json:"From"`
	To          *[]*parsemail.User    `json:"To"`
	Cc          *[]*parsemail.User    `json:"Cc"`
	Bcc         *[]*parsemail.User    `json:"Bcc"`
	ReplyTo     *[]*parsemail.User    `json:"ReplyTo"`
	Sender      *parsemail.User       `json:"Sender"`
	Headers     *textproto.MIMEHeader `json:"Headers"`
	Date        *string               `json:"Date"`
	ReadReceipt *[]string             `json:"ReadReceipt"`
}

type emailResponse struct {
	Email *emailPatch `json:"email"`
}

type rawResponse struct {
	Raw string `json:"raw"`
}

type nameResponse struct {
	Name string `json:"name"`
}

type htmlResponse struct {
	HTML string `json:"html"`
}

func redactEmail(src *parsemail.Email) *parsemail.Email {
	out := *src
	out.Text = nil
	out.HTML = nil
	out.Headers = nil
	out.Attachments = nil
	return &out
}

func applyEmail(dst *parsemail.Email, src *emailPatch) {
	if dst == nil || src == nil {
		return
	}
	if src.Subject != nil {
		dst.Subject = *src.Subject
	}
	if src.Text != nil {
		dst.Text = *src.Text
	}
	if src.HTML != nil {
		dst.HTML = *src.HTML
	}
	if src.Status != nil {
		dst.Status = *src.Status
	}
	if src.From != nil {
		dst.From = src.From
	}
	if src.To != nil {
		dst.To = *src.To
	}
	if src.Cc != nil {
		dst.Cc = *src.Cc
	}
	if src.Bcc != nil {
		dst.Bcc = *src.Bcc
	}
	if src.ReplyTo != nil {
		dst.ReplyTo = *src.ReplyTo
	}
	if src.Sender != nil {
		dst.Sender = src.Sender
	}
	if src.Headers != nil {
		dst.Headers = *src.Headers
	}
	if src.Date != nil {
		dst.Date = *src.Date
	}
	if src.ReadReceipt != nil {
		dst.ReadReceipt = *src.ReadReceipt
	}
}
