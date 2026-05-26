package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AgentDefinition struct {
	Name             string
	DisplayName      string
	ProjectDir       string
	GlobalDir        string
	UniversalProject bool
	DetectionDirs    []string
}

type AgentTarget struct {
	AgentName        string
	AgentDisplayName string
	Scope            string
	RootDir          string
	RelativePath     string
	InstallPath      string
}

func AgentDefinitions(home string, env map[string]string) []AgentDefinition {
	configHome := defaultString(strings.TrimSpace(envValue(env, "XDG_CONFIG_HOME")), filepath.Join(home, ".config"))
	codexHome := defaultString(strings.TrimSpace(envValue(env, "CODEX_HOME")), filepath.Join(home, ".codex"))
	claudeHome := defaultString(strings.TrimSpace(envValue(env, "CLAUDE_CONFIG_DIR")), filepath.Join(home, ".claude"))
	vibeHome := defaultString(strings.TrimSpace(envValue(env, "VIBE_HOME")), filepath.Join(home, ".vibe"))
	openClawHome := openClawHome(home)

	return markUniversalProjectAgents([]AgentDefinition{
		{Name: "aider-desk", DisplayName: "AiderDesk", ProjectDir: ".aider-desk/skills", GlobalDir: filepath.Join(home, ".aider-desk", "skills")},
		{Name: "amp", DisplayName: "Amp", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(configHome, "agents", "skills"), DetectionDirs: []string{filepath.Join(configHome, "amp")}},
		{Name: "antigravity", DisplayName: "Antigravity", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".gemini", "antigravity", "skills"), DetectionDirs: []string{filepath.Join(home, ".gemini", "antigravity")}},
		{Name: "augment", DisplayName: "Augment", ProjectDir: ".augment/skills", GlobalDir: filepath.Join(home, ".augment", "skills")},
		{Name: "bob", DisplayName: "IBM Bob", ProjectDir: ".bob/skills", GlobalDir: filepath.Join(home, ".bob", "skills")},
		{Name: "claude-code", DisplayName: "Claude Code", ProjectDir: ".claude/skills", GlobalDir: filepath.Join(claudeHome, "skills")},
		{Name: "openclaw", DisplayName: "OpenClaw", ProjectDir: "skills", GlobalDir: filepath.Join(openClawHome, "skills")},
		{Name: "cline", DisplayName: "Cline", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".agents", "skills"), DetectionDirs: []string{filepath.Join(home, ".cline")}},
		{Name: "codearts-agent", DisplayName: "CodeArts Agent", ProjectDir: ".codeartsdoer/skills", GlobalDir: filepath.Join(home, ".codeartsdoer", "skills")},
		{Name: "codebuddy", DisplayName: "CodeBuddy", ProjectDir: ".codebuddy/skills", GlobalDir: filepath.Join(home, ".codebuddy", "skills")},
		{Name: "codemaker", DisplayName: "Codemaker", ProjectDir: ".codemaker/skills", GlobalDir: filepath.Join(home, ".codemaker", "skills")},
		{Name: "codestudio", DisplayName: "Code Studio", ProjectDir: ".codestudio/skills", GlobalDir: filepath.Join(home, ".codestudio", "skills")},
		{Name: "codex", DisplayName: "Codex", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(codexHome, "skills"), DetectionDirs: []string{codexHome, "/etc/codex"}},
		{Name: "command-code", DisplayName: "Command Code", ProjectDir: ".commandcode/skills", GlobalDir: filepath.Join(home, ".commandcode", "skills")},
		{Name: "continue", DisplayName: "Continue", ProjectDir: ".continue/skills", GlobalDir: filepath.Join(home, ".continue", "skills")},
		{Name: "cortex", DisplayName: "Cortex Code", ProjectDir: ".cortex/skills", GlobalDir: filepath.Join(home, ".snowflake", "cortex", "skills")},
		{Name: "crush", DisplayName: "Crush", ProjectDir: ".crush/skills", GlobalDir: filepath.Join(home, ".config", "crush", "skills")},
		{Name: "cursor", DisplayName: "Cursor", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".cursor", "skills"), DetectionDirs: []string{filepath.Join(home, ".cursor")}},
		{Name: "deepagents", DisplayName: "Deep Agents", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".deepagents", "agent", "skills"), DetectionDirs: []string{filepath.Join(home, ".deepagents")}},
		{Name: "devin", DisplayName: "Devin for Terminal", ProjectDir: ".devin/skills", GlobalDir: filepath.Join(configHome, "devin", "skills")},
		{Name: "dexto", DisplayName: "Dexto", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".agents", "skills"), DetectionDirs: []string{filepath.Join(home, ".dexto")}},
		{Name: "droid", DisplayName: "Droid", ProjectDir: ".factory/skills", GlobalDir: filepath.Join(home, ".factory", "skills")},
		{Name: "firebender", DisplayName: "Firebender", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".firebender", "skills"), DetectionDirs: []string{filepath.Join(home, ".firebender")}},
		{Name: "forgecode", DisplayName: "ForgeCode", ProjectDir: ".forge/skills", GlobalDir: filepath.Join(home, ".forge", "skills")},
		{Name: "gemini-cli", DisplayName: "Gemini CLI", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".gemini", "skills"), DetectionDirs: []string{filepath.Join(home, ".gemini")}},
		{Name: "github-copilot", DisplayName: "GitHub Copilot", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".copilot", "skills"), DetectionDirs: []string{filepath.Join(home, ".copilot")}},
		{Name: "goose", DisplayName: "Goose", ProjectDir: ".goose/skills", GlobalDir: filepath.Join(configHome, "goose", "skills")},
		{Name: "hermes-agent", DisplayName: "Hermes Agent", ProjectDir: ".hermes/skills", GlobalDir: filepath.Join(home, ".hermes", "skills")},
		{Name: "junie", DisplayName: "Junie", ProjectDir: ".junie/skills", GlobalDir: filepath.Join(home, ".junie", "skills")},
		{Name: "iflow-cli", DisplayName: "iFlow CLI", ProjectDir: ".iflow/skills", GlobalDir: filepath.Join(home, ".iflow", "skills")},
		{Name: "kilo", DisplayName: "Kilo Code", ProjectDir: ".kilocode/skills", GlobalDir: filepath.Join(home, ".kilocode", "skills")},
		{Name: "kimi-cli", DisplayName: "Kimi Code CLI", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".config", "agents", "skills"), DetectionDirs: []string{filepath.Join(home, ".kimi")}},
		{Name: "kiro-cli", DisplayName: "Kiro CLI", ProjectDir: ".kiro/skills", GlobalDir: filepath.Join(home, ".kiro", "skills")},
		{Name: "kode", DisplayName: "Kode", ProjectDir: ".kode/skills", GlobalDir: filepath.Join(home, ".kode", "skills")},
		{Name: "mcpjam", DisplayName: "MCPJam", ProjectDir: ".mcpjam/skills", GlobalDir: filepath.Join(home, ".mcpjam", "skills")},
		{Name: "mistral-vibe", DisplayName: "Mistral Vibe", ProjectDir: ".vibe/skills", GlobalDir: filepath.Join(vibeHome, "skills")},
		{Name: "mux", DisplayName: "Mux", ProjectDir: ".mux/skills", GlobalDir: filepath.Join(home, ".mux", "skills")},
		{Name: "opencode", DisplayName: "OpenCode", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(configHome, "opencode", "skills"), DetectionDirs: []string{filepath.Join(configHome, "opencode")}},
		{Name: "openhands", DisplayName: "OpenHands", ProjectDir: ".openhands/skills", GlobalDir: filepath.Join(home, ".openhands", "skills")},
		{Name: "pi", DisplayName: "Pi", ProjectDir: ".pi/skills", GlobalDir: filepath.Join(home, ".pi", "agent", "skills")},
		{Name: "qoder", DisplayName: "Qoder", ProjectDir: ".qoder/skills", GlobalDir: filepath.Join(home, ".qoder", "skills")},
		{Name: "qwen-code", DisplayName: "Qwen Code", ProjectDir: ".qwen/skills", GlobalDir: filepath.Join(home, ".qwen", "skills")},
		{Name: "replit", DisplayName: "Replit", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(configHome, "agents", "skills")},
		{Name: "rovodev", DisplayName: "Rovo Dev", ProjectDir: ".rovodev/skills", GlobalDir: filepath.Join(home, ".rovodev", "skills")},
		{Name: "roo", DisplayName: "Roo Code", ProjectDir: ".roo/skills", GlobalDir: filepath.Join(home, ".roo", "skills")},
		{Name: "tabnine-cli", DisplayName: "Tabnine CLI", ProjectDir: ".tabnine/agent/skills", GlobalDir: filepath.Join(home, ".tabnine", "agent", "skills")},
		{Name: "trae", DisplayName: "Trae", ProjectDir: ".trae/skills", GlobalDir: filepath.Join(home, ".trae", "skills")},
		{Name: "trae-cn", DisplayName: "Trae CN", ProjectDir: ".trae/skills", GlobalDir: filepath.Join(home, ".trae-cn", "skills")},
		{Name: "warp", DisplayName: "Warp", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(home, ".agents", "skills"), DetectionDirs: []string{filepath.Join(home, ".warp")}},
		{Name: "windsurf", DisplayName: "Windsurf", ProjectDir: ".windsurf/skills", GlobalDir: filepath.Join(home, ".codeium", "windsurf", "skills")},
		{Name: "zencoder", DisplayName: "Zencoder", ProjectDir: ".zencoder/skills", GlobalDir: filepath.Join(home, ".zencoder", "skills")},
		{Name: "neovate", DisplayName: "Neovate", ProjectDir: ".neovate/skills", GlobalDir: filepath.Join(home, ".neovate", "skills")},
		{Name: "pochi", DisplayName: "Pochi", ProjectDir: ".pochi/skills", GlobalDir: filepath.Join(home, ".pochi", "skills")},
		{Name: "adal", DisplayName: "AdaL", ProjectDir: ".adal/skills", GlobalDir: filepath.Join(home, ".adal", "skills")},
		{Name: "universal", DisplayName: "Universal", ProjectDir: ".agents/skills", GlobalDir: filepath.Join(configHome, "agents", "skills")},
	})
}

