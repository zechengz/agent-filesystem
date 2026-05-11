package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/redis/agent-filesystem/internal/controlplane"
)

type workspaceCompositionControlPlane interface {
	CreateWorkspaceComposition(context.Context, controlplane.CreateWorkspaceCompositionRequest) (controlplane.WorkspaceCompositionDetail, error)
	ListWorkspaceCompositions(context.Context) (controlplane.WorkspaceCompositionListResponse, error)
	GetWorkspaceComposition(context.Context, string) (controlplane.WorkspaceCompositionDetail, error)
	AddWorkspaceCompositionMount(context.Context, string, controlplane.WorkspaceCompositionMount) (controlplane.WorkspaceCompositionDetail, error)
	RemoveWorkspaceCompositionMount(context.Context, string, string) (controlplane.WorkspaceCompositionDetail, error)
	CreateWorkspaceBookmark(context.Context, string, controlplane.CreateWorkspaceBookmarkRequest) (controlplane.WorkspaceBookmark, error)
	RestoreWorkspaceBookmark(context.Context, string, string) (controlplane.WorkspaceBookmark, error)
}

func openWorkspaceCompositionControlPlane(ctx context.Context) (workspaceCompositionControlPlane, func(), error) {
	cfg, err := loadAFSConfig()
	if err != nil {
		return nil, nil, err
	}
	_, service, closeStore, err := openAFSControlPlaneForConfig(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	composer, ok := service.(workspaceCompositionControlPlane)
	if !ok {
		closeStore()
		return nil, nil, fmt.Errorf("workspace composition API is unavailable")
	}
	return composer, closeStore, nil
}

func cmdRootMountArgs(args []string) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		fmt.Fprint(os.Stderr, workspaceCompositionMountUsageFor(filepath.Base(os.Args[0])+" mount"))
		return nil
	}
	opts, err := parseMountOptions(args)
	if err != nil {
		return err
	}
	return mountWorkspaceComposition(opts)
}

func cmdRootUnmountArgs(args []string) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		fmt.Fprint(os.Stderr, workspaceCompositionUnmountUsageFor(filepath.Base(os.Args[0])+" unmount"))
		return nil
	}
	opts, err := parseUnmountOptions(args, false)
	if err != nil {
		return err
	}
	return unmountWorkspaceComposition(opts)
}

func cmdWorkspaceManifestCreate(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceManifestCreateUsage(filepath.Base(os.Args[0])))
		return nil
	}
	opts, err := parseWorkspaceManifestCreateArgs(args[2:])
	if err != nil {
		return err
	}
	if opts.name == "" {
		return fmt.Errorf("%s", workspaceManifestCreateUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.CreateWorkspaceComposition(ctx, controlplane.CreateWorkspaceCompositionRequest{
		Name:        opts.name,
		Description: opts.description,
	})
	if err != nil {
		return err
	}
	if opts.jsonOut {
		return writeJSON(detail)
	}
	printSection(markerSuccess+" "+clr(ansiBold, "workspace manifest created"), []outputRow{
		{Label: "workspace", Value: detail.Name},
		{Label: "id", Value: detail.ID},
		{Label: "mounts", Value: fmt.Sprintf("%d", len(detail.Mounts))},
	})
	return nil
}

func cmdWorkspaceManifestList(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceManifestListUsage(filepath.Base(os.Args[0])))
		return nil
	}
	jsonOut, err := parseOnlyJSONFlag(args[2:])
	if err != nil {
		return err
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	response, err := service.ListWorkspaceCompositions(ctx)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(response)
	}
	if len(response.Items) == 0 {
		fmt.Fprintln(os.Stdout, "No workspace manifests found.")
		return nil
	}
	fmt.Fprintf(os.Stdout, "%-28s  %-6s  %s\n", "Workspace", "Mounts", "Updated")
	for _, item := range response.Items {
		fmt.Fprintf(os.Stdout, "%-28s  %-6d  %s\n", item.Name, item.MountCount, item.UpdatedAt)
	}
	return nil
}

func cmdWorkspaceManifestShow(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceManifestShowUsage(filepath.Base(os.Args[0])))
		return nil
	}
	rest, jsonOut, err := parseJSONFlagWithPositionals(args[2:])
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("%s", workspaceManifestShowUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.GetWorkspaceComposition(ctx, rest[0])
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(detail)
	}
	printWorkspaceCompositionDetail(detail)
	return nil
}

