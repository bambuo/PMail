package domain

import (
	"strings"

	"github.com/dlclark/regexp2"
)

const (
	RuleTypeRegex    = "regex"
	RuleTypeContains = "contains"
	RuleTypeEq       = "equal"
)

type RuleMatch interface {
	Match(content string) (bool, error)
}

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

type EqualRule struct {
	Rule  string
	Field string
}

func NewEqualRule(field, rule string) *EqualRule {
	return &EqualRule{
		Rule:  rule,
		Field: field,
	}
}

func (r *EqualRule) Match(content string) (bool, error) {
	return content == r.Rule, nil
}

type RegexRule struct {
	Rule  string
	Field string
}

func NewRegexRule(field, rule string) *RegexRule {
	return &RegexRule{
		Rule:  rule,
		Field: field,
	}
}

func (r *RegexRule) Match(content string) (bool, error) {
	re := regexp2.MustCompile(r.Rule, 0)
	match, err := re.MatchString(content)

	return match, err
}
