package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdDaemon(args []string) error {
	if len(args) < 2 || isHelpArg(args[1]) {
		fmt.Fprint(os.Stderr, daemonUsageText(filepath.Base(os.Args[0])))
		return nil
	}
	switch args[1] {
	case "status":
		return cmdStatusArgs(args[2:])
	case "stop":
		if len(args) > 2 && isHelpArg(args[2]) {
			fmt.Fprint(os.Stderr, daemonStopUsageText(filepath.Base(os.Args[0])))
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("%s", daemonStopUsageText(filepath.Base(os.Args[0])))
		}
		return unmountAllActive(false)
	default:
		return fmt.Errorf("unknown daemon subcommand %q\n\n%s", args[1], daemonUsageText(filepath.Base(os.Args[0])))
	}
}

func daemonUsageText(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s daemon <subcommand>

Subcommands:
  status [--verbose]   Show daemon status and active mount sessions
  stop                 Stop active AFS mount sessions without deleting local files
`, bin)
}

func daemonStopUsageText(bin string) string {
	return brandHeaderString() + fmt.Sprintf(`Usage:
  %s daemon stop

Stop active AFS mount sessions without deleting local files.
`, bin)
}
