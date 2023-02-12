package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/urfave/cli"
	"golang.org/x/crypto/openpgp"

	"github.com/rocket-pool/smartnode/shared"
	cliutils "github.com/rocket-pool/smartnode/shared/utils/cli"
)

// Settings
const (
	GithubAPIGetLatest string = "https://api.github.com/repos/rocket-pool/smartnode-install/releases/latest"
	SigningKeyURL      string = "https://github.com/rocket-pool/smartnode-install/releases/download/v%s/smartnode-signing-key-v3.asc"
	ReleaseBinaryURL   string = "https://github.com/rocket-pool/smartnode-install/releases/download/v%s/rocketpool-cli-%s-%s"
)

func getHttpClientWithTimeout() *http.Client {
	return &http.Client{
		Timeout: time.Second * 5,
	}
}

func checkSignature(signatureUrl string, pubkeyUrl string, verification_target *os.File) error {
	pubkeyResponse, err := http.Get(pubkeyUrl)
	if err != nil {
		return err
	}
	defer pubkeyResponse.Body.Close()
	if pubkeyResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("public key request failed with code %d", pubkeyResponse.StatusCode)
	}
	keyring, err := openpgp.ReadArmoredKeyRing(pubkeyResponse.Body)
	if err != nil {
		return fmt.Errorf("error while reading public key: %w", err)
	}

	signatureResponse, err := http.Get(signatureUrl)
	if err != nil {
		return err
	}
	defer signatureResponse.Body.Close()
	if signatureResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("signature request failed with code %d", signatureResponse.StatusCode)
	}

	entity, err := openpgp.CheckDetachedSignature(keyring, verification_target, signatureResponse.Body)
	if err != nil {
		return fmt.Errorf("error while verifying signature: %w", err)
	}

	for _, v := range entity.Identities {
		fmt.Printf("Signature verified. Signed by: %s\n", v.Name)
	}
	return nil
}

// Update the Rocket Pool CLI
func updateCLI(c *cli.Context) error {
	// Check the latest version published to the Github repository
	client := getHttpClientWithTimeout()
	resp, err := client.Get(GithubAPIGetLatest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with code %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var apiResponse map[string]interface{}
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return fmt.Errorf("could not decode Github API response: %w", err)
	}
	var latestVersion semver.Version
	if x, found := apiResponse["url"]; found {
		var name string
		var ok bool
		if name, ok = x.(string); !ok {
			return fmt.Errorf("unexpected Github API response format")
		}
		latestVersion, err = semver.Make(strings.TrimLeft(name, "v"))
		if err != nil {
			return fmt.Errorf("could not parse version number from release name '%s': %w", name, err)
		}
	} else {
		return fmt.Errorf("unexpected Github API response format")
	}

	// Check this version against the currently installed version
	if !c.Bool("force") {
		currentVersion, err := semver.Make(shared.RocketPoolVersion)
		if err != nil {
			return fmt.Errorf("could not parse local Rocket Pool version number '%s': %w", shared.RocketPoolVersion, err)
		}
		switch latestVersion.Compare(currentVersion) {
		case 1:
			fmt.Printf("Newer version avilable online (v%s). Downloading...\n", latestVersion.String())
		case 0:
			fmt.Printf("Already on latest version (v%s). Aborting update\n", latestVersion.String())
			return nil
		default:
			fmt.Printf("Online version (v%s) is lower than running version (v%s). Aborting update\n", latestVersion.String(), currentVersion.String())
			return nil
		}
	} else {
		fmt.Printf("Forced update to v%s. Downloading...\n", latestVersion.String())
	}

	// Download the new binary to same folder as the running RP binary, as `rocketpool-vX.X.X`
	var ClientURL = fmt.Sprintf(ReleaseBinaryURL, latestVersion.String(), runtime.GOOS, runtime.GOARCH)
	resp, err = http.Get(ClientURL)
	if err != nil {
		return fmt.Errorf("error while downloading %s: %w", ClientURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with code %d", resp.StatusCode)
	}

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error while determining running rocketpool location: %w", err)
	}
	var rpBinDir = filepath.Dir(ex)
	var fileName = filepath.Join(rpBinDir, "rocketpool-v"+latestVersion.String())
	output, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("error while creating %s: %w", fileName, err)
	}
	defer output.Close()

	_, err = io.Copy(output, resp.Body)
	if err != nil {
		return fmt.Errorf("error while downloading %s: %w", ClientURL, err)
	}

	// Verify the signature of the downloaded binary
	if !c.Bool("skip-signature-verification") {
		var pubkeyUrl = fmt.Sprintf(SigningKeyURL, latestVersion.String())
		output.Seek(0, io.SeekStart)
		err = checkSignature(ClientURL+".sig", pubkeyUrl, output)
		if err != nil {
			return fmt.Errorf("error while verifying GPG signature: %w", err)
		}
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm("Are you sure you want to update? Current Rocketpool Client will be replaced.")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Do the switcheroo - move `rocketpool-vX.X.X` to the location of the current Rocketpool Client
	err = os.Remove(ex)
	if err != nil {
		return fmt.Errorf("error while removing old rocketpool binary: %w", err)
	}
	err = os.Rename(fileName, ex)
	if err != nil {
		return fmt.Errorf("error while writing new rocketpool binary: %w", err)
	}

	fmt.Printf("Updated Rocketpool Client to v%s. Please run `rocketpool service install` to finish the installation and update your smartstack.\n", latestVersion.String())
	return nil
}

// Register commands
func RegisterCommands(app *cli.App, name string, aliases []string) {

	app.Commands = append(app.Commands, cli.Command{
		Name:    name,
		Aliases: aliases,
		Subcommands: []cli.Command{
			{
				Name:      "cli",
				Aliases:   []string{"c"},
				Usage:     "Update the Rocket Pool CLI",
				UsageText: "rocketpool update cli [options]",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "force, f",
						Usage: "Force update, even if same version or lower",
					},
					cli.BoolFlag{
						Name:  "skip-signature-verification, s",
						Usage: "Skip signature verification",
					},
					cli.BoolFlag{
						Name:  "yes, y",
						Usage: "Automatically confirm update",
					},
				},
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run command
					return updateCLI(c)

				},
			},
		},
	})
}
