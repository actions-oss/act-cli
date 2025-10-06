package functions

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	s, err := Format("Hello {0}, you have {1} new messages", "Alice", 5)
	assert.NoError(t, err)
	fmt.Println(s) // Hello Alice, you have 5 new messages
}