func cmdWorkspaceMountVolume(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceMountVolumeUsage(filepath.Base(os.Args[0])))
		return nil
	}
	opts, err := parseWorkspaceMountVolumeArgs(args[2:])
	if err != nil {
		return err
	}
	if opts.workspace == "" || opts.volume == "" {
		return fmt.Errorf("%s", workspaceMountVolumeUsage(filepath.Base(os.Args[0])))
	}
	if opts.mountPath == "" {
		opts.mountPath = "/"
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.AddWorkspaceCompositionMount(ctx, opts.workspace, controlplane.WorkspaceCompositionMount{
		VolumeID:      opts.volume,
		MountPath:     opts.mountPath,
		Readonly:      opts.readonly,
		VolumeTokenID: opts.volumeTokenID,
	})
	if err != nil {
		return err
	}
	if opts.jsonOut {
		return writeJSON(detail)
	}
	printSection(markerSuccess+" "+clr(ansiBold, "volume mounted"), []outputRow{
		{Label: "workspace", Value: detail.Name},
		{Label: "volume", Value: opts.volume},
		{Label: "path", Value: opts.mountPath},
	})
	return nil
}

func cmdWorkspaceUnmountVolume(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceUnmountVolumeUsage(filepath.Base(os.Args[0])))
		return nil
	}
	rest, jsonOut, err := parseJSONFlagWithPositionals(args[2:])
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("%s", workspaceUnmountVolumeUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.RemoveWorkspaceCompositionMount(ctx, rest[0], rest[1])
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(detail)
	}
	printSection(markerSuccess+" "+clr(ansiBold, "volume unmounted"), []outputRow{
		{Label: "workspace", Value: detail.Name},
		{Label: "volume", Value: rest[1]},
	})
	return nil
}

func cmdWorkspaceBookmark(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceBookmarkUsage(filepath.Base(os.Args[0])))
		return nil
	}
	opts, err := parseWorkspaceBookmarkArgs(args[2:])
	if err != nil {
		return err
	}
	if opts.workspace == "" || opts.name == "" {
		return fmt.Errorf("%s", workspaceBookmarkUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	bookmark, err := service.CreateWorkspaceBookmark(ctx, opts.workspace, controlplane.CreateWorkspaceBookmarkRequest{
		Name:        opts.name,
		Description: opts.description,
	})
	if err != nil {
		return err
	}
	if opts.jsonOut {
		return writeJSON(bookmark)
	}
	printSection(markerSuccess+" "+clr(ansiBold, "workspace bookmark created"), []outputRow{
		{Label: "workspace", Value: bookmark.WorkspaceID},
		{Label: "bookmark", Value: bookmark.Name},
		{Label: "volumes", Value: fmt.Sprintf("%d", len(bookmark.Volumes))},
	})
	return nil
}

func cmdWorkspaceRestoreBookmark(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceRestoreBookmarkUsage(filepath.Base(os.Args[0])))
		return nil
	}
	rest, jsonOut, err := parseJSONFlagWithPositionals(args[2:])
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("%s", workspaceRestoreBookmarkUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	bookmark, err := service.RestoreWorkspaceBookmark(ctx, rest[0], rest[1])
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(bookmark)
	}
	printSection(markerSuccess+" "+clr(ansiBold, "workspace bookmark restored"), []outputRow{
		{Label: "workspace", Value: bookmark.WorkspaceID},
		{Label: "bookmark", Value: bookmark.Name},
		{Label: "volumes", Value: fmt.Sprintf("%d", len(bookmark.Volumes))},
	})
	return nil
}

func cmdWorkspaceBookmarkCommand(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceBookmarkUsage(filepath.Base(os.Args[0])))
		return nil
	}
	if len(args) > 2 {
		switch args[2] {
		case "create":
			return cmdWorkspaceBookmark(append([]string{args[0], args[1]}, args[3:]...))
		case "list":
			return cmdWorkspaceBookmarkList(append([]string{args[0], args[1]}, args[3:]...))
		case "restore":
			return cmdWorkspaceRestoreBookmark(append([]string{args[0], args[1]}, args[3:]...))
		}
	}
	return cmdWorkspaceBookmark(args)
}

