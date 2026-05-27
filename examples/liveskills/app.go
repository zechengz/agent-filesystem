package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	scopeProject = "project"
	scopeGlobal  = "global"

	installModeSymlink = "symlink"
	installModeCopy    = "copy"

	targetStatusLinked = "linked"
	targetStatusCopied = "copied"
)

type App struct {
	CWD   string
	Env   map[string]string
	Home  string
	Store *RegistryStore
	AFS   *LocalAFSAdapter
}

type discoveredInstalledSkill struct {
	item InstalledSkillItem
	path string
}

func NewApp(cwd string, env map[string]string) *App {
	home := defaultHome(env)
	return &App{
		CWD:   cwd,
		Env:   env,
		Home:  home,
		Store: NewRegistryStore(home),
		AFS:   NewLocalAFSAdapter(home, env),
	}
}

func (a *App) AuthLogin(endpoint, token string) (map[string]string, error) {
	if endpoint == "" {
		endpoint = "local"
	}
	if token == "" {
		token = envValue(a.Env, "LIVESKILLS_TOKEN")
	}
	if token == "" {
		token = "local-dev"
	}
	registry, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	registry.Config.Auth = &AuthConfig{
		Endpoint:  endpoint,
		Token:     token,
		UpdatedAt: now(),
	}
	if err := a.Store.Save(registry); err != nil {
		return nil, err
	}
	return map[string]string{"endpoint": endpoint}, nil
}

func (a *App) Publish(source string, options map[string]string) (*PublishResult, error) {
	if err := require(source != "", "Usage: liveskills publish <source>"); err != nil {
		return nil, err
	}
	if visibility := options["visibility"]; visibility != "" && visibility != "public" && visibility != "private" && visibility != "team" {
		return nil, fail("Visibility must be public, private, or team")
	}
	resolved, err := resolveSkillSource(source, a.CWD, options["skill"])
	if err != nil {
		return nil, err
	}
	name := options["name"]
	if name == "" {
		name = resolved.Metadata["name"]
	}
	description := resolved.Metadata["description"]
	if err := require(name != "", "Skill metadata must include a name in SKILL.md or --name"); err != nil {
		return nil, err
	}
	if err := require(description != "", "Skill metadata must include a description in SKILL.md"); err != nil {
		return nil, err
	}
	if err := require(hasFile(resolved.Files, "SKILL.md"), "Skill must include SKILL.md"); err != nil {
		return nil, err
	}
	owner := slugify(options["owner"])
	if owner == "" {
		owner = slugify(envValue(a.Env, "LIVESKILLS_OWNER"))
	}
	if owner == "" {
		owner = "local"
	}
	slug := slugify(options["slug"])
	if slug == "" {
		slug = slugify(name)
	}
	if err := require(owner != "", "Owner cannot be empty"); err != nil {
		return nil, err
	}
	if err := require(slug != "", "Skill slug cannot be empty"); err != nil {
		return nil, err
	}

	registry, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	skill := findSkill(registry, owner, slug)
	volumeID, err := a.AFS.EnsureSkillVolume(owner, slug)
	if err != nil {
		return nil, err
	}
	versionName := options["version"]
	if versionName == "" {
		versionName = resolved.Metadata["version"]
	}
	if versionName == "" {
		versionName = nextVersionFor(registry, skill)
	}
	if skill != nil {
		if existingVersion := findSkillVersionByName(registry, skill.ID, versionName); existingVersion != nil {
			if options["allowExisting"] == "true" {
				return &PublishResult{
					Skill:   *skill,
					Version: *existingVersion,
					Scripts: scriptFiles(resolved.Files),
				}, nil
			}
			return nil, fail("%s/%s version %s already exists", owner, slug, versionName)
		}
	}

	publishedAt := now()
	if skill == nil {
		registry.Skills = append(registry.Skills, Skill{
			ID:                shortID("skill", owner+"/"+slug),
			Owner:             owner,
			Slug:              slug,
			DisplayName:       name,
			Description:       description,
			Visibility:        defaultString(options["visibility"], "private"),
			Status:            "active",
			CanonicalVolumeID: volumeID,
			CreatedBy:         owner,
			CreatedAt:         publishedAt,
			UpdatedAt:         publishedAt,
		})
		skill = &registry.Skills[len(registry.Skills)-1]
	} else {
		skill.DisplayName = name
		skill.Description = description
		if options["visibility"] != "" {
			skill.Visibility = options["visibility"]
		}
		skill.UpdatedAt = publishedAt
	}

	checkpointID, contentHash, err := a.AFS.PublishVersion(volumeID, versionName, resolved.Files)
	if err != nil {
		return nil, err
	}
	metadataJSON, _ := json.Marshal(resolved.Metadata)
	version := SkillVersion{
		ID:           shortID("ver", skill.ID+":"+versionName+":"+contentHash),
		SkillID:      skill.ID,
		Version:      versionName,
		CheckpointID: checkpointID,
		ManifestHash: hashBytes(metadataJSON),
		ContentHash:  contentHash,
		ReleaseNotes: options["releaseNotes"],
		CreatedBy:    owner,
		CreatedAt:    publishedAt,
	}
	registry.SkillVersions = append(registry.SkillVersions, version)
	skill.LatestVersionID = version.ID
	skill.UpdatedAt = publishedAt
	if err := a.Store.Save(registry); err != nil {
		return nil, err
	}

	return &PublishResult{
		Skill:   *skill,
		Version: version,
		Scripts: scriptFiles(resolved.Files),
	}, nil
}

