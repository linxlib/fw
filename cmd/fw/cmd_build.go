package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/linxlib/fw/v2/astp"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type buildOptions struct {
	output string // -o
	goos   string // -os
	goarch string // -arch
	dir    string // working directory (positional or -dir)
}

func newBuildCmd() *cobra.Command {
	var opts buildOptions
	cmd := &cobra.Command{
		Use:   "build [directory]",
		Short: "Build a project from a working directory",
		Long:  `Run pre-build flow, compile the project from the target working directory, then run post-build flow.`,
		Example: `  fw build
  fw build -o myapp
  fw build -os linux -arch amd64
  fw build -os macos -arch arm64 -o app
  fw build ./cmd/server`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			effective := opts
			if len(args) > 0 {
				effective.dir = args[0]
			}

			if err := runBuild(effective); err != nil {
				return fmt.Errorf("fw build: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "output binary name (default: directory name)")
	cmd.Flags().StringVar(&opts.goos, "os", "", "target OS: windows, linux, macos (default: current)")
	cmd.Flags().StringVar(&opts.goarch, "arch", "", "target arch: amd64, arm64 (default: current)")
	cmd.Flags().StringVar(&opts.dir, "dir", ".", "project working directory")

	return cmd
}

func runBuild(opts buildOptions) error {
	absDir, err := filepath.Abs(opts.dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absDir); err != nil {
		return fmt.Errorf("directory not found: %s", absDir)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("stat working directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("working directory must be a directory: %s", absDir)
	}

	targetOS := opts.goos
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	targetOS = normalizeOS(targetOS)
	targetArch := opts.goarch
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	// Resolve output name
	outName := opts.output
	if outName == "" {
		outName = filepath.Base(absDir)
	}
	if targetOS == "windows" && !strings.HasSuffix(outName, ".exe") {
		outName += ".exe"
	}
	if targetOS != "windows" {
		outName = strings.TrimSuffix(outName, ".exe")
	}

	// Validate OS/Arch
	if !isValidOS(targetOS) {
		return fmt.Errorf("unsupported OS: %q (supported: windows, linux, macos)", opts.goos)
	}
	if !isValidArch(targetArch) {
		return fmt.Errorf("unsupported arch: %q (supported: amd64, arm64)", targetArch)
	}

	// Step 1: Pre-build flow
	astpFile := filepath.Join(absDir, astp.DefaultOutputFile)
	if err := runSpinnerStep("[1/3] Running pre-build flow", func(sp *pterm.SpinnerPrinter) error {
		sp.UpdateText("[1/3] Running pre-build flow (go generate ./...)")
		preGenerateCmd := exec.Command("go", "generate", "./...")
		preGenerateCmd.Dir = absDir
		preGenerateCmd.Stdout = os.Stdout
		preGenerateCmd.Stderr = os.Stderr
		if err := preGenerateCmd.Run(); err != nil {
			return fmt.Errorf("pre-build go generate: %w", err)
		}

		sp.UpdateText(fmt.Sprintf("[1/3] Running pre-build flow (generating %s)", astp.DefaultOutputFile))
		g := astp.NewGenerator()
		g.ExportedOnly = true
		if err := g.Generate(absDir, ""); err != nil {
			return fmt.Errorf("pre-build astp generation: %w", err)
		}

		sp.UpdateText(fmt.Sprintf("[1/3] Running pre-build flow (%s)", astpFile))
		return nil
	}); err != nil {
		return err
	}

	// Step 2: Compile
	if err := runSpinnerStep(fmt.Sprintf("[2/3] Compiling %s/%s -> %s", targetOS, targetArch, outName), func(sp *pterm.SpinnerPrinter) error {
		buildArgs := []string{"build", "-o", outName, "."}
		cmd := exec.Command("go", buildArgs...)
		cmd.Dir = absDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = buildEnv(targetOS, targetArch)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go build: %w", err)
		}
		sp.UpdateText(fmt.Sprintf("[2/3] Compiled %s/%s -> %s", targetOS, targetArch, outName))
		return nil
	}); err != nil {
		return err
	}

	// Step 3: Post-build flow
	outPath := filepath.Join(absDir, outName)
	var artifactInfo os.FileInfo
	var checksum string
	if err := runSpinnerStep("[3/3] Running post-build flow", func(sp *pterm.SpinnerPrinter) error {
		sp.UpdateText("[3/3] Running post-build flow (artifact check)")
		var err error
		artifactInfo, err = os.Stat(outPath)
		if err != nil {
			return fmt.Errorf("post-build output check: %w", err)
		}

		sp.UpdateText("[3/3] Running post-build flow (sha256)")
		checksum, err = fileSHA256(outPath)
		if err != nil {
			return fmt.Errorf("post-build checksum: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Printf("      artifact: %s (%s)\n", outPath, humanSize(artifactInfo.Size()))
	fmt.Printf("      sha256 : %s\n", checksum)
	fmt.Printf("\nBuild succeeded.\n")
	return nil
}

func runSpinnerStep(title string, fn func(sp *pterm.SpinnerPrinter) error) error {
	sp, err := pterm.DefaultSpinner.Start(title + " ...")
	if err != nil {
		return fmt.Errorf("start spinner: %w", err)
	}

	if err := fn(sp); err != nil {
		sp.Fail(title + " failed")
		return err
	}

	sp.Success(title + " done")
	return nil
}

func buildEnv(goos, goarch string) []string {
	env := os.Environ()
	// Remove existing GOOS/GOARCH if any, then append ours
	var filtered []string
	for _, e := range env {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "GOOS=") || strings.HasPrefix(upper, "GOARCH=") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, "GOOS="+goos, "GOARCH="+goarch)
	// CGO is usually off for cross-compile
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		filtered = append(filtered, "CGO_ENABLED=0")
	}
	return filtered
}

func isValidOS(s string) bool {
	switch s {
	case "windows", "linux", "darwin":
		return true
	}
	return false
}

func normalizeOS(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "mac", "macos", "osx":
		return "darwin"
	}
	return s
}

func isValidArch(s string) bool {
	switch s {
	case "amd64", "arm64":
		return true
	}
	return false
}

func humanSize(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