func cmdWorkspaceBookmarkList(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceBookmarkListUsage(filepath.Base(os.Args[0])))
		return nil
	}
	rest, jsonOut, err := parseJSONFlagWithPositionals(args[2:])
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("%s", workspaceBookmarkListUsage(filepath.Base(os.Args[0])))
	}
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.GetWorkspaceComposition(ctx, rest[0])
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(detail.Bookmarks)
	}
	if len(detail.Bookmarks) == 0 {
		fmt.Fprintln(os.Stdout, "No workspace bookmarks found.")
		return nil
	}
	fmt.Fprintf(os.Stdout, "%-28s  %-7s  %s\n", "Bookmark", "Volumes", "Created")
	for _, bookmark := range detail.Bookmarks {
		fmt.Fprintf(os.Stdout, "%-28s  %-7d  %s\n", bookmark.Name, len(bookmark.Volumes), bookmark.CreatedAt)
	}
	return nil
}

func cmdWorkspaceCompositionMount(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceCompositionMountUsage(filepath.Base(os.Args[0])))
		return nil
	}
	opts, err := parseMountOptions(args[2:])
	if err != nil {
		return err
	}
	return mountWorkspaceComposition(opts)
}

func mountWorkspaceComposition(opts mountOptions) error {
	if strings.TrimSpace(opts.workspace) == "" {
		return promptWorkspaceCompositionMountSelection(opts)
	}

	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()
	detail, err := service.GetWorkspaceComposition(ctx, opts.workspace)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workspaceCompositionNotFoundError(opts.workspace)
		}
		return err
	}
	if len(detail.Mounts) == 0 {
		return fmt.Errorf("workspace %s has no mounted volumes", detail.Name)
	}
	if strings.TrimSpace(opts.directory) == "" {
		directory, ok, err := promptMountPathForWorkspace(detail.Name)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		opts.directory = directory
	}
	rootPath, err := normalizeMountPath(opts.directory)
	if err != nil {
		return err
	}

	resultRows := make([]workspaceCompositionMountResultRow, 0, len(detail.Mounts))
	for _, mount := range detail.Mounts {
		localPath := workspaceCompositionLocalMountPath(rootPath, mount.MountPath)
		sessionName := workspaceCompositionSessionName(opts.sessionName, detail.Name, mount.MountPath)
		volumeName := mount.VolumeName
		if strings.TrimSpace(volumeName) == "" {
			volumeName = mount.VolumeID
		}
		if err := mountWorkspace(mountOptions{
			workspace:   mount.VolumeID,
			directory:   localPath,
			sessionName: sessionName,
			verbose:     opts.verbose,
			dryRun:      opts.dryRun,
			yes:         opts.yes,
			readonly:    opts.readonly || mount.Readonly,
			quiet:       true,
		}); err != nil {
			return fmt.Errorf("mount volume %s at %s: %w", volumeName, mount.MountPath, err)
		}
		if !opts.dryRun {
			if err := tagWorkspaceCompositionMount(localPath, rootPath, detail, mount); err != nil {
				return err
			}
		}
		resultRows = append(resultRows, workspaceCompositionMountResultRow{
			Path:      homeRelativeDisplayPath(localPath),
			Mode:      workspaceCompositionSummaryPermission(opts.readonly || mount.Readonly),
			FileCount: workspaceCompositionMountedFileCount(volumeName),
			HasCount:  !opts.dryRun,
		})
	}
	printWorkspaceCompositionMountResult(detail.Name, rootPath, workspaceCompositionMountResultMode(opts), resultRows)
	return nil
}

type workspaceCompositionMountResultRow struct {
	Path      string
	Mode      string
	FileCount int
	HasCount  bool
}

func workspaceCompositionSummaryPermission(readonly bool) string {
	if readonly {
		return "read-only"
	}
	return "read-write"
}

func workspaceCompositionMountResultMode(opts mountOptions) string {
	if opts.dryRun {
		return "dry-run"
	}
	cfg, err := loadAFSConfig()
	if err == nil {
		mode, err := effectiveMode(cfg)
		if err == nil && strings.TrimSpace(mode) != "" {
			return mode
		}
	}
	return modeSync
}

func workspaceCompositionMountedFileCount(volumeName string) int {
	st, err := loadSyncState(volumeName)
	if err != nil || st == nil {
		return 0
	}
	count := 0
	for _, entry := range st.Entries {
		if entry.Deleted || entry.Type != "file" {
			continue
		}
		count++
	}
	return count
}

