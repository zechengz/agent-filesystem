package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
}

func resolveSkillSource(source string, cwd string, skillSubdir string) (*SkillSource, error) {
	localPath := resolvePath(cwd, source)
	if _, err := os.Stat(localPath); err != nil {
		if looksLikeRemoteSource(source) {
			cloned, err := cloneSource(source)
			if err != nil {
				return nil, err
			}
			localPath = cloned
		} else {
			return nil, fail("Skill source not found: %s", source)
		}
	}

	skillPath := localPath
	if skillSubdir != "" {
		skillPath = filepath.Join(localPath, skillSubdir)
	} else {
		discovered, err := discoverSkillPath(localPath)
		if err != nil {
			return nil, err
		}
		skillPath = discovered
	}

	if err := validateSkillDirectory(skillPath); err != nil {
		return nil, err
	}
	skillMarkdown, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	files, err := collectSkillFiles(skillPath)
	if err != nil {
		return nil, err
	}
	return &SkillSource{
		SourceRoot: localPath,
		SkillPath:  skillPath,
		Metadata:   parseSkillMarkdown(string(skillMarkdown)),
		Files:      files,
	}, nil
}

func listSkillSources(source string, cwd string) ([]SkillSourceListItem, error) {
	localPath := resolvePath(cwd, source)
	if _, err := os.Stat(localPath); err != nil {
		if looksLikeRemoteSource(source) {
			cloned, err := cloneSource(source)
			if err != nil {
				return nil, err
			}
			localPath = cloned
		} else {
			return nil, fail("Skill source not found: %s", source)
		}
	}

	candidates := []string{}
	if fileExists(filepath.Join(localPath, "SKILL.md")) {
		candidates = append(candidates, localPath)
	} else {
		entries, err := os.ReadDir(localPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() && fileExists(filepath.Join(localPath, entry.Name(), "SKILL.md")) {
				candidates = append(candidates, filepath.Join(localPath, entry.Name()))
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fail("No SKILL.md found in %s", localPath)
	}

	items := []SkillSourceListItem{}
	for _, candidate := range candidates {
		data, err := os.ReadFile(filepath.Join(candidate, "SKILL.md"))
		if err != nil {
			return nil, err
		}
		metadata := parseSkillMarkdown(string(data))
		name := metadata["name"]
		if name == "" {
			name = filepath.Base(candidate)
		}
		items = append(items, SkillSourceListItem{
			Name:        name,
			Slug:        slugify(name),
			Description: metadata["description"],
			Path:        filepath.ToSlash(candidate),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func discoverSkillPath(sourcePath string) (string, error) {
	if fileExists(filepath.Join(sourcePath, "SKILL.md")) {
		return sourcePath, nil
	}
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(sourcePath, entry.Name())
		if fileExists(filepath.Join(candidate, "SKILL.md")) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return "", fail("No SKILL.md found in %s", sourcePath)
	}
	if len(matches) > 1 {
		return "", fail("Multiple skills found in %s; pass --skill <directory> to choose one", sourcePath)
	}
	return matches[0], nil
}

func validateSkillDirectory(skillPath string) error {
	info, err := os.Stat(skillPath)
	if err != nil || !info.IsDir() {
		return fail("Skill path is not a directory: %s", skillPath)
	}
	if !fileExists(filepath.Join(skillPath, "SKILL.md")) {
		return fail("Missing SKILL.md in %s", skillPath)
	}
	return nil
}

func parseSkillMarkdown(markdown string) map[string]string {
	metadata := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(markdown))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	bodyStart := 0
	if len(lines) > 0 && lines[0] == "---" {
		end := -1
		for index := 1; index < len(lines); index++ {
			if lines[index] == "---" {
				end = index
				break
			}
		}
		if end != -1 {
			for _, line := range lines[1:end] {
				key, value, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				metadata[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
			}
			bodyStart = end + 1
		}
	}
	if metadata["name"] == "" {
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				metadata["name"] = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
	}
	if metadata["description"] == "" {
		for _, line := range lines[bodyStart:] {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				metadata["description"] = trimmed
				break
			}
		}
	}
	return metadata
}

func collectSkillFiles(root string) ([]SkillFile, error) {
	var files []SkillFile
	err := filepath.WalkDir(root, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if candidate == root {
			return nil
		}
		rel, err := filepath.Rel(root, candidate)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return fail("Unsafe path detected: %s", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fail("Symlinks are not allowed in skills: %s", rel)
		}
		if entry.IsDir() {
			if ignoredDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(candidate)
		if err != nil {
			return err
		}
		files = append(files, SkillFile{
			Path:    rel,
			Content: content,
			Hash:    hashBytes(content),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func looksLikeRemoteSource(source string) bool {
	return strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "git@") ||
		isGitHubShorthand(source)
}

func isGitHubShorthand(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) < 2 {
		return false
	}
	return parts[0] != "" && parts[1] != "" && !strings.Contains(parts[0], ".")
}

func cloneSource(source string) (string, error) {
	url := source
	subpath := ""
	if isGitHubShorthand(source) {
		parts := strings.Split(source, "/")
		url = "https://github.com/" + parts[0] + "/" + parts[1] + ".git"
		if len(parts) > 2 {
			subpath = filepath.Join(parts[2:]...)
		}
	}
	target := filepath.Join(os.TempDir(), "liveskills-source-"+time.Now().UTC().Format("20060102150405.000000000"))
	cmd := exec.Command("git", "clone", "--depth", "1", url, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fail("Could not clone %s: %s", source, strings.TrimSpace(string(output)))
	}
	if subpath != "" {
		return filepath.Join(target, subpath), nil
	}
	return target, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
