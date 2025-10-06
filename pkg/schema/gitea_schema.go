package schema

import "slices"

func GetGiteaWorkflowSchema() *Schema {
	schema := GetWorkflowSchema()
	in := schema.Definitions
	schema.Definitions = map[string]Definition{}
	for k, v := range in {
		if v.Context != nil && slices.Contains(v.Context, "github") {
			v.Context = append(v.Context, "gitea", "env")
		}
		if k == "step-if" || k == "job-if" || k == "string-strategy-context" {
			v.Context = append(v.Context, "secrets")
		}
		schema.Definitions[k] = v
	}
	updateUses(schema.Definitions["workflow-job"].Mapping)
	updateUses(schema.Definitions["regular-step"].Mapping)

	schema.Definitions["container-mapping"].Mapping.Properties["cmd"] = MappingProperty{
		Type: "sequence-of-non-empty-string",
	}
	return schema
}

func updateUses(mapping *MappingDefinition) {
	uses := mapping.Properties["uses"]
	uses.Type = "string-strategy-context"
	mapping.Properties["uses"] = uses
}