func printWorkspaceCompositionMountResult(workspace, rootPath, mode string, rows []workspaceCompositionMountResultRow) {
	printSection("Workspace Mounted", []outputRow{
		{Label: "workspace", Value: workspace},
		{Label: "path", Value: homeRelativeDisplayPath(rootPath)},
		{Label: "mode", Value: mode},
	})
	fmt.Println("Volumes:")
	fmt.Println()
	printWorkspaceCompositionMountRows(rows)
	fmt.Println()
	fmt.Printf("unmount  %s ws unmount %s\n", filepath.Base(os.Args[0]), workspace)
	fmt.Println()
}

func printWorkspaceCompositionMountRows(rows []workspaceCompositionMountResultRow) {
	if len(rows) == 0 {
		fmt.Println("none")
		return
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		files := "files unavailable"
		if row.HasCount {
			files = workspaceCompositionFileCountLabel(row.FileCount)
		}
		tableRows = append(tableRows, []string{row.Path, row.Mode, files})
	}
	printPlainRows(tableRows)
}

func printPlainRows(rows [][]string) {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return
	}
	widths := make([]int, maxCols)
	for _, row := range rows {
		for i, value := range row {
			if width := runeWidth(value); width > widths[i] {
				widths[i] = width
			}
		}
	}
	for _, row := range rows {
		printPlainTableRow(row, widths)
	}
}

func workspaceCompositionFileCountLabel(count int) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}

type workspaceCompositionPromptChoice struct {
	Name       string
	ID         string
	Mounts     int
	Root       string
	Mounted    bool
	VolumeRows int
}

func promptWorkspaceCompositionMountSelection(opts mountOptions) error {
	ctx := context.Background()
	service, closeStore, err := openWorkspaceCompositionControlPlane(ctx)
	if err != nil {
		return err
	}
	defer closeStore()

	response, err := service.ListWorkspaceCompositions(ctx)
	if err != nil {
		return err
	}
	reg, err := loadMountRegistry()
	if err != nil {
		return err
	}
	choices := workspaceCompositionMountChoices(reg, response.Items)
	if len(choices) == 0 {
		fmt.Println()
		fmt.Println("Mount Agent Workspace")
		fmt.Println()
		fmt.Println("No Agent Workspaces found.")
		fmt.Println("Create one with: " + filepath.Base(os.Args[0]) + " ws create <workspace>")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("Mount Agent Workspace")
	fmt.Println()
	headers := []string{"#", "Workspace", "Volumes", "Status", "Path"}
	printPlainTable(headers, workspaceCompositionPromptRows(choices))
	fmt.Println()
	fmt.Print("Workspace to mount: ")

	reader := bufio.NewReader(os.Stdin)
	raw, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(raw) == "" {
		fmt.Println()
		fmt.Println("Mount cancelled.")
		fmt.Println()
		return nil
	}
	choiceText := strings.TrimSpace(raw)
	if choiceText == "" {
		fmt.Println("Mount cancelled.")
		fmt.Println()
		return nil
	}
	idx, err := strconv.Atoi(choiceText)
	if err != nil || idx < 1 || idx > len(choices) {
		return fmt.Errorf("invalid selection %q", choiceText)
	}
	selected := choices[idx-1]
	if selected.Mounted {
		printSection("Agent Workspace already mounted", []outputRow{
			{Label: "workspace", Value: selected.Name},
			{Label: "path", Value: homeRelativeDisplayPath(selected.Root)},
		})
		return nil
	}
	opts.workspace = selected.Name
	if strings.TrimSpace(selected.ID) != "" {
		opts.workspace = selected.ID
	}
	defaultPath := "~/" + selected.Name
	fmt.Println()
	fmt.Printf("Local folder [%s]: ", defaultPath)
	rawPath, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(rawPath) == "" {
		fmt.Println()
		fmt.Println("Mount cancelled.")
		fmt.Println()
		return nil
	}
	localPath := strings.TrimSpace(rawPath)
	if localPath == "" {
		localPath = defaultPath
	}
	opts.directory = localPath
	return mountWorkspaceComposition(opts)
}

func workspaceCompositionMountChoices(reg mountRegistry, items []controlplane.WorkspaceCompositionSummary) []workspaceCompositionPromptChoice {
	roots := workspaceCompositionMountedRoots(reg)
	rootByID := make(map[string]workspaceCompositionMountedRoot, len(roots))
	rootByName := make(map[string]workspaceCompositionMountedRoot, len(roots))
	for _, root := range roots {
		if id := strings.TrimSpace(root.WorkspaceID); id != "" {
			rootByID[id] = root
		}
		if name := strings.TrimSpace(root.WorkspaceName); name != "" {
			rootByName[name] = root
		}
	}
	choices := make([]workspaceCompositionPromptChoice, 0, len(items))
	for _, item := range items {
		choice := workspaceCompositionPromptChoice{
			Name:   item.Name,
			ID:     item.ID,
			Mounts: item.MountCount,
		}
		if root, ok := rootByID[item.ID]; ok {
			choice.Root = root.Root
			choice.Mounted = true
			choice.VolumeRows = root.VolumeRows
		} else if root, ok := rootByName[item.Name]; ok {
			choice.Root = root.Root
			choice.Mounted = true
			choice.VolumeRows = root.VolumeRows
		}
		choices = append(choices, choice)
	}
	sort.Slice(choices, func(i, j int) bool {
		return strings.ToLower(choices[i].Name) < strings.ToLower(choices[j].Name)
	})
	return choices
}

func workspaceCompositionPromptRows(choices []workspaceCompositionPromptChoice) [][]string {
	rows := make([][]string, 0, len(choices))
	for i, choice := range choices {
		status := "available"
		path := ""
		if choice.Mounted {
			status = "mounted"
			path = homeRelativeDisplayPath(choice.Root)
		}
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			choice.Name,
			fmt.Sprintf("%d", choice.Mounts),
			status,
			path,
		})
	}
	return rows
}

