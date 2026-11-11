package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	code, err := totp.GenerateCode(os.Args[1], time.Now())
	if err != nil {
		panic(err)
	}
	fmt.Print(code)
}
