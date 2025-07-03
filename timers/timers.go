package timers

import (
	"fmt"
	"strings"
	"time"
)

var indent = 0

func Timer(name string) func() {
	fmt.Print(strings.Repeat("  ", indent))
	fmt.Println(name, "started")
	indent++
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		indent--
		fmt.Print(strings.Repeat("  ", indent))
		fmt.Println(name, "took", elapsed)
	}
}
