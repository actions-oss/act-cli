package schema

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGiteaSchemaFactory(t *testing.T) {
	schema := GetGiteaWorkflowSchema()
	_ = schema

	data, _ := json.MarshalIndent(schema, "", "  ")
	os.WriteFile("gitea_workflow_schema.json", data, 0o666)
}
