package domain

import (
	"fmt"
	"testing"

	"github.com/dlclark/regexp2"
)

func TestRegexMatch_Match(t *testing.T) {
	re := regexp2.MustCompile("^(?!.*abc\\.com).*", 0)
	match, err := re.MatchString("aa@abc.com")
	fmt.Println(match, err)
}
