package plugin

import "errors"

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
	StatusRejected Status = "rejected"
	StatusError    Status = "error"
)

// Info 是管理接口与用例层共用的插件视图。
type Info struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Version             string   `json:"version"`
	ABI                 string   `json:"abi"`
	Status              Status   `json:"status"`
	Hooks               []string `json:"hooks"`
	Capabilities        []string `json:"capabilities"`
	GrantedHooks        []string `json:"grantedHooks"`
	GrantedCapabilities []string `json:"grantedCapabilities"`
	LastError           string   `json:"lastError"`
	CallCount           int64    `json:"callCount"`
	ErrorCount          int64    `json:"errorCount"`
	LastUsedAt          string   `json:"lastUsedAt"`
}

var (
	ErrNotFound     = errors.New("plugin not found")
	ErrInvalidState = errors.New("plugin state does not allow this operation")
	ErrInvalidGrant = errors.New("requested grant is not declared in manifest")
	ErrNameConflict = errors.New("plugin name conflicts with an existing hook")
	ErrDisabled     = errors.New("wasm plugin feature is disabled")
)
