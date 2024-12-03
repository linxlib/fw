package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Build(c *cli.Context) error {
	fmt.Println("build called")
	return nil
}