func (a *App) List(query string, installedOnly bool) ([]SkillListItem, error) {
	registry, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	installed := map[string]bool{}
	for _, installation := range registry.Installations {
		installed[installation.SkillID] = true
	}
	query = strings.ToLower(query)
	items := []SkillListItem{}
	for _, skill := range registry.Skills {
		if installedOnly && !installed[skill.ID] {
			continue
		}
		version := findSkillVersionByID(registry, skill.LatestVersionID)
		item := formatSkill(skill, version)
		if query != "" && !strings.Contains(strings.ToLower(item.Name), query) && !strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (a *App) ListInstalled(global bool) ([]InstalledSkillItem, error) {
	registry, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	scope := scopeProject
	root := filepath.Clean(a.CWD)
	if global {
		scope = scopeGlobal
	}
	grouped := map[string]*InstalledSkillItem{}
	knownPaths := map[string]bool{}
	for _, installation := range registry.Installations {
		currentProjectInstall := installation.Scope == scopeProject && filepath.Clean(installation.WorkspaceRoot) == root
		if installation.WorkspaceRoot == "" {
			currentProjectInstall = installation.Scope == scopeProject && filepath.Clean(installation.MountPath) == root
		}
		if global {
			if installation.Scope != scopeGlobal && !currentProjectInstall {
				continue
			}
		} else if !currentProjectInstall {
			continue
		}
		skill := findSkillByID(registry, installation.SkillID)
		if skill == nil {
			continue
		}
		version := findSkillVersionByID(registry, installation.VersionID)
		if version == nil {
			version = findSkillVersionByID(registry, skill.LatestVersionID)
		}
		name := skill.Owner + "/" + skill.Slug
		itemPath := installationDisplayPath(installation)
		key := name + "\x00" + itemPath + "\x00" + installation.Scope
		item := grouped[key]
		if item == nil {
			for _, target := range installation.Targets {
				knownPaths[installedPathKey(installation.Scope, target.LinkPath)] = true
			}
			item = &InstalledSkillItem{
				Name:        name,
				DisplayName: defaultString(skill.DisplayName, skill.Slug),
				Path:        displayInstallPath(itemPath, a.CWD, installation.Scope),
				Canonical:   homeRelative(canonicalSkillPath(installation.MountPath, skill.Slug)),
				Agents:      []string{},
				Scope:       installation.Scope,
				Live:        true,
				Managed:     true,
				ManagedBy:   "LiveSkills",
				Version:     skillVersionName(version),
				Volume:      skill.CanonicalVolumeID,
				Checkpoint:  skillCheckpointID(version),
				Mode:        firstTargetMode(installation.Targets),
				Status:      installationStatus(installation, skill.Slug),
			}
			grouped[key] = item
		}
		for _, agent := range a.installationAgents(installation) {
			if !containsString(item.Agents, agent) {
				item.Agents = append(item.Agents, agent)
			}
		}
	}
	discovered, err := a.discoverInstalledSkills(scope)
	if err != nil {
		return nil, err
	}
	for _, discoveredSkill := range discovered {
		if knownPaths[installedPathKey(scope, discoveredSkill.path)] {
			continue
		}
		key := discoveredSkill.item.Name + "\x00" + discoveredSkill.path + "\x00" + scope
		if grouped[key] == nil {
			item := discoveredSkill.item
			grouped[key] = &item
		}
	}
	items := make([]InstalledSkillItem, 0, len(grouped))
	for _, item := range grouped {
		sort.Strings(item.Agents)
		items = append(items, *item)
	}
	sortInstalledSkillItems(items, global)
	return items, nil
}

func (a *App) discoverInstalledSkills(scope string) ([]discoveredInstalledSkill, error) {
	home := homeForScan(a.Env)
	includeProject := scope == scopeProject
	includeGlobal := scope == scopeGlobal
	canonicalPath := filepath.Join(a.CWD, ".agents", "skills")
	if scope == scopeGlobal {
		canonicalPath = filepath.Join(home, ".agents", "skills")
	}
	canonicalPath = filepath.Clean(canonicalPath)
	locations := scanLocations(a.CWD, home, a.Env, includeProject, includeGlobal, "")
	byName := map[string]*discoveredInstalledSkill{}
	var names []string

	for _, location := range locations {
		if location.Scope != scope {
			continue
		}
		skillDirs, err := scanSkillDirs(location)
		if err != nil {
			return nil, err
		}
		for _, skillDir := range skillDirs {
			scanned, err := scanSkillDirectory(skillDir, location.Scope, a.CWD, home)
			if err != nil {
				return nil, err
			}
			key := scanned.Scope + "\x00" + scanned.Name
			existing := byName[key]
			if existing == nil {
				agents := append([]string(nil), location.Agents...)
				if filepath.Clean(location.Path) == canonicalPath {
					if scope == scopeProject {
						agents = detectedProjectUniversalAgents(a.CWD, home, a.Env)
					} else {
						agents = []string{}
					}
				}
				item := InstalledSkillItem{
					Name:    scanned.Name,
					Path:    scanned.DisplayPath,
					Agents:  agents,
					Scope:   scanned.Scope,
					Live:    false,
					Managed: false,
				}
				byName[key] = &discoveredInstalledSkill{item: item, path: scanned.Path}
				names = append(names, key)
				continue
			}
			for _, agent := range location.Agents {
				if !containsString(existing.item.Agents, agent) {
					existing.item.Agents = append(existing.item.Agents, agent)
				}
			}
		}
	}

	items := make([]discoveredInstalledSkill, 0, len(names))
	for _, name := range names {
		item := byName[name]
		sort.Strings(item.item.Agents)
		items = append(items, *item)
	}
	return items, nil
}

func (a *App) installationAgents(installation Installation) []string {
	seen := map[string]bool{}
	var agents []string
	for _, target := range installation.Targets {
		detected := a.detectAgentsForInstallPath(target.LinkPath, installation.Scope)
		if len(detected) > 0 {
			for _, agent := range detected {
				if !seen[agent] {
					seen[agent] = true
					agents = append(agents, agent)
				}
			}
			continue
		}
		display := agentDisplayName(target.Agent)
		if !seen[display] {
			seen[display] = true
			agents = append(agents, display)
		}
	}
	if len(agents) == 0 && installation.Agent != "" {
		agents = append(agents, agentDisplayName(installation.Agent))
	}
	sort.Strings(agents)
	return agents
}

func (a *App) detectAgentsForInstallPath(path string, scope string) []string {
	if scope != scopeProject {
		return []string{}
	}
	canonicalProjectRoot := filepath.Join(a.CWD, ".agents", "skills")
	if !pathWithinDirectory(canonicalProjectRoot, path) {
		return []string{}
	}
	return detectedProjectUniversalAgents(a.CWD, homeForScan(a.Env), a.Env)
}

func pathWithinDirectory(root string, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func installedPathKey(scope string, path string) string {
	return scope + "\x00" + filepath.Clean(path)
}

func installationDisplayPath(installation Installation) string {
	for _, target := range installation.Targets {
		if strings.Contains(filepath.ToSlash(target.LinkPath), "/.agents/skills/") {
			return target.LinkPath
		}
	}
	if len(installation.Targets) > 0 && installation.Targets[0].LinkPath != "" {
		return installation.Targets[0].LinkPath
	}
	return installation.InstallPath
}

func installationStatus(installation Installation, slug string) string {
	canonical := canonicalSkillPath(installation.MountPath, slug)
	if _, err := os.Stat(canonical); err != nil {
		return "mount missing"
	}
	for _, target := range installation.Targets {
		if target.Mode == installModeCopy {
			if _, err := os.Stat(filepath.Join(target.LinkPath, "SKILL.md")); err != nil {
				return "copy missing"
			}
			continue
		}
		if !SkillSymlinkPointsTo(target.LinkPath, canonical) {
			return "link missing"
		}
	}
	return "live"
}

func skillVersionName(version *SkillVersion) string {
	if version == nil {
		return ""
	}
	return version.Version
}

func skillCheckpointID(version *SkillVersion) string {
	if version == nil {
		return ""
	}
	return version.CheckpointID
}

func sortInstalledSkillItems(items []InstalledSkillItem, global bool) {
	sort.Slice(items, func(i, j int) bool {
		if global && items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if items[i].Name == items[j].Name {
			return items[i].Path < items[j].Path
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

func (a *App) ListSource(source string) ([]SkillSourceListItem, error) {
	return listSkillSources(source, a.CWD)
}

func (a *App) Show(ref string) (*SkillDetail, error) {
	registry, skill, versions, err := a.resolveSkillWithVersions(ref)
	if err != nil {
		return nil, err
	}
	latest := findSkillVersionByID(registry, skill.LatestVersionID)
	detail := SkillDetail{
		SkillListItem: formatSkill(*skill, latest),
		Visibility:    skill.Visibility,
		Volume:        skill.CanonicalVolumeID,
		Versions:      []VersionDetail{},
	}
	for _, version := range versions {
		detail.Versions = append(detail.Versions, VersionDetail{
			ID:          version.ID,
			Version:     version.Version,
			Checkpoint:  version.CheckpointID,
			ContentHash: version.ContentHash,
			CreatedAt:   version.CreatedAt,
		})
	}
	return &detail, nil
}

func (a *App) Download(ref string, versionName string, output string) (*DownloadResult, error) {
	skill, version, err := a.resolveSkillVersion(ref, versionName)
	if err != nil {
		return nil, err
	}
	if output == "" {
		output = skill.Slug
	}
	output = resolvePath(a.CWD, output)
	if err := a.AFS.ExportVersion(skill.CanonicalVolumeID, version.CheckpointID, output); err != nil {
		return nil, err
	}
	return &DownloadResult{Name: skill.Owner + "/" + skill.Slug, Version: version.Version, Checkpoint: version.CheckpointID, Output: output}, nil
}

func (a *App) Add(ref string, options map[string]string) (*InstallResult, error) {
	results, err := a.AddMany(ref, options)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fail("No skills installed")
	}
	return &results[0], nil
}

func (a *App) AddMany(ref string, options map[string]string) ([]InstallResult, error) {
	if err := require(ref != "", "Usage: liveskills add <source-or-ref>"); err != nil {
		return nil, err
	}
	inputs, err := a.resolveInstallInputs(ref, options)
	if err != nil {
		return nil, err
	}
	results := make([]InstallResult, 0, len(inputs))
	for _, input := range inputs {
		result, err := a.installResolvedSkill(input.skill, input.version, options)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, nil
}

type installInput struct {
	skill   *Skill
	version *SkillVersion
}

func (a *App) resolveInstallInputs(ref string, options map[string]string) ([]installInput, error) {
	sourceRef, inlineSkill := splitInlineSkillRef(ref)
	skillSelectors := optionList(options, "skills")
	if inlineSkill != "" {
		skillSelectors = append(skillSelectors, inlineSkill)
	}
	if len(skillSelectors) == 0 && options["skill"] != "" {
		skillSelectors = append(skillSelectors, options["skill"])
	}
	if a.shouldTreatAsSource(sourceRef) {
		rows, err := listSkillSources(sourceRef, a.CWD)
		if err != nil {
			return nil, err
		}
		selected, err := selectSourceSkills(rows, skillSelectors, options["all"] == "true")
		if err != nil {
			return nil, err
		}
		inputs := make([]installInput, 0, len(selected))
		for _, row := range selected {
			publishOptions := copyOptions(options)
			publishOptions["allowExisting"] = "true"
			publishOptions["skill"] = ""
			published, err := a.Publish(row.Path, publishOptions)
			if err != nil {
				return nil, err
			}
			skill := findSkillByIDFromStore(&published.Skill)
			version := published.Version
			inputs = append(inputs, installInput{skill: skill, version: &version})
		}
		return inputs, nil
	}
	skill, version, err := a.resolveSkillVersion(ref, options["version"])
	if err != nil {
		return nil, err
	}
	return []installInput{{skill: skill, version: version}}, nil
}

func findSkillByIDFromStore(skill *Skill) *Skill {
	return skill
}

func selectSourceSkills(rows []SkillSourceListItem, selectors []string, all bool) ([]SkillSourceListItem, error) {
	if len(rows) == 0 {
		return nil, fail("No skills found in source")
	}
	if all || len(selectors) == 0 && len(rows) == 1 {
		return rows, nil
	}
	if len(selectors) == 0 {
		return nil, fail("Multiple skills found; pass --skill <name> for each skill or --all to install all")
	}
	selected := []SkillSourceListItem{}
	seen := map[string]bool{}
	for _, selector := range selectors {
		selector = slugify(selector)
		matched := false
		for _, row := range rows {
			if selector == row.Slug || selector == slugify(row.Name) || selector == slugify(filepath.Base(row.Path)) {
				if !seen[row.Path] {
					selected = append(selected, row)
					seen[row.Path] = true
				}
				matched = true
			}
		}
		if !matched {
			return nil, fail("Skill %q not found in source", selector)
		}
	}
	return selected, nil
}

func (a *App) installResolvedSkill(skill *Skill, version *SkillVersion, options map[string]string) (*InstallResult, error) {
	scope := installScope(options)
	workspaceName := workspaceNameForScope(a.CWD, options, scope)
	projectRoot := installProjectRoot(a.CWD, a.Home, workspaceName, options, scope)
	mountPoint := skillsWorkspaceMountPoint(projectRoot, a.Home, scope)
	mode := installMode(options)
	targets, err := a.installTargets(scope, skill.Slug, projectRoot, options)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fail("No install targets selected")
	}
	workspace, err := a.AFS.EnsureSkillsWorkspace(scope, workspaceName)
	if err != nil {
		return nil, err
	}
	workspace, err = a.AFS.MountSkillsWorkspace(workspace, mountPoint)
	if err != nil {
		return nil, err
	}
	canonicalPath, err := a.AFS.AttachSkillVolume(workspace, skill.CanonicalVolumeID, version.CheckpointID, skill.Slug)
	if err != nil {
		return nil, err
	}
	targetResults := make([]InstallTargetResult, 0, len(targets))
	targetRecords := make([]InstallationTarget, 0, len(targets))
	nowValue := now()
	status := "created"
	for _, target := range dedupeAgentTargets(targets) {
		targetStatus := targetStatusLinked
		if mode == installModeCopy {
			if err := CopySkillFromCanonical(canonicalPath, target.InstallPath); err != nil {
				return nil, err
			}
			targetStatus = targetStatusCopied
		} else if err := EnsureRelativeSkillSymlink(target.InstallPath, canonicalPath); err != nil {
			return nil, err
		}
		targetResults = append(targetResults, InstallTargetResult{
			Agent:  target.AgentDisplayName,
			Path:   target.InstallPath,
			Mode:   mode,
			Status: targetStatus,
		})
		targetRecords = append(targetRecords, InstallationTarget{
			Agent:     target.AgentName,
			LinkPath:  target.InstallPath,
			Mode:      mode,
			Status:    targetStatus,
			CreatedAt: nowValue,
			UpdatedAt: nowValue,
		})
	}
	if scope == scopeProject {
		manifest, err := readWorkspaceManifest(projectRoot, workspaceName)
		if err != nil {
			return nil, err
		}
		status, err = upsertManifestSkill(manifest, ManifestSkill{
			ID:         skill.ID,
			Name:       skill.Owner + "/" + skill.Slug,
			Version:    version.Version,
			Checkpoint: version.CheckpointID,
			Volume:     skill.CanonicalVolumeID,
			Path:       workspaceSkillPath(skill.Slug),
			Targets:    manifestTargets(targetRecords),
			Tracking:   "pinned",
		})
		if err != nil {
			return nil, err
		}
		if err := writeWorkspaceManifest(projectRoot, manifest); err != nil {
			return nil, err
		}
	}
	if err := a.upsertInstallation(workspaceName, scope, projectRoot, workspace, canonicalPath, skill, version, targetRecords); err != nil {
		return nil, err
	}
	return &InstallResult{
		Status:        status,
		Name:          skill.Owner + "/" + skill.Slug,
		Version:       version.Version,
		Checkpoint:    version.CheckpointID,
		Scope:         scope,
		Workspace:     workspaceName,
		WorkspaceRoot: projectRoot,
		MountPoint:    workspace.Root,
		CanonicalPath: canonicalPath,
		Path:          workspaceSkillPath(skill.Slug),
		Mode:          mode,
		Targets:       targetResults,
		ListCommand:   listCommandForScope(scope),
	}, nil
}

func (a *App) installTargets(scope string, slug string, projectRoot string, options map[string]string) ([]AgentTarget, error) {
	selectors := optionList(options, "agents")
	if len(selectors) == 0 && options["agent"] != "" {
		selectors = append(selectors, options["agent"])
	}
	if len(selectors) == 0 {
		if scope == scopeGlobal {
			selectors = []string{"codex"}
		} else {
			selectors = []string{"codex"}
		}
	}
	return AgentInstallTargets(projectRoot, homeForScan(a.Env), a.Env, scope, selectors, slug)
}

func (a *App) Update(ref string, options map[string]string) (*InstallResult, error) {
	scope := installScope(options)
	workspaceName := workspaceNameForScope(a.CWD, options, scope)
	projectRoot := installProjectRoot(a.CWD, a.Home, workspaceName, options, scope)
	skill, version, err := a.resolveSkillVersion(ref, options["version"])
	if err != nil {
		return nil, err
	}
	registry, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	installation := findInstallation(registry, skill, options["workspace"], options["agent"], scope, projectRoot)
	if installation == nil {
		return nil, fail("%s/%s is not installed in the requested workspace", skill.Owner, skill.Slug)
	}
	workspace := SkillsWorkspace{ID: installation.SkillsWorkspace, Name: installation.Workspace, Scope: installation.Scope, Root: installation.MountPath}
	canonicalPath, err := a.AFS.AttachSkillVolume(workspace, skill.CanonicalVolumeID, version.CheckpointID, skill.Slug)
	if err != nil {
		return nil, err
	}
	for _, target := range installation.Targets {
		if target.Mode == installModeCopy {
			if err := CopySkillFromCanonical(canonicalPath, target.LinkPath); err != nil {
				return nil, err
			}
		} else if err := EnsureRelativeSkillSymlink(target.LinkPath, canonicalPath); err != nil {
			return nil, err
		}
	}
	installation.VersionID = version.ID
	installation.WorkspacePath = workspaceSkillPath(skill.Slug)
	installation.UpdatedAt = now()
	if err := a.Store.Save(registry); err != nil {
		return nil, err
	}
	if installation.Scope == scopeProject {
		if err := a.updateManifestVersion(installation.WorkspaceRoot, installation.Workspace, skill, version); err != nil {
			return nil, err
		}
	}
	return installResultFromInstallation(installation, skill, version), nil
}

func (a *App) Remove(ref string, options map[string]string) (*RemoveResult, error) {
	scope := installScope(options)
	workspaceName := workspaceNameForScope(a.CWD, options, scope)
	projectRoot := installProjectRoot(a.CWD, a.Home, workspaceName, options, scope)
	registry, skill, _, err := a.resolveSkillWithVersions(ref)
	if err != nil {
		return nil, err
	}
	installation := findInstallation(registry, skill, options["workspace"], options["agent"], scope, projectRoot)
	if installation == nil {
		return nil, fail("%s/%s is not installed in the requested workspace", skill.Owner, skill.Slug)
	}
	removeAgents := optionList(options, "agents")
	if len(removeAgents) == 0 && options["agent"] != "" {
		removeAgents = append(removeAgents, options["agent"])
	}
	removeAll := len(removeAgents) == 0
	remainingTargets := installation.Targets[:0]
	removedPaths := []string{}
	for _, target := range installation.Targets {
		removeTarget := removeAll || targetMatchesAnyAgent(target, removeAgents, homeForScan(a.Env), a.Env)
		if !removeTarget {
			remainingTargets = append(remainingTargets, target)
			continue
		}
		if err := os.RemoveAll(target.LinkPath); err != nil {
			return nil, err
		}
		removedPaths = append(removedPaths, target.LinkPath)
	}
	installation.Targets = remainingTargets
	if len(installation.Targets) == 0 {
		workspace := SkillsWorkspace{ID: installation.SkillsWorkspace, Name: installation.Workspace, Scope: installation.Scope, Root: installation.MountPath}
		if err := a.AFS.DetachSkillVolumeIfUnused(workspace, skill.CanonicalVolumeID, skill.Slug, nil); err != nil {
			return nil, err
		}
		next := registry.Installations[:0]
		for _, candidate := range registry.Installations {
			if candidate.ID != installation.ID {
				next = append(next, candidate)
			}
		}
		registry.Installations = next
		if installation.Scope == scopeProject {
			if err := a.removeManifestTargets(installation.WorkspaceRoot, installation.Workspace, skill.Owner+"/"+skill.Slug, nil, true); err != nil {
				return nil, err
			}
		}
	} else {
		installation.UpdatedAt = now()
		if installation.Scope == scopeProject {
			if err := a.removeManifestTargets(installation.WorkspaceRoot, installation.Workspace, skill.Owner+"/"+skill.Slug, removeAgents, false); err != nil {
				return nil, err
			}
		}
	}
	if err := a.Store.Save(registry); err != nil {
		return nil, err
	}
	return &RemoveResult{Name: skill.Owner + "/" + skill.Slug, Workspace: installation.Workspace, WorkspaceRoot: installation.WorkspaceRoot, Path: strings.Join(removedPaths, ", ")}, nil
}

func (a *App) upsertInstallation(workspaceName string, scope string, projectRoot string, workspace SkillsWorkspace, canonicalPath string, skill *Skill, version *SkillVersion, targets []InstallationTarget) error {
	registry, err := a.Store.Load()
	if err != nil {
		return err
	}
	nowValue := now()
	for index := range registry.Installations {
		installation := &registry.Installations[index]
		if installation.Workspace == workspaceName && installation.Scope == scope && installation.SkillID == skill.ID {
			installation.VersionID = version.ID
			installation.WorkspacePath = workspaceSkillPath(skill.Slug)
			installation.MountPath = workspace.Root
			installation.WorkspaceRoot = projectRoot
			installation.SkillsWorkspace = workspace.ID
			installation.InstallPath = firstTargetPath(targets)
			installation.Agent = firstTargetAgent(targets)
			installation.Targets = mergeInstallationTargets(installation.Targets, targets, nowValue)
			installation.UpdatedAt = nowValue
			return a.Store.Save(registry)
		}
	}
	registry.Installations = append(registry.Installations, Installation{
		ID:              shortID("inst", workspaceName+":"+scope+":"+skill.ID),
		Workspace:       workspaceName,
		Scope:           scope,
		SkillID:         skill.ID,
		VersionID:       version.ID,
		Agent:           firstTargetAgent(targets),
		WorkspacePath:   workspaceSkillPath(skill.Slug),
		MountPath:       workspace.Root,
		WorkspaceRoot:   projectRoot,
		SkillsWorkspace: workspace.ID,
		InstallPath:     firstTargetPath(targets),
		Targets:         mergeInstallationTargets(nil, targets, nowValue),
		Tracking:        "pinned",
		InstalledBy:     defaultString(envValue(a.Env, "USER"), "local"),
		InstalledAt:     nowValue,
		UpdatedAt:       nowValue,
	})
	return a.Store.Save(registry)
}

func (a *App) resolveSkillWithVersions(ref string) (*Registry, *Skill, []SkillVersion, error) {
	owner, slug, err := parseSkillRef(ref)
	if err != nil {
		return nil, nil, nil, err
	}
	registry, err := a.Store.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	skill := findSkill(registry, owner, slug)
	if skill == nil {
		return nil, nil, nil, fail("Skill not found: %s/%s", owner, slug)
	}
	versions := []SkillVersion{}
	for _, version := range registry.SkillVersions {
		if version.SkillID == skill.ID {
			versions = append(versions, version)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt > versions[j].CreatedAt
	})
	return registry, skill, versions, nil
}

func (a *App) resolveSkillVersion(ref string, versionName string) (*Skill, *SkillVersion, error) {
	_, skill, versions, err := a.resolveSkillWithVersions(ref)
	if err != nil {
		return nil, nil, err
	}
	var version *SkillVersion
	if versionName != "" {
		for index := range versions {
			if versions[index].Version == versionName {
				version = &versions[index]
				break
			}
		}
	} else {
		for index := range versions {
			if versions[index].ID == skill.LatestVersionID {
				version = &versions[index]
				break
			}
		}
		if version == nil && len(versions) > 0 {
			version = &versions[0]
		}
	}
	if version == nil {
		if versionName != "" {
			return nil, nil, fail("%s version %s not found", ref, versionName)
		}
		return nil, nil, fail("%s has no published versions", ref)
	}
	return skill, version, nil
}

type PublishResult struct {
	Skill   Skill        `json:"skill"`
	Version SkillVersion `json:"version"`
	Scripts []string     `json:"scripts"`
}

type SkillListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Checkpoint  string `json:"checkpoint"`
}

type SkillDetail struct {
	SkillListItem
	Visibility string          `json:"visibility"`
	Volume     string          `json:"volume"`
	Versions   []VersionDetail `json:"versions"`
}

type VersionDetail struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Checkpoint  string `json:"checkpoint"`
	ContentHash string `json:"contentHash"`
	CreatedAt   string `json:"createdAt"`
}

type DownloadResult struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Checkpoint string `json:"checkpoint"`
	Output     string `json:"output"`
}

type InstallResult struct {
	Status        string                `json:"status,omitempty"`
	Name          string                `json:"name"`
	Version       string                `json:"version"`
	Checkpoint    string                `json:"checkpoint,omitempty"`
	Scope         string                `json:"scope"`
	Workspace     string                `json:"workspace"`
	WorkspaceRoot string                `json:"workspaceRoot"`
	MountPoint    string                `json:"mountPoint"`
	CanonicalPath string                `json:"canonicalPath,omitempty"`
	Path          string                `json:"path"`
	Mode          string                `json:"mode,omitempty"`
	Targets       []InstallTargetResult `json:"targets,omitempty"`
	ListCommand   string                `json:"listCommand"`
}

type InstallTargetResult struct {
	Agent  string `json:"agent"`
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Status string `json:"status"`
}

type RemoveResult struct {
	Name          string `json:"name"`
	Workspace     string `json:"workspace"`
	WorkspaceRoot string `json:"workspaceRoot"`
	Path          string `json:"path"`
}

func formatSkill(skill Skill, latest *SkillVersion) SkillListItem {
	version := "-"
	checkpoint := ""
	if latest != nil {
		version = latest.Version
		checkpoint = latest.CheckpointID
	}
	return SkillListItem{
		ID:          skill.ID,
		Name:        skill.Owner + "/" + skill.Slug,
		DisplayName: skill.DisplayName,
		Description: skill.Description,
		Version:     version,
		Checkpoint:  checkpoint,
	}
}

func findSkill(registry *Registry, owner, slug string) *Skill {
	for index := range registry.Skills {
		if registry.Skills[index].Owner == owner && registry.Skills[index].Slug == slug {
			return &registry.Skills[index]
		}
	}
	return nil
}

func findSkillByID(registry *Registry, id string) *Skill {
	for index := range registry.Skills {
		if registry.Skills[index].ID == id {
			return &registry.Skills[index]
		}
	}
	return nil
}

func findSkillVersionByID(registry *Registry, id string) *SkillVersion {
	for index := range registry.SkillVersions {
		if registry.SkillVersions[index].ID == id {
			return &registry.SkillVersions[index]
		}
	}
	return nil
}

func findSkillVersionByName(registry *Registry, skillID, versionName string) *SkillVersion {
	for index := range registry.SkillVersions {
		if registry.SkillVersions[index].SkillID == skillID && registry.SkillVersions[index].Version == versionName {
			return &registry.SkillVersions[index]
		}
	}
	return nil
}

func findInstallation(registry *Registry, skill *Skill, workspace string, agent string, scope string, cwd string) *Installation {
	agent = normalizeAgentSelector(agent)
	for index := range registry.Installations {
		installation := &registry.Installations[index]
		if installation.SkillID != skill.ID {
			continue
		}
		if scope != "" && installation.Scope != scope {
			continue
		}
		if scope == scopeProject && workspace == "" {
			root := installation.WorkspaceRoot
			if root == "" {
				root = installation.MountPath
			}
			if filepath.Clean(root) != filepath.Clean(cwd) {
				continue
			}
		}
		if workspace != "" && installation.Workspace != workspace {
			continue
		}
		if agent != "" && !installationHasAgent(installation, agent) {
			continue
		}
		return installation
	}
	return nil
}

func installationHasAgent(installation *Installation, agent string) bool {
	if agentSelectorMatches(agent, installation.Agent) {
		return true
	}
	for _, target := range installation.Targets {
		if agentSelectorMatches(agent, target.Agent) || agentSelectorMatches(agent, agentDisplayName(target.Agent)) {
			return true
		}
	}
	return false
}

func nextVersionFor(registry *Registry, skill *Skill) string {
	if skill == nil || skill.LatestVersionID == "" {
		return "0.1.0"
	}
	latest := findSkillVersionByID(registry, skill.LatestVersionID)
	if latest == nil {
		return "0.1.0"
	}
	return nextPatchVersion(latest.Version)
}

func scriptFiles(files []SkillFile) []string {
	scripts := []string{}
	for _, file := range files {
		if strings.HasPrefix(file.Path, "scripts/") {
			scripts = append(scripts, file.Path)
		}
	}
	sort.Strings(scripts)
	return scripts
}

func hasFile(files []SkillFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func looksLikeExistingLocalPath(cwd, input string) bool {
	if strings.HasPrefix(input, ".") || strings.HasPrefix(input, "/") || strings.HasPrefix(input, "~") {
		return true
	}
	_, err := os.Stat(resolvePath(cwd, input))
	return err == nil
}

func (a *App) shouldTreatAsSource(input string) bool {
	if looksLikeExistingLocalPath(a.CWD, input) {
		return true
	}
	registry, err := a.Store.Load()
	if err == nil {
		if owner, slug, parseErr := parseSkillRef(input); parseErr == nil && findSkill(registry, owner, slug) != nil {
			return false
		}
	}
	return looksLikeRemoteSource(input)
}

func splitInlineSkillRef(ref string) (source string, skill string) {
	if strings.Count(ref, "@") != 1 {
		return ref, ""
	}
	index := strings.LastIndex(ref, "@")
	if index <= 0 || index == len(ref)-1 {
		return ref, ""
	}
	return ref[:index], ref[index+1:]
}

func copyOptions(options map[string]string) map[string]string {
	next := map[string]string{}
	for key, value := range options {
		next[key] = value
	}
	return next
}

func optionList(options map[string]string, key string) []string {
	raw := strings.TrimSpace(options[key])
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, "\n")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func resolveWorkspaceRoot(cwd, workspace, mount string) string {
	if mount != "" {
		return resolvePath(cwd, mount)
	}
	return cwd
}

func installProjectRoot(cwd, liveskillsHome, workspace string, options map[string]string, scope string) string {
	if scope == scopeGlobal {
		return liveskillsHome
	}
	return resolveWorkspaceRoot(cwd, workspace, options["mount"])
}

func skillsWorkspaceMountPoint(projectRoot, liveskillsHome, scope string) string {
	if scope == scopeGlobal {
		return filepath.Join(liveskillsHome, "mount")
	}
	return filepath.Join(projectRoot, ".liveskills", "mount")
}

func workspaceSkillPath(slug string) string {
	return filepath.ToSlash(filepath.Join("skills", slug))
}

func installMode(options map[string]string) string {
	if options["copy"] == "true" {
		return installModeCopy
	}
	return installModeSymlink
}

func (a *App) installTarget(scope string, agent string, slug string, workspace string, options map[string]string) (workspaceRoot string, relativePath string, installPath string, err error) {
	if scope == scopeGlobal {
		root, err := globalAgentRoot(agent)
		if err != nil {
			return "", "", "", err
		}
		relativePath = slug
		return root, relativePath, filepath.Join(root, relativePath), nil
	}
	relativePath, err = adapterPath(agent, slug)
	if err != nil {
		return "", "", "", err
	}
	workspaceRoot = resolveWorkspaceRoot(a.CWD, workspace, options["mount"])
	return workspaceRoot, relativePath, filepath.Join(workspaceRoot, filepath.FromSlash(relativePath)), nil
}

func installScope(options map[string]string) string {
	if options["global"] == "true" {
		return scopeGlobal
	}
	return scopeProject
}

func listCommandForScope(scope string) string {
	if scope == scopeGlobal {
		return "liveskills list -g"
	}
	return "liveskills list"
}

func workspaceNameForScope(cwd string, options map[string]string, scope string) string {
	if scope == scopeGlobal {
		return "global"
	}
	workspace := options["workspace"]
	if workspace == "" && options["mount"] != "" {
		workspace = filepath.Base(resolvePath(cwd, options["mount"]))
	}
	if workspace == "" {
		workspace = filepath.Base(cwd)
	}
	return workspace
}

func dedupeAgentTargets(targets []AgentTarget) []AgentTarget {
	seen := map[string]bool{}
	deduped := make([]AgentTarget, 0, len(targets))
	for _, target := range targets {
		key := target.AgentName + "\x00" + filepath.Clean(target.InstallPath)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, target)
	}
	return deduped
}

func manifestTargets(targets []InstallationTarget) []ManifestTarget {
	rows := make([]ManifestTarget, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, ManifestTarget{Agent: target.Agent, Path: target.LinkPath, Mode: target.Mode})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Agent == rows[j].Agent {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Agent < rows[j].Agent
	})
	return rows
}

func firstTargetPath(targets []InstallationTarget) string {
	if len(targets) == 0 {
		return ""
	}
	return targets[0].LinkPath
}

func firstTargetAgent(targets []InstallationTarget) string {
	if len(targets) == 0 {
		return ""
	}
	return targets[0].Agent
}

func mergeInstallationTargets(existing []InstallationTarget, incoming []InstallationTarget, timestamp string) []InstallationTarget {
	merged := append([]InstallationTarget(nil), existing...)
	for _, target := range incoming {
		found := false
		for index := range merged {
			if merged[index].Agent == target.Agent && filepath.Clean(merged[index].LinkPath) == filepath.Clean(target.LinkPath) {
				if merged[index].CreatedAt == "" {
					merged[index].CreatedAt = defaultString(target.CreatedAt, timestamp)
				}
				merged[index].Mode = target.Mode
				merged[index].Status = target.Status
				merged[index].UpdatedAt = timestamp
				found = true
				break
			}
		}
		if !found {
			if target.CreatedAt == "" {
				target.CreatedAt = timestamp
			}
			if target.UpdatedAt == "" {
				target.UpdatedAt = timestamp
			}
			merged = append(merged, target)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Agent == merged[j].Agent {
			return merged[i].LinkPath < merged[j].LinkPath
		}
		return merged[i].Agent < merged[j].Agent
	})
	return merged
}

func targetMatchesAnyAgent(target InstallationTarget, selectors []string, home string, env map[string]string) bool {
	for _, selector := range selectors {
		agent, ok := AgentDefinitionByName(home, env, selector)
		if ok && agent.Name == target.Agent {
			return true
		}
		if agentSelectorMatches(normalizeAgentSelector(selector), target.Agent) {
			return true
		}
	}
	return false
}

func installResultFromInstallation(installation *Installation, skill *Skill, version *SkillVersion) *InstallResult {
	targets := make([]InstallTargetResult, 0, len(installation.Targets))
	for _, target := range installation.Targets {
		targets = append(targets, InstallTargetResult{
			Agent:  agentDisplayName(target.Agent),
			Path:   target.LinkPath,
			Mode:   target.Mode,
			Status: target.Status,
		})
	}
	return &InstallResult{
		Name:          skill.Owner + "/" + skill.Slug,
		Version:       skillVersionName(version),
		Checkpoint:    skillCheckpointID(version),
		Scope:         installation.Scope,
		Workspace:     installation.Workspace,
		WorkspaceRoot: installation.WorkspaceRoot,
		MountPoint:    installation.MountPath,
		CanonicalPath: canonicalSkillPath(installation.MountPath, skill.Slug),
		Path:          installation.WorkspacePath,
		Mode:          firstTargetMode(installation.Targets),
		Targets:       targets,
		ListCommand:   listCommandForScope(installation.Scope),
	}
}

func firstTargetMode(targets []InstallationTarget) string {
	if len(targets) == 0 {
		return ""
	}
	mode := targets[0].Mode
	for _, target := range targets[1:] {
		if target.Mode != mode {
			return "mixed"
		}
	}
	return mode
}

func (a *App) updateManifestVersion(projectRoot string, workspace string, skill *Skill, version *SkillVersion) error {
	manifest, err := readWorkspaceManifest(projectRoot, workspace)
	if err != nil {
		return err
	}
	for index := range manifest.Skills {
		if manifest.Skills[index].Name == skill.Owner+"/"+skill.Slug {
			manifest.Skills[index].Version = version.Version
			manifest.Skills[index].Checkpoint = version.CheckpointID
			manifest.Skills[index].Volume = skill.CanonicalVolumeID
		}
	}
	return writeWorkspaceManifest(projectRoot, manifest)
}

func (a *App) removeManifestTargets(projectRoot string, workspace string, name string, selectors []string, removeSkill bool) error {
	manifest, err := readWorkspaceManifest(projectRoot, workspace)
	if err != nil {
		return err
	}
	for index := 0; index < len(manifest.Skills); index++ {
		if manifest.Skills[index].Name != name {
			continue
		}
		if removeSkill {
			manifest.Skills = append(manifest.Skills[:index], manifest.Skills[index+1:]...)
			return writeWorkspaceManifest(projectRoot, manifest)
		}
		nextTargets := manifest.Skills[index].Targets[:0]
		for _, target := range manifest.Skills[index].Targets {
			if manifestTargetMatchesAnyAgent(target, selectors, homeForScan(a.Env), a.Env) {
				continue
			}
			nextTargets = append(nextTargets, target)
		}
		manifest.Skills[index].Targets = nextTargets
		return writeWorkspaceManifest(projectRoot, manifest)
	}
	return nil
}

func manifestTargetMatchesAnyAgent(target ManifestTarget, selectors []string, home string, env map[string]string) bool {
	return targetMatchesAnyAgent(InstallationTarget{Agent: target.Agent}, selectors, home, env)
}

func displayInstallPath(path string, cwd string, scope string) string {
	return homeRelative(path)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