func AgentDefinitionByName(home string, env map[string]string, name string) (AgentDefinition, bool) {
	name = normalizeAgentSelector(name)
	for _, agent := range AgentDefinitions(home, env) {
		if agentSelectorMatches(name, agent.Name) || agentSelectorMatches(name, agent.DisplayName) {
			return agent, true
		}
	}
	return AgentDefinition{}, false
}

func AgentDefinitionsForSelectors(home string, env map[string]string, selectors []string) ([]AgentDefinition, error) {
	all := AgentDefinitions(home, env)
	var selected []AgentDefinition
	seen := map[string]bool{}
	add := func(agent AgentDefinition) {
		if seen[agent.Name] {
			return
		}
		seen[agent.Name] = true
		selected = append(selected, agent)
	}

	for _, selector := range selectors {
		selector = normalizeAgentSelector(selector)
		if selector == "" {
			continue
		}
		switch selector {
		case "*", "all":
			for _, agent := range all {
				add(agent)
			}
			continue
		case "universal", "shared", "agents":
			for _, agent := range all {
				if agent.UniversalProject {
					add(agent)
				}
			}
			continue
		}
		if agent, ok := AgentDefinitionByName(home, env, selector); ok {
			add(agent)
			continue
		}
		return nil, fail("Unsupported agent %q. Supported agents: %s", selector, supportedAgentList(all))
	}
	return selected, nil
}

