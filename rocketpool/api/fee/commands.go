package fee

import (
    "strconv"

    "github.com/urfave/cli"

    cliutils "github.com/rocket-pool/smartnode-cli/rocketpool/utils/cli"
)


// Register deposit commands
func RegisterCommands(app *cli.App, name string, aliases []string) {
    app.Commands = append(app.Commands, cli.Command{
        Name:      name,
        Aliases:   aliases,
        Usage:     "Manage user fees",
        Subcommands: []cli.Command{

            // Display the current user fee
            cli.Command{
                Name:      "display",
                Aliases:   []string{"d"},
                Usage:     "Display the current user fee percentage",
                UsageText: "rocketpool fee display",
                Action: func(c *cli.Context) error {

                    // Validate arguments
                    err := cliutils.ValidateArgs(c, 0, nil)
                    if err != nil {
                        return err
                    }

                    // Run command
                    return displayUserFee(c)

                },
            },

            // Set the target user fee to vote for
            cli.Command{
                Name:      "set",
                Aliases:   []string{"s"},
                Usage:     "Set the target user fee percentage to vote for",
                UsageText: "rocketpool fee set percent" + "\n   " +
                           "- percent must be a decimal number between 0 and 100",
                Action: func(c *cli.Context) error {

                    // Arguments
                    var feePercent float64

                    // Validate arguments
                    err := cliutils.ValidateArgs(c, 1, func(messages *[]string) {
                        var err error

                        // Parse fee percentage
                        feePercent, err = strconv.ParseFloat(c.Args().Get(0), 64)
                        if err != nil {
                            *messages = append(*messages, "Invalid fee percentage - must be a decimal number")
                        }
                        if feePercent < 0 || feePercent > 100 {
                            *messages = append(*messages, "Invalid fee percentage - must be between 0 and 100")
                        }

                    })
                    if err != nil {
                        return err
                    }

                    // Run command
                    return setTargetUserFee(c, feePercent)

                },
            },

        },
    })
}