func tagWorkspaceCompositionMount(localPath, rootPath string, detail controlplane.WorkspaceCompositionDetail, mount controlplane.WorkspaceCompositionMount) error {
	normalizedLocal, err := normalizeMountPath(localPath)
	if err != nil {
		return err
	}
	normalizedRoot, err := normalizeMountPath(rootPath)
	if err != nil {
		return err
	}
	reg, err := loadMountRegistry()
	if err != nil {
		return err
	}
	for i := range reg.Mounts {
		if filepath.Clean(reg.Mounts[i].LocalPath) != normalizedLocal {
			continue
		}
		reg.Mounts[i].AgentWorkspace = detail.Name
		reg.Mounts[i].AgentWorkspaceID = detail.ID
		reg.Mounts[i].AgentWorkspaceRoot = normalizedRoot
		reg.Mounts[i].AgentWorkspacePath = mount.MountPath
		return saveMountRegistry(reg)
	}
	return fmt.Errorf("mounted volume record for %s was not registered", normalizedLocal)
}

func workspaceCompositionNotFoundError(workspace string) error {
	ref := strings.TrimSpace(workspace)
	if ref == "" {
		ref = "<workspace>"
	}
	bin := filepath.Base(os.Args[0])
	return fmt.Errorf("Agent Workspace %q does not exist\n\nRun '%s ws list' to see Agent Workspaces, or create one with '%s ws create <workspace>'.", ref, bin, bin)
}

func cmdWorkspaceCompositionUnmount(args []string) error {
	if len(args) > 2 && isHelpArg(args[2]) {
		fmt.Fprint(os.Stderr, workspaceCompositionUnmountUsage(filepath.Base(os.Args[0])))
		return nil
	}
	opts, err := parseUnmountOptions(args[2:], false)
	if err != nil {
		return err
	}
	return unmountWorkspaceComposition(opts)
}

func unmountWorkspaceComposition(opts unmountOptions) error {
	if strings.TrimSpace(opts.target) == "" {
		return promptWorkspaceCompositionUnmountSelection(opts.deleteLocal)
	}
	if !unmountTargetLooksLikePath(opts.target) {
		return unmountWorkspaceCompositionByRef(opts.target, opts.deleteLocal)
	}
	return unmountWorkspaceCompositionRoot(opts.target, opts.deleteLocal)
}

