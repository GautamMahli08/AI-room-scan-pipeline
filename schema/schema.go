// Package schema embeds the published output schema so the binary validates
// every plan it writes without needing the repo on disk.
package schema

import _ "embed"

//go:embed plan.schema.json
var PlanJSON []byte
