package functions

import (
	"fmt"
	"log"
	"testing"
)

func TestFormat(t *testing.T) {
	s, err := Format("Hello {0}, you have {1} new messages", "Alice", 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(s) // Hello Alice, you have 5 new messages
}
