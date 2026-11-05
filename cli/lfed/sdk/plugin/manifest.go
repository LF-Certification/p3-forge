package plugin

const ManifestSchemaVersion = "1"

type Manifest struct {
	SchemaVersion string `json:"schema_version"`

	Name    string `json:"name"`
	Version string `json:"version"`

	Summary string `json:"summary"`

	RequiresLfed string `json:"requires_lfed"`

	Commands []Command `json:"commands"`
	Hidden   bool      `json:"hidden,omitempty"`

	SDKVersion string `json:"sdk_version,omitempty"`
}

type Command struct {
	Path    []string `json:"path"`
	Summary string   `json:"summary"`
}