func AgentInstallTargets(cwd string, home string, env map[string]string, scope string, selectors []string, slug string) ([]AgentTarget, error) {
	agents, err := AgentDefinitionsForSelectors(home, env, selectors)
	if err != nil {
		return nil, err
	}
	targets := make([]AgentTarget, 0, len(agents))
	for _, agent := range agents {
		if scope == scopeGlobal {
			target, ok := AgentGlobalTarget(agent, slug)
			if ok {
				targets = append(targets, target)
			}
			continue
		}
		targets = append(targets, AgentProjectTarget(cwd, agent, slug))
	}
	return targets, nil
}

func AgentProjectTarget(cwd string, agent AgentDefinition, slug string) AgentTarget {
	relativePath := agentRelativeSkillPath(agent.ProjectDir, slug)
	return AgentTarget{
		AgentName:        agent.Name,
		AgentDisplayName: agent.DisplayName,
		Scope:            scopeProject,
		RootDir:          cwd,
		RelativePath:     relativePath,
		InstallPath:      filepath.Join(cwd, filepath.FromSlash(relativePath)),
	}
}

func AgentGlobalTarget(agent AgentDefinition, slug string) (AgentTarget, bool) {
	if agent.GlobalDir == "" {
		return AgentTarget{}, false
	}
	return AgentTarget{
		AgentName:        agent.Name,
		AgentDisplayName: agent.DisplayName,
		Scope:            scopeGlobal,
		RootDir:          agent.GlobalDir,
		RelativePath:     slug,
		InstallPath:      filepath.Join(agent.GlobalDir, slug),
	}, true
}

