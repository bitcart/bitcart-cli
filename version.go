package main

import (
	"fmt"
)

var Version = "dev"

func UserAgent() string {
	return fmt.Sprintf("bitcart-cli/%s", Version)
}
