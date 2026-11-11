package domain

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
