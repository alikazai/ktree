package install

// Target is the stable installer target identifier that owns a metadata record.
// Shared core bundles use the shared target until a concrete adapter stamps its own target.
const TargetShared = "shared"

type Metadata struct {
	ID              string
	Version         string
	Hash            string
	Target          string
	InstalledAtUnix int64
}

type SkillBundle struct {
	Metadata Metadata
	Content  string
}

func MetadataForTarget(metadata Metadata, target string) Metadata {
	stamped := metadata
	if target == "" {
		return stamped
	}
	stamped.Target = target
	return stamped
}
