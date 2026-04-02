package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runInit(args []string) error {
	name := "myapp"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
	}

	root, err := filepath.Abs(name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("directory %q already exists", root)
	}

	fmt.Printf("Creating project %s ...\n", name)

	// Create directory structure
	dirs := []string{
		"controllers",
		"services",
		"middlewares",
		"models",
		"config",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Write files
	files := map[string]string{
		"main.go":                               tplMain(name),
		"controllers/hello_world_controller.go": tplHelloWorldController(name),
		"services/hello_service.go":             tplHelloService(),
		"middlewares/log.go":                    tplLogMiddleware(),
		"models/hello.go":                       tplHelloModel(),
		"config/app.yaml":                       tplConfig(name),
		".gitignore":                            tplGitignore(),
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}

	// Run go mod init
	fmt.Printf("Initializing Go module ...\n")
	cmd := exec.Command("go", "mod", "init", name)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	// Run go mod tidy to fetch dependencies
	fmt.Printf("Fetching dependencies ...\n")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	fmt.Printf("\nProject created at %s\n\n", root)
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  fw build              # run pre-build, build and post-build\n")
	fmt.Printf("  go run .              # run in development mode\n")
	return nil
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func tplMain(name string) string {
	return `package main

import (
	_ "embed"
	"log"

	"github.com/linxlib/fw/app"

	"` + name + `/controllers"
	"` + name + `/middlewares"
)

//go:generate go run github.com/linxlib/fw/astp/cmd/astp -exported .

//go:embed .astp.json
var astpData []byte

func main() {
	e, err := app.New("config/app.yaml")
	if err != nil {
		panic(err)
	}

	e.EmbedProject(astpData)

	// Register middlewares
	e.RegisterMiddleware(middlewares.LogMiddleware{})

	// Register controllers
	e.RegisterController(&controllers.HelloWorldController{})

	log.Println("starting server ...")
	if err := e.ListenAndServe(); err != nil {
		panic(err)
	}
}
`
}

func tplHelloWorldController(name string) string {
	return `package controllers

import (
	"github.com/linxlib/fw/context"
	"github.com/valyala/fasthttp"

	"` + name + `/services"
)

// HelloWorldController Hello World demo controller.
// @Route /api
// @Log
type HelloWorldController struct {
	Service *services.HelloService
}

// HelloWorld Hello World
// @GET /hello
func (c *HelloWorldController) HelloWorld(ctx context.Context) error {
	if c.Service == nil {
		c.Service = services.NewHelloService()
	}

	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{
		"message": c.Service.Message(),
		"time":    c.Service.NowRFC3339(),
	})
}

// Greeting Greet a user by name
// @GET /hello/:name
func (c *HelloWorldController) Greeting(ctx context.Context) error {
	if c.Service == nil {
		c.Service = services.NewHelloService()
	}

	name := ctx.Param("name")
	return ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{
		"message": c.Service.Greeting(name),
	})
}
`
}

func tplHelloService() string {
	return `package services

import "time"

// HelloService is demo business logic.
type HelloService struct{}

func NewHelloService() *HelloService {
	return &HelloService{}
}

func (s *HelloService) Message() string {
	return "Hello, World!"
}

func (s *HelloService) Greeting(name string) string {
	if name == "" {
		return s.Message()
	}
	return "Hello, " + name + "!"
}

func (s *HelloService) NowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
`
}

func tplLogMiddleware() string {
	return `package middlewares

import (
	"log"
	"time"

	"github.com/linxlib/fw/context"
	"github.com/linxlib/fw/middleware"
)

// LogMiddleware logs request method, path and duration.
type LogMiddleware struct{}

func (LogMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{
		Name:  "Log",
		Scope: middleware.ScopeBoth,
		Stage: middleware.StageBoth,
	}
}

func (LogMiddleware) Handle(ctx context.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	start := time.Now()
	raw := ctx.Raw()
	log.Printf("--> %s %s", string(raw.Method()), string(raw.RequestURI()))

	err := next(ctx)

	status := raw.Response.StatusCode()
	log.Printf("<-- %s %s %d %s", string(raw.Method()), string(raw.RequestURI()), status, time.Since(start))
	return err
}
`
}

func tplHelloModel() string {
	return `package models

// HelloResponse is an example response model.
type HelloResponse struct {
	Message string ` + "`json:\"message\"`" + `
	Time    string ` + "`json:\"time\"`" + `
}
`
}

func tplConfig(name string) string {
	return `# ` + name + ` configuration
server:
  host: "0.0.0.0"
  port: 8080

log:
  level: info
  output: console

openapi:
  enabled: true
  output: openapi.json
  title: "` + name + ` API"
  version: "1.0.0"
`
}

func tplGitignore() string {
	return `# Build output
*.exe
*.exe~
*.dll
*.so
*.dylib

# AST metadata (generated)
.astp.json

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
`
}
