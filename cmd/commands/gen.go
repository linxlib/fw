package commands

import (
	"fmt"
	"github.com/linxlib/fw/cmd/utils"
	"github.com/urfave/cli/v2"
)

func Generate(c *cli.Context) error {
	fmt.Println("gen called")
	err := utils.RunCmd("go", "run", "github.com/linxlib/astp/astpg", "-o", "gen.json")
	if err != nil {
		return err
	}
	return nil
}
