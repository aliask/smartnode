package node

import (
    "errors"
    "fmt"

    "gopkg.in/urfave/cli.v1"

    "github.com/rocket-pool/smartnode-cli/rocketpool/services"
)


// Set the node's timezone
func setNodeTimezone(c *cli.Context) error {

    // Initialise services
    p, err := services.NewProvider(c, services.ProviderOpts{
        AM: true,
        CM: true,
        NodeContractAddress: true,
        LoadContracts: []string{"rocketNodeAPI"},
    })
    if err != nil {
        return err
    }

    // Prompt user for timezone
    timezone := promptTimezone()

    // Set node timezone
    if txor, err := p.AM.GetNodeAccountTransactor(); err != nil {
        return err
    } else if _, err := p.CM.Contracts["rocketNodeAPI"].Transact(txor, "setTimezoneLocation", timezone); err != nil {
        return errors.New("Error setting node timezone: " + err.Error())
    }

    // Get node timezone
    nodeTimezone := new(string)
    if err := p.CM.Contracts["rocketNodeAPI"].Call(nil, nodeTimezone, "getTimezoneLocation", p.AM.GetNodeAccount().Address); err != nil {
        return errors.New("Error retrieving node timezone: " + err.Error())
    }

    // Log & return
    fmt.Println("Node timezone successfully updated to:", *nodeTimezone)
    return nil

}

