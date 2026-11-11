package domain

import (
	"github.com/dlclark/regexp2"
)

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
