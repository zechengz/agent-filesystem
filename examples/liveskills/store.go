package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type RegistryStore struct {
	Home string
	File string
}

func NewRegistryStore(home string) *RegistryStore {
	return &RegistryStore{
		Home: home,
		File: filepath.Join(home, "registry.json"),
	}
}

func (s *RegistryStore) Load() (*Registry, error) {
	data, err := os.ReadFile(s.File)
	if errors.Is(err, os.ErrNotExist) {
		return emptyRegistry(), nil
	}
	if err != nil {
		return nil, err
	}
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}
	normalizeRegistry(&registry)
	return &registry, nil
}

func (s *RegistryStore) Save(registry *Registry) error {
	normalizeRegistry(registry)
	return writeJSON(s.File, registry)
}

func emptyRegistry() *Registry {
	registry := &Registry{Version: 1}
	normalizeRegistry(registry)
	return registry
}

func normalizeRegistry(registry *Registry) {
	if registry.Version == 0 {
		registry.Version = 1
	}
	if registry.Skills == nil {
		registry.Skills = []Skill{}
	}
	if registry.SkillVersions == nil {
		registry.SkillVersions = []SkillVersion{}
	}
	if registry.Installations == nil {
		registry.Installations = []Installation{}
	}
	for index := range registry.Installations {
		installation := &registry.Installations[index]
		if installation.Scope == "" {
			installation.Scope = scopeProject
		}
		if installation.WorkspaceRoot == "" {
			installation.WorkspaceRoot = installation.MountPath
		}
		if installation.InstallPath == "" {
			if installation.MountPath != "" && installation.WorkspacePath != "" {
				installation.InstallPath = filepath.Join(installation.MountPath, filepath.FromSlash(installation.WorkspacePath))
			}
		}
		if installation.Targets == nil {
			installation.Targets = []InstallationTarget{}
		}
		if len(installation.Targets) == 0 && installation.InstallPath != "" {
			installation.Targets = append(installation.Targets, InstallationTarget{
				Agent:     installation.Agent,
				LinkPath:  installation.InstallPath,
				Mode:      installModeSymlink,
				Status:    targetStatusLinked,
				CreatedAt: installation.InstalledAt,
				UpdatedAt: installation.UpdatedAt,
			})
		}
		for targetIndex := range installation.Targets {
			target := &installation.Targets[targetIndex]
			if target.Agent == "" {
				target.Agent = installation.Agent
			}
			if target.Mode == "" {
				target.Mode = installModeSymlink
			}
			if target.Status == "" {
				target.Status = targetStatusLinked
			}
			if target.CreatedAt == "" {
				target.CreatedAt = installation.InstalledAt
			}
			if target.UpdatedAt == "" {
				target.UpdatedAt = installation.UpdatedAt
			}
		}
	}
}
