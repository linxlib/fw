package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Init(c *cli.Context) error {
	fmt.Println("init called")
	return nil
}
