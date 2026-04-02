package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/linxlib/fw/astp"
)

type buildOptions struct {
	output string // -o
	goos   string // -os
	goarch string // -arch
	dir    string // working directory (positional or -dir)
}

func runBuild(args []string) error {
	var opts buildOptions

	fs := flag.NewFlagSet("build", flag.ExitOnError)
	fs.StringVar(&opts.output, "o", "", "output binary name (default: directory name)")
	fs.StringVar(&opts.goos, "os", "", "target OS: windows, linux, macos (default: current)")
	fs.StringVar(&opts.goarch, "arch", "", "target arch: amd64, arm64 (default: current)")
	fs.StringVar(&opts.dir, "dir", ".", "project directory")
	fs.Usage = func() {
		fmt.Print(`Usage: fw build [flags] [directory]

Run pre-build flow, compile the project, then run post-build flow.

Flags:
`)
		fs.PrintDefaults()
		fmt.Print(`
Examples:
  fw build                              # pre-build + build + post-build
  fw build -o myapp                     # custom output name
  fw build -os linux -arch amd64        # cross-compile for Linux amd64
  fw build -os macos -arch arm64 -o app # cross-compile for macOS arm64
  fw build ./cmd/server                 # build specific directory
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		opts.dir = fs.Arg(0)
	}

	absDir, err := filepath.Abs(opts.dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absDir); err != nil {
		return fmt.Errorf("directory not found: %s", absDir)
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
	fmt.Printf("[1/3] Running pre-build flow ...\n")
	preGenerateCmd := exec.Command("go", "generate", "./...")
	preGenerateCmd.Dir = absDir
	preGenerateCmd.Stdout = os.Stdout
	preGenerateCmd.Stderr = os.Stderr
	if err := preGenerateCmd.Run(); err != nil {
		return fmt.Errorf("pre-build go generate: %w", err)
	}

	fmt.Printf("      generating %s ...\n", astp.DefaultOutputFile)
	g := astp.NewGenerator()
	g.ExportedOnly = true
	if err := g.Generate(absDir, ""); err != nil {
		return fmt.Errorf("pre-build astp generation: %w", err)
	}
	astpFile := filepath.Join(absDir, astp.DefaultOutputFile)
	fmt.Printf("      %s\n", astpFile)

	// Step 2: Compile
	fmt.Printf("[2/3] Compiling %s/%s -> %s ...\n", targetOS, targetArch, outName)
	buildArgs := []string{"build", "-o", outName}
	buildArgs = append(buildArgs, ".")

	cmd := exec.Command("go", buildArgs...)
	cmd.Dir = absDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = buildEnv(targetOS, targetArch)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	// Step 3: Post-build flow
	fmt.Printf("[3/3] Running post-build flow ...\n")
	outPath := filepath.Join(absDir, outName)
	info, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("post-build output check: %w", err)
	}

	checksum, err := fileSHA256(outPath)
	if err != nil {
		return fmt.Errorf("post-build checksum: %w", err)
	}

	fmt.Printf("      artifact: %s (%s)\n", outPath, humanSize(info.Size()))
	fmt.Printf("      sha256 : %s\n", checksum)
	fmt.Printf("\nBuild succeeded.\n")
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