func unmountWorkspaceCompositionRoot(target string, deleteLocal bool) error {
	root, err := normalizeMountPath(target)
	if err != nil {
		return err
	}
	reg, err := loadMountRegistry()
	if err != nil {
		return err
	}
	matches := make([]mountRecord, 0)
	for _, rec := range sortedMountRecords(reg.Mounts) {
		if mountRecordUnderRoot(rec, root) {
			matches = append(matches, rec)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no volume mounts found under %s", root)
	}
	workspaceName := workspaceCompositionUnmountName(matches, root)
	for _, match := range matches {
		rec, ok := removeMountByPath(&reg, match.LocalPath)
		if !ok {
			continue
		}
		if err := stopMount(rec, deleteLocal); err != nil {
			return err
		}
	}
	if deleteLocal {
		if err := os.RemoveAll(root); err != nil {
			return err
		}
	}
	if err := saveMountRegistry(reg); err != nil {
		return err
	}
	printWorkspaceCompositionUnmountResult(workspaceName, root, len(matches), deleteLocal)
	return nil
}

func workspaceCompositionUnmountName(records []mountRecord, root string) string {
	for _, rec := range records {
		if name := strings.TrimSpace(rec.AgentWorkspace); name != "" {
			return name
		}
	}
	base := filepath.Base(filepath.Clean(root))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return filepath.Clean(root)
	}
	return base
}

func printWorkspaceCompositionUnmountResult(workspace, root string, volumeCount int, deleteLocal bool) {
	local := "preserved"
	if deleteLocal {
		local = "deleted"
	}
	printSection("Agent Workspace unmounted", []outputRow{
		{Label: "workspace", Value: workspace},
		{Label: "path", Value: homeRelativeDisplayPath(root)},
		{Label: "volumes", Value: fmt.Sprintf("%d", volumeCount)},
		{Label: "local", Value: local},
	})
}

func unmountWorkspaceCompositionByRef(ref string, deleteLocal bool) error {
	reg, err := loadMountRegistry()
	if err != nil {
		return err
	}
	matches := workspaceCompositionMountedRootsForRef(reg, ref)
	switch len(matches) {
	case 0:
		return fmt.Errorf("no mounted Agent Workspace found for %s", ref)
	case 1:
		return unmountWorkspaceCompositionRoot(matches[0].Root, deleteLocal)
	default:
		paths := make([]string, 0, len(matches))
		for _, match := range matches {
			paths = append(paths, homeRelativeDisplayPath(match.Root))
		}
		return fmt.Errorf("Agent Workspace %s matches multiple mounted roots: %s\nRun '%s unmount <directory>' to choose one.", ref, strings.Join(paths, ", "), filepath.Base(os.Args[0]))
	}
}

func promptWorkspaceCompositionUnmountSelection(deleteLocal bool) error {
	reg, err := loadMountRegistry()
	if err != nil {
		return err
	}
	roots := workspaceCompositionMountedRoots(reg)
	if len(roots) == 0 {
		fmt.Println()
		fmt.Println("No mounted Agent Workspaces.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("Unmount Agent Workspace")
	fmt.Println()
	headers := []string{"#", "Workspace", "Volumes", "Path"}
	printPlainTable(headers, workspaceCompositionUnmountPromptRows(roots))
	fmt.Println()
	fmt.Print("Workspace to unmount: ")

	reader := bufio.NewReader(os.Stdin)
	raw, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(raw) == "" {
		fmt.Println()
		fmt.Println("Unmount cancelled.")
		fmt.Println()
		return nil
	}
	choice := strings.TrimSpace(raw)
	if choice == "" {
		fmt.Println("Unmount cancelled.")
		fmt.Println()
		return nil
	}
	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(roots) {
		return fmt.Errorf("invalid selection %q", choice)
	}
	return unmountWorkspaceCompositionRoot(roots[idx-1].Root, deleteLocal)
}

type workspaceCompositionMountedRoot struct {
	WorkspaceName string
	WorkspaceID   string
	Root          string
	VolumeRows    int
}

func workspaceCompositionMountedRoots(reg mountRegistry) []workspaceCompositionMountedRoot {
	byKey := make(map[string]*workspaceCompositionMountedRoot)
	for _, rec := range sortedMountRecords(reg.Mounts) {
		root := strings.TrimSpace(rec.AgentWorkspaceRoot)
		name := strings.TrimSpace(rec.AgentWorkspace)
		id := strings.TrimSpace(rec.AgentWorkspaceID)
		if root == "" || (name == "" && id == "") {
			continue
		}
		key := id + "\x00" + name + "\x00" + filepath.Clean(root)
		item, ok := byKey[key]
		if !ok {
			item = &workspaceCompositionMountedRoot{
				WorkspaceName: name,
				WorkspaceID:   id,
				Root:          filepath.Clean(root),
			}
			byKey[key] = item
		}
		item.VolumeRows++
	}
	roots := make([]workspaceCompositionMountedRoot, 0, len(byKey))
	for _, item := range byKey {
		roots = append(roots, *item)
	}
	sort.Slice(roots, func(i, j int) bool {
		left := strings.ToLower(roots[i].WorkspaceName)
		right := strings.ToLower(roots[j].WorkspaceName)
		if left == right {
			return roots[i].Root < roots[j].Root
		}
		return left < right
	})
	return roots
}

func workspaceCompositionMountedRootsForRef(reg mountRegistry, ref string) []workspaceCompositionMountedRoot {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	matches := make([]workspaceCompositionMountedRoot, 0)
	for _, root := range workspaceCompositionMountedRoots(reg) {
		if root.WorkspaceName == ref || root.WorkspaceID == ref {
			matches = append(matches, root)
		}
	}
	return matches
}

func workspaceCompositionUnmountPromptRows(roots []workspaceCompositionMountedRoot) [][]string {
	rows := make([][]string, 0, len(roots))
	for i, root := range roots {
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			root.WorkspaceName,
			fmt.Sprintf("%d", root.VolumeRows),
			homeRelativeDisplayPath(root.Root),
		})
	}
	return rows
}

