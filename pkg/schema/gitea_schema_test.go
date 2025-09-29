package schema

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGiteaSchemaFactory(t *testing.T) {
	schema := GetGiteaWorkflowSchema()
	_ = schema

	data, err := json.MarshalIndent(schema, "", "  ")
	assert.NoError(t, err)
	err = os.WriteFile("gitea_workflow_schema.json", data, 0o600)
	assert.NoError(t, err)
}
