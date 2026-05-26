package main

type Registry struct {
	Version       int            `json:"version"`
	Config        RegistryConfig `json:"config"`
	Skills        []Skill        `json:"skills"`
	SkillVersions []SkillVersion `json:"skillVersions"`
	Installations []Installation `json:"installations"`
}

type RegistryConfig struct {
	Auth *AuthConfig `json:"auth,omitempty"`
}

type AuthConfig struct {
	Endpoint  string `json:"endpoint"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updatedAt"`
}

type Skill struct {
	ID                string `json:"id"`
	Owner             string `json:"owner"`
	Slug              string `json:"slug"`
	DisplayName       string `json:"displayName"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	Status            string `json:"status"`
	CanonicalVolumeID string `json:"canonicalVolumeId"`
	LatestVersionID   string `json:"latestVersionId"`
	CreatedBy         string `json:"createdBy"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type SkillVersion struct {
	ID           string `json:"id"`
	SkillID      string `json:"skillId"`
	Version      string `json:"version"`
	CheckpointID string `json:"checkpointId"`
	ManifestHash string `json:"manifestHash"`
	ContentHash  string `json:"contentHash"`
	ReleaseNotes string `json:"releaseNotes"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    string `json:"createdAt"`
}

type Installation struct {
	ID              string               `json:"id"`
	Workspace       string               `json:"workspace"`
	Scope           string               `json:"scope"`
	SkillID         string               `json:"skillId"`
	VersionID       string               `json:"versionId"`
	Agent           string               `json:"agent,omitempty"`
	WorkspacePath   string               `json:"workspacePath"`
	MountPath       string               `json:"mountPath"`
	WorkspaceRoot   string               `json:"workspaceRoot,omitempty"`
	SkillsWorkspace string               `json:"skillsWorkspace,omitempty"`
	InstallPath     string               `json:"installPath,omitempty"`
	Targets         []InstallationTarget `json:"targets,omitempty"`
	Tracking        string               `json:"tracking"`
	InstalledBy     string               `json:"installedBy"`
	InstalledAt     string               `json:"installedAt"`
	UpdatedAt       string               `json:"updatedAt"`
}

type InstallationTarget struct {
	Agent     string `json:"agent"`
	LinkPath  string `json:"linkPath"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type WorkspaceManifest struct {
	Version   int             `json:"version"`
	Workspace string          `json:"workspace"`
	Skills    []ManifestSkill `json:"skills"`
}

type ManifestSkill struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Version    string           `json:"version"`
	Checkpoint string           `json:"checkpoint"`
	Volume     string           `json:"volume"`
	Agent      string           `json:"agent,omitempty"`
	Path       string           `json:"path"`
	Targets    []ManifestTarget `json:"targets,omitempty"`
	Tracking   string           `json:"tracking"`
}

type ManifestTarget struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
	Mode  string `json:"mode"`
}

type SkillFile struct {
	Path    string
	Content []byte
	Hash    string
}

type SkillSource struct {
	SourceRoot string
	SkillPath  string
	Metadata   map[string]string
	Files      []SkillFile
}

type SkillSourceListItem struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type InstalledSkillItem struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName,omitempty"`
	Path        string   `json:"path"`
	Canonical   string   `json:"canonical,omitempty"`
	Agents      []string `json:"agents"`
	Scope       string   `json:"scope"`
	Live        bool     `json:"live"`
	Managed     bool     `json:"managed"`
	ManagedBy   string   `json:"managedBy,omitempty"`
	Version     string   `json:"version,omitempty"`
	Volume      string   `json:"volume,omitempty"`
	Checkpoint  string   `json:"checkpoint,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Status      string   `json:"status,omitempty"`
}
