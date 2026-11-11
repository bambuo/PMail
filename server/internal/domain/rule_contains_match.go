package domain

import (
	"strings"
)

type ContainsRule struct {
	Rule  string
	Field string
}

func NewContainsRule(field, rule string) *ContainsRule {
	return &ContainsRule{
		Rule:  rule,
		Field: field,
	}
}

func (r *ContainsRule) Match(content string) (bool, error) {
	return strings.Contains(content, r.Rule), nil
}