func DetectedProjectUniversalAgents(cwd string, home string, env map[string]string) []string {
	var agents []string
	for _, agent := range AgentDefinitions(home, env) {
		if !agent.UniversalProject {
			continue
		}
		detectionDirs := agent.DetectionDirs
		if agent.Name == "replit" {
			detectionDirs = append(detectionDirs, filepath.Join(cwd, ".replit"))
		}
		for _, path := range detectionDirs {
			if directoryExists(path) {
				agents = append(agents, agent.DisplayName)
				break
			}
		}
	}
	sort.Strings(agents)
	return agents
}

func scanAgents(home string, env map[string]string) []AgentDefinition {
	return AgentDefinitions(home, env)
}

func detectedProjectUniversalAgents(cwd string, home string, env map[string]string) []string {
	return DetectedProjectUniversalAgents(cwd, home, env)
}

func adapterPath(agent, slug string) (string, error) {
	definition, ok := agentDefinitionForLocalUser(agent)
	if !ok || definition.ProjectDir == "" {
		return "", fail("Unsupported agent %q. Supported agents: %s", agent, supportedAgentListForLocalUser())
	}
	return agentRelativeSkillPath(definition.ProjectDir, slug), nil
}

func globalAgentRoot(agent string) (string, error) {
	definition, ok := agentDefinitionForLocalUser(agent)
	if !ok || definition.GlobalDir == "" {
		return "", fail("Unsupported agent %q. Supported agents: %s", agent, supportedAgentListForLocalUser())
	}
	return definition.GlobalDir, nil
}

func agentDisplayName(agent string) string {
	if definition, ok := agentDefinitionForLocalUser(agent); ok {
		return definition.DisplayName
	}
	return agent
}

func agentDefinitionForLocalUser(agent string) (AgentDefinition, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return AgentDefinitionByName(home, nil, agent)
}

func supportedAgentListForLocalUser() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return supportedAgentList(AgentDefinitions(home, nil))
}

func supportedAgentList(agents []AgentDefinition) string {
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func markUniversalProjectAgents(agents []AgentDefinition) []AgentDefinition {
	for index := range agents {
		agents[index].UniversalProject = filepath.ToSlash(filepath.Clean(agents[index].ProjectDir)) == ".agents/skills"
	}
	return agents
}

func agentRelativeSkillPath(root string, slug string) string {
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(root), slug))
}

func normalizeAgentSelector(agent string) string {
	return strings.ToLower(strings.TrimSpace(agent))
}

func agentSelectorMatches(selector string, agent string) bool {
	agent = strings.ToLower(strings.TrimSpace(agent))
	return selector == agent || selector == slugify(agent)
}