func workspaceCompositionLocalMountPath(root, mountPath string) string {
	clean := strings.TrimSpace(mountPath)
	if clean == "" || clean == "/" {
		return root
	}
	return filepath.Join(root, strings.TrimPrefix(clean, "/"))
}

func workspaceCompositionSessionName(prefix, workspaceName, mountPath string) string {
	base := strings.TrimSpace(prefix)
	if base == "" {
		base = strings.TrimSpace(workspaceName)
	}
	path := strings.TrimSpace(mountPath)
	if path == "" {
		path = "/"
	}
	if base == "" {
		return path
	}
	return base + " " + path
}

func mountRecordUnderRoot(rec mountRecord, root string) bool {
	path := strings.TrimSpace(rec.LocalPath)
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if abs == root {
		return true
	}
	rel, err := filepath.Rel(root, abs)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

type workspaceManifestCreateArgs struct {
	name        string
	description string
	jsonOut     bool
}

func parseWorkspaceManifestCreateArgs(args []string) (workspaceManifestCreateArgs, error) {
	var opts workspaceManifestCreateArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.jsonOut = true
		case arg == "--description":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --description")
			}
			i++
			opts.description = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--description="):
			opts.description = strings.TrimSpace(strings.TrimPrefix(arg, "--description="))
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %q", arg)
		default:
			if opts.name != "" {
				return opts, fmt.Errorf("unexpected argument %q", arg)
			}
			opts.name = arg
		}
	}
	return opts, nil
}

type workspaceMountVolumeArgs struct {
	workspace     string
	volume        string
	mountPath     string
	readonly      bool
	volumeTokenID string
	jsonOut       bool
}

func parseWorkspaceMountVolumeArgs(args []string) (workspaceMountVolumeArgs, error) {
	var opts workspaceMountVolumeArgs
	positionals := make([]string, 0, 3)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.jsonOut = true
		case arg == "--readonly" || arg == "--read-only":
			opts.readonly = true
		case arg == "--at":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --at")
			}
			i++
			opts.mountPath = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--at="):
			opts.mountPath = strings.TrimSpace(strings.TrimPrefix(arg, "--at="))
		case arg == "--token":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --token")
			}
			i++
			opts.volumeTokenID = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--token="):
			opts.volumeTokenID = strings.TrimSpace(strings.TrimPrefix(arg, "--token="))
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 3 {
		return opts, fmt.Errorf("too many arguments")
	}
	if len(positionals) > 0 {
		opts.workspace = positionals[0]
	}
	if len(positionals) > 1 {
		opts.volume = positionals[1]
	}
	if len(positionals) > 2 {
		opts.mountPath = positionals[2]
	}
	return opts, nil
}

type workspaceBookmarkArgs struct {
	workspace   string
	name        string
	description string
	jsonOut     bool
}

