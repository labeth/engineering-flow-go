package engflow

import (
	"flag"
	"fmt"
	"io"
)

const version = "0.1.0"

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		printUsage(out)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(out)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintf(out, "engflow %s\n", version)
		return 0
	case "init":
		return runInit(args[1:], out, errOut)
	case "gate":
		return runGate(args[1:], out, errOut)
	case "trace-query":
		return runTraceQuery(args[1:], out, errOut)
	case "status":
		return runStatus(args[1:], out, errOut)
	case "verify":
		return runVerify(args[1:], out, errOut)
	case "drift":
		return runDrift(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown command: %s\n\n", args[0])
		printUsage(errOut)
		return 2
	}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `engflow coordinates feature specs, engineering artifacts, and verification.

Usage:
  engflow <command> [flags]

Commands:
  init        Scaffold a new project (single-source engmodel artifacts)
  gate        Run verify + drift + status workflow gate
  trace-query Search repository files for REQ-* ID occurrences
  status      Show done/blockers/next-action summary
  verify      Run regeneration/tests and emit verification report
  drift       Detect cross-artifact drift and severity
  version     Print version
  help        Show this help

All commands:
  --config <path>  Load defaults from config file (default: .engflow/config.yml)

Examples:
  engflow init --project-dir ../my-project --feature initial-feature
  engflow init --project-dir ../my-project --feature initial-feature --regen-cmd "make engmodel-generate"
  engflow init --project-dir ../my-project --feature initial-feature --no-speckit-init --no-generate-outputs
  engflow gate --feature auth-login --config .engflow/config.yml
  engflow trace-query --id REQ-AUTH-001
  engflow status --feature auth-login --config .engflow/config.yml
  engflow verify --feature auth-login --regen-cmd "make ai-gen" --test-cmd "go test ./..."
`)
}
