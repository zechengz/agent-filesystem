package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

func readWorkspaceManifest(workspaceRoot, workspaceName string) (*WorkspaceManifest, error) {
	file := workspaceManifestPath(workspaceRoot)
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return &WorkspaceManifest{Version: 1, Workspace: workspaceName, Skills: []ManifestSkill{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest WorkspaceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fail("Workspace manifest is corrupt: %s", file)
	}
	if manifest.Version != 1 || manifest.Skills == nil {
		return nil, fail("Workspace manifest has unsupported schema: %s", file)
	}
	if manifest.Workspace == "" {
		manifest.Workspace = workspaceName
	}
	return &manifest, nil
}

func writeWorkspaceManifest(workspaceRoot string, manifest *WorkspaceManifest) error {
	sort.Slice(manifest.Skills, func(i, j int) bool {
		return manifest.Skills[i].Name < manifest.Skills[j].Name
	})
	return writeJSON(workspaceManifestPath(workspaceRoot), manifest)
}

func workspaceManifestPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".liveskills", "manifest.json")
}

func upsertManifestSkill(manifest *WorkspaceManifest, next ManifestSkill) (string, error) {
	for index, existing := range manifest.Skills {
		if existing.Name == next.Name && existing.Agent == next.Agent && existing.Path == next.Path {
			if existing.Version != next.Version || existing.Checkpoint != next.Checkpoint {
				return "", fail("%s is already installed at %s with version %s. Use liveskills update to change versions.", next.Name, next.Path, existing.Version)
			}
			next.Targets = mergeManifestTargets(existing.Targets, next.Targets)
			manifest.Skills[index] = next
			return "unchanged", nil
		}
	}
	manifest.Skills = append(manifest.Skills, next)
	return "created", nil
}

func mergeManifestTargets(existing []ManifestTarget, incoming []ManifestTarget) []ManifestTarget {
	merged := append([]ManifestTarget(nil), existing...)
	for _, target := range incoming {
		found := false
		for index := range merged {
			if merged[index].Agent == target.Agent && filepath.Clean(merged[index].Path) == filepath.Clean(target.Path) {
				merged[index] = target
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, target)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Agent == merged[j].Agent {
			return merged[i].Path < merged[j].Path
		}
		return merged[i].Agent < merged[j].Agent
	})
	return merged
}

func removeManifestSkill(manifest *WorkspaceManifest, name, agent string) bool {
	next := manifest.Skills[:0]
	removed := false
	for _, skill := range manifest.Skills {
		if skill.Name == name && (agent == "" || skill.Agent == agent) {
			removed = true
			continue
		}
		next = append(next, skill)
	}
	manifest.Skills = next
	return removed
}
