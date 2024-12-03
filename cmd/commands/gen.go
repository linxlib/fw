package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Generate(c *cli.Context) error {
	fmt.Println("gen called")
	return nil
}
