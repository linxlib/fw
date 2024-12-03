package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Config(c *cli.Context) error {
	fmt.Println("config called")
	return nil
}