func parseWorkspaceBookmarkArgs(args []string) (workspaceBookmarkArgs, error) {
	var opts workspaceBookmarkArgs
	positionals := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.jsonOut = true
		case arg == "--description":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --description")
			}
			i++
			opts.description = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--description="):
			opts.description = strings.TrimSpace(strings.TrimPrefix(arg, "--description="))
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 2 {
		return opts, fmt.Errorf("too many arguments")
	}
	if len(positionals) > 0 {
		opts.workspace = positionals[0]
	}
	if len(positionals) > 1 {
		opts.name = positionals[1]
	}
	return opts, nil
}

func parseOnlyJSONFlag(args []string) (bool, error) {
	jsonOut := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		return false, fmt.Errorf("unknown flag %q", arg)
	}
	return jsonOut, nil
}

func parseJSONFlagWithPositionals(args []string) ([]string, bool, error) {
	jsonOut := false
	positionals := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return nil, false, fmt.Errorf("unknown flag %q", arg)
		}
		positionals = append(positionals, arg)
	}
	return positionals, jsonOut, nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printWorkspaceCompositionDetail(detail controlplane.WorkspaceCompositionDetail) {
	printSection(clr(ansiBold, "workspace manifest"), []outputRow{
		{Label: "workspace", Value: detail.Name},
		{Label: "id", Value: detail.ID},
		{Label: "mounts", Value: fmt.Sprintf("%d", len(detail.Mounts))},
		{Label: "bookmarks", Value: fmt.Sprintf("%d", len(detail.Bookmarks))},
	})
	if len(detail.Mounts) > 0 {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintf(os.Stdout, "%-28s  %-18s  %s\n", "Volume", "Mode", "Path")
		for _, mount := range detail.Mounts {
			mode := "rw"
			if mount.Readonly {
				mode = "ro"
			}
			name := mount.VolumeName
			if strings.TrimSpace(name) == "" {
				name = mount.VolumeID
			}
			fmt.Fprintf(os.Stdout, "%-28s  %-18s  %s\n", name, mode, mount.MountPath)
		}
	}
}

func workspaceManifestCreateUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws create [--description <text>] [--json] <workspace>

Create an Agent Workspace manifest that can mount one or more volumes.
`, bin)
}

func workspaceManifestListUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws list [--json]

List Agent Workspace manifests.
`, bin)
}

func workspaceManifestShowUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws show [--json] <workspace>

Show mounted volumes and bookmarks for an Agent Workspace.
`, bin)
}

func workspaceMountVolumeUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws add [--at <mount-path>] [--readonly] [--token <volume-token>] [--json] <workspace> <volume>

Add a volume to an Agent Workspace manifest. The default mount path is /.
`, bin)
}

func workspaceUnmountVolumeUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws remove [--json] <workspace> <volume>

Remove a volume from an Agent Workspace manifest.
`, bin)
}

func workspaceBookmarkUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws bookmark create [--description <text>] [--json] <workspace> <name>
  %s ws bookmark list [--json] <workspace>
  %s ws bookmark restore [--json] <workspace> <name>

Capture each mounted volume's current checkpoint.
`, bin, bin, bin)
}

func workspaceBookmarkListUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws bookmark list [--json] <workspace>

List Agent Workspace bookmarks.
`, bin)
}

func workspaceRestoreBookmarkUsage(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s ws bookmark restore [--json] <workspace> <name>

Restore mounted volumes to the checkpoints captured by a bookmark.
`, bin)
}

func workspaceCompositionMountUsage(bin string) string {
	return workspaceCompositionMountUsageFor(bin + " ws mount")
}

func workspaceCompositionMountUsageFor(command string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s [--dry-run] [--yes] [--readonly] [--verbose] [--session <name>] [<workspace> [directory]]

Mount every volume from an Agent Workspace manifest under the local directory.
Each manifest mount path becomes a mounted volume directory under that root.
With no workspace, lists Agent Workspaces and prompts for a selection.
With no directory, prompts for a local folder.
`, command)
}

func workspaceCompositionUnmountUsage(bin string) string {
	return workspaceCompositionUnmountUsageFor(bin + " ws unmount")
}

func workspaceCompositionUnmountUsageFor(command string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s [--delete] [<workspace|workspace-root-directory>]

Unmount volume mounts under a local Agent Workspace root directory.
With no target, lists mounted Agent Workspaces and prompts for a selection.
`, command)
}
