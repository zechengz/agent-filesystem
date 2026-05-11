package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func cmdTokens(args []string) error {
	if len(args) < 2 || isHelpArg(args[1]) {
		fmt.Fprint(os.Stderr, tokensUsageText(filepath.Base(os.Args[0])))
		return nil
	}
	switch args[1] {
	case "help":
		if len(args) > 2 && args[2] == "create" {
			fmt.Fprint(os.Stderr, tokenCreateUsageText(filepath.Base(os.Args[0])))
			return nil
		}
		fmt.Fprint(os.Stderr, tokensUsageText(filepath.Base(os.Args[0])))
		return nil
	case "create":
		return cmdTokenCreate(args[2:])
	default:
		return fmt.Errorf("unknown tokens command %q\n\n%s", args[1], tokensUsageText(filepath.Base(os.Args[0])))
	}
}

func cmdTokenCreate(args []string) error {
	for _, arg := range args {
		if isHelpArg(arg) {
			fmt.Fprint(os.Stderr, tokenCreateUsageText(filepath.Base(os.Args[0])))
			return nil
		}
	}

	fs := flag.NewFlagSet("tokens create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var workspace optionalString
	var name string
	var use string
	var permission string
	var expires string
	fs.Var(&workspace, "workspace", "workspace id or name")
	fs.StringVar(&name, "name", "", "token label")
	fs.StringVar(&use, "use", "mount", "token use (mount)")
	fs.StringVar(&permission, "permission", "rw", "mount permission: ro or rw")
	fs.StringVar(&expires, "expires", "30d", "expiry duration, RFC3339 timestamp, or never")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s", tokenCreateUsageText(filepath.Base(os.Args[0])))
	}
	positionals := fs.Args()
	if workspace.value == "" && len(positionals) == 1 {
		workspace.value = positionals[0]
		positionals = nil
	}
	if len(positionals) != 0 {
		return fmt.Errorf("%s", tokenCreateUsageText(filepath.Base(os.Args[0])))
	}
	workspaceRef := strings.TrimSpace(workspace.value)
	if workspaceRef == "" {
		return fmt.Errorf("--workspace is required")
	}
	if strings.TrimSpace(use) != "" && strings.TrimSpace(use) != "mount" {
		return fmt.Errorf("unsupported token use %q (expected mount)", use)
	}
	capability, err := mountTokenCapability(permission)
	if err != nil {
		return err
	}
	expiresAt, err := tokenExpiryTimestamp(expires)
	if err != nil {
		return err
	}

	cfg, err := loadAFSConfig()
	if err != nil {
		return err
	}
	productMode, err := effectiveProductMode(cfg)
	if err != nil {
		return err
	}
	if productMode == productModeLocal {
		return fmt.Errorf("token creation requires Cloud or a Self-managed control plane; run '%s auth login' first", filepath.Base(os.Args[0]))
	}
	client, _, err := newHTTPControlPlaneClient(context.Background(), cfg)
	if err != nil {
		return err
	}
	response, err := client.CreateWorkspaceCLIAccessToken(context.Background(), workspaceRef, httpCreateCLIAccessTokenRequest{
		Name:       name,
		Capability: capability,
		Readonly:   capability == "mount-ro",
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(response.Token) == "" {
		return fmt.Errorf("control plane created the token but did not return its secret")
	}

	bin := filepath.Base(os.Args[0])
	fmt.Fprintln(os.Stdout, "Access token created")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  token:      %s\n", response.Token)
	fmt.Fprintf(os.Stdout, "  scope:      %s\n", response.Scope)
	fmt.Fprintf(os.Stdout, "  capability: %s\n", response.Capability)
	if response.ExpiresAt != "" {
		fmt.Fprintf(os.Stdout, "  expires:    %s\n", response.ExpiresAt)
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  login: %s auth login --url %s --access-token %s\n", bin, shellQuote(cfg.URL), shellQuote(response.Token))
	fmt.Fprintf(os.Stdout, "  mount: %s ws mount %s <directory>\n", bin, shellQuote(firstNonEmptyString(response.WorkspaceName, workspaceRef)))
	return nil
}

func mountTokenCapability(permission string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(permission)) {
	case "", "rw", "write", "read-write", "mount-rw":
		return "mount-rw", nil
	case "ro", "read", "read-only", "readonly", "mount-ro":
		return "mount-ro", nil
	default:
		return "", fmt.Errorf("unsupported permission %q (expected ro or rw)", permission)
	}
}

func tokenExpiryTimestamp(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "never") {
		return "", nil
	}
	if timestamp, err := time.Parse(time.RFC3339, raw); err == nil {
		if !timestamp.After(time.Now()) {
			return "", fmt.Errorf("--expires must be in the future")
		}
		return timestamp.UTC().Format(time.RFC3339), nil
	}
	duration, err := tokenExpiryDuration(raw)
	if err != nil {
		return "", fmt.Errorf("invalid --expires value %q (use 12h, 30d, 4w, RFC3339, or never)", raw)
	}
	if duration <= 0 {
		return "", fmt.Errorf("--expires duration must be positive")
	}
	return time.Now().UTC().Add(duration).Format(time.RFC3339), nil
}

func tokenExpiryDuration(raw string) (time.Duration, error) {
	if duration, err := time.ParseDuration(raw); err == nil {
		return duration, nil
	}
	lower := strings.ToLower(strings.TrimSpace(raw))
	multiplier := 24 * time.Hour
	number := strings.TrimSuffix(lower, "d")
	if strings.HasSuffix(lower, "w") {
		multiplier = 7 * 24 * time.Hour
		number = strings.TrimSuffix(lower, "w")
	} else if !strings.HasSuffix(lower, "d") {
		return 0, fmt.Errorf("unsupported duration")
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(value * float64(multiplier)), nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func tokensUsageText(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s tokens [command]

Commands:
  create     Create a scoped CLI access token
  help       Display help for command
`, bin)
}

func tokenCreateUsageText(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s tokens create --workspace <workspace> [--permission ro|rw] [--expires 30d]
  %s tokens create <workspace> [--permission ro|rw]

Creates a workspace-scoped CLI token for mounting one workspace.

Flags:
  --workspace <name|id>    Workspace the token may mount
  --use mount              Token use; only mount is supported
  --permission ro|rw       Mount permission (default rw)
  --expires <duration>     Expiry such as 12h, 30d, 4w, RFC3339, or never
  --name <label>           Token label
`, bin, bin)
}
