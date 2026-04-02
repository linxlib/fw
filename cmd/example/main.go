package main

import (
	_ "embed"
	"log"

	"github.com/linxlib/fw/app"
	"github.com/linxlib/fw/cmd/example/controllers"
	"github.com/linxlib/fw/cmd/example/middlewares"
	"github.com/linxlib/fw/cmd/example/services"
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

	// Register a singleton into the global container.
	// It can be injected into controller method parameters by type.
	db := services.NewDemoDB("demo-main")
	userService := services.NewUserService()
	e.Container().Map(db)
	e.Container().Map(userService)

	e.RegisterMiddleware(middlewares.AuthorizationMiddleware{})
	e.RegisterMiddleware(middlewares.LogMiddleware{})
	e.RegisterMiddleware(middlewares.WebSocketMiddleware{})
	e.RegisterMiddleware(middlewares.SSEMiddleware{})
	e.RegisterController(&controllers.UserController{})
	log.Println("starting server...")
	if err := e.ListenAndServe(); err != nil {
		panic(err)
	}
}
