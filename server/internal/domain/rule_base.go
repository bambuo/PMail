package domain

const (
	RuleTypeRegex    = "regex"
	RuleTypeContains = "contains"
	RuleTypeEq       = "equal"
)

type RuleMatch interface {
	Match(content string) (bool, error)
}
