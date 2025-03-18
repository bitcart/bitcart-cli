package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
	"github.com/ybbus/jsonrpc/v3"
)

var rootOptions *Config

func getSpec(
	client *http.Client,
	endpoint string,
	user string,
	password string,
) map[string]interface{} {
	req, err := http.NewRequest("GET", endpoint+"/spec", nil)
	checkErr(err)
	req.Header.Add("User-Agent", UserAgent())
	req.SetBasicAuth(user, password)
	resp, err := client.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return jsonDecodeBytes(bodyBytes)
}

func getDefaultURL(coin string) string {
	symbol := strings.ToUpper(coin)
	envHost := os.Getenv(symbol + "_HOST")
	envPort := os.Getenv(symbol + "_PORT")
	host := "localhost"
	if envHost != "" {
		host = envHost
	}
	var port = COINS[coin]
	if envPort != "" {
		port = envPort
	}
	return "http://" + host + ":" + port
}

func runCommand(c *cli.Command, help bool) (*jsonrpc.RPCResponse, map[string]interface{}, error) {
	args := c.Args()
	wallet := c.String("wallet")
	contract := c.String("contract")
	address := c.String("address")
	diskless := c.Bool("diskless")
	user := c.String("user")
	password := c.String("password")
	coin := c.String("coin")
	url := c.String("url")
	noSpec := c.Bool("no-spec")
	if url == "" {
		url = getDefaultURL(coin)
	}
	httpClient := &http.Client{}
	// initialize rpc client
	rpcClient := jsonrpc.NewClientWithOpts(url, &jsonrpc.RPCClientOpts{
		HTTPClient: httpClient,
		CustomHeaders: map[string]string{
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString(
				[]byte(user+":"+password),
			),
			"User-Agent": UserAgent(),
		},
	})
	// some magic to make array with the last element being a dictionary with xpub in it
	sl := []string{}
	if !help {
		sl = args.Slice()[1:]
	}
	var params []interface{}
	keyParams := map[string]interface{}{
		"xpub": map[string]interface{}{
			"xpub":     wallet,
			"contract": contract,
			"address":  address,
			"diskless": diskless,
		},
	}
	acceptFlags := false
	i := 0
	for i < len(sl) {
		if sl[i] == "--" {
			acceptFlags = true
			i += 1
		}
		if strings.HasPrefix(sl[i], "--") && acceptFlags {
			if i+1 >= len(sl) {
				exitErr("Error: missing value for flag " + sl[i])
			}
			keyParams[sl[i][2:]] = sl[i+1]
			i += 1
		} else {
			params = append(params, sl[i])
		}
		i += 1
	}
	params = append(params, keyParams)
	// call RPC method
	command := "help"
	if !help {
		command = args.Get(0)
	}
	result, err := rpcClient.Call(context.Background(), command, params)
	if err != nil {
		return nil, nil, err
	}
	spec := map[string]interface{}{}
	if !noSpec {
		spec = getSpec(httpClient, url, user, password)
	}
	return result, spec, nil
}

func main() {
	rootOptions = &Config{}
	rootOptions.Load()
	app := &cli.Command{
		Name:                  "bitcart-cli",
		Version:               Version,
		HideHelp:              true,
		Usage:                 "Call RPC methods from console",
		UsageText:             "bitcart-cli method [args]",
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "help",
				Aliases: []string{"h"},
				Usage:   "show help",
			},
			&cli.StringFlag{
				Name:     "wallet",
				Aliases:  []string{"w"},
				Usage:    "specify wallet",
				Required: false,
				Sources:  cli.EnvVars("BITCART_WALLET"),
			},
			&cli.StringFlag{
				Name:     "contract",
				Usage:    "specify contract",
				Required: false,
				Sources:  cli.EnvVars("BITCART_CONTRACT"),
			},
			&cli.StringFlag{
				Name:     "address",
				Usage:    "specify address (XMR-only)",
				Required: false,
				Sources:  cli.EnvVars("BITCART_ADDRESS"),
			},
			&cli.BoolFlag{
				Name:    "diskless",
				Aliases: []string{"d"},
				Usage:   "Load wallet in memory only",
				Value:   false,
				Sources: cli.EnvVars("BITCART_DISKLESS"),
			},
			&cli.StringFlag{
				Name:    "coin",
				Aliases: []string{"c"},
				Usage:   "specify coin to use",
				Value:   "btc",
				Sources: cli.EnvVars("BITCART_COIN"),
			},
			&cli.StringFlag{
				Name:    "user",
				Aliases: []string{"u"},
				Usage:   "specify daemon user",
				Value:   "electrum",
				Sources: cli.EnvVars("BITCART_LOGIN"),
			},
			&cli.StringFlag{
				Name:    "password",
				Aliases: []string{"p"},
				Usage:   "specify daemon password",
				Value:   "electrumz",
				Sources: cli.EnvVars("BITCART_PASSWORD"),
			},
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"U"},
				Usage:    "specify daemon URL (overrides defaults)",
				Required: false,
				Sources:  cli.EnvVars("BITCART_DAEMON_URL"),
			},
			&cli.BoolFlag{
				Name:    "no-spec",
				Usage:   "Disables spec fetching for better exceptions display",
				Value:   false,
				Sources: cli.EnvVars("BITCART_NO_SPEC"),
			},
			&cli.StringFlag{
				Name:        "github-api",
				Value:       "https://api.github.com",
				Usage:       "Change the default endpoint to GitHub API for retrieving updates",
				Destination: &rootOptions.GitHubAPI,
			},
			&cli.BoolFlag{
				Name:        "skip-update-check",
				Usage:       "Skip the check for updates check run before every command.",
				Value:       skipUpdateByDefault(),
				Destination: &rootOptions.SkipUpdateCheck,
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Args().Get(0) == "update" {
				return ctx, nil
			}
			err := checkForUpdates(rootOptions)
			if err != nil {
				fmt.Printf("Error checking for updates: %s\n", err)
			}
			return ctx, nil
		},
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			output, _, err := runCommand(cmd, true)
			if err != nil || output.Error != nil {
				fmt.Println("plugin")
				if updatesEnabled() {
					fmt.Println("update")
				}
				return
			}
			output.Result = append(output.Result.([]interface{}), "plugin")
			if updatesEnabled() {
				output.Result = append(output.Result.([]interface{}), "update")
			}
			for _, v := range output.Result.([]interface{}) {
				fmt.Println(v)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args()
			if args.Len() >= 1 {
				result, spec, err := runCommand(cmd, false)
				checkErr(err)
				// Print either error if found or result
				if result.Error != nil {
					if len(spec) != 0 {
						if spec["error"] != nil {
							exitErr(jsonEncode(spec["error"]))
						}
						exceptions := spec["exceptions"].(map[string]interface{})
						errorCode := fmt.Sprint(result.Error.Code)
						if exception, ok := exceptions[errorCode]; ok {
							exception, _ := exception.(map[string]interface{})
							exitErr(
								exception["exc_name"].(string) + ": " + exception["docstring"].(string),
							)
						}
					}
					exitErr(jsonEncode(result.Error))
				} else {
					var v, ok = result.Result.(string)
					if ok {
						smartPrint(v)
					} else {
						smartPrint(jsonEncode(result.Result))
					}
					return nil
				}
			} else {
				checkErr(cli.ShowAppHelp(cmd))
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "plugin",
				Usage: "Manage plugins",
				Commands: []*cli.Command{
					{
						Name:      "init",
						Action:    initPlugin,
						Usage:     "Create a new plugin",
						UsageText: "bitcart-cli plugin init <path>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "save",
								Aliases: []string{"s"},
								Usage:   "Save repository directories to not ask later",
								Value:   false,
							},
						},
					},
					{
						Name:      "install",
						Action:    installPlugin,
						Usage:     "Install a plugin",
						UsageText: "bitcart-cli plugin install [command options] <path>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "dev",
								Usage:   "Install in development mode (symlink instead of copying)",
								Value:   false,
								Aliases: []string{"D"},
							},
							&cli.BoolFlag{
								Name:    "save",
								Aliases: []string{"s"},
								Usage:   "Save repository directories to not ask later",
								Value:   false,
							},
						},
					},
					{
						Name:      "uninstall",
						Action:    uninstallPlugin,
						Usage:     "Uninstall a plugin",
						UsageText: "bitcart-cli plugin uninstall <path>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "save",
								Aliases: []string{"s"},
								Usage:   "Save repository directories to not ask later",
								Value:   false,
							},
						},
					},
					{
						Name:      "validate",
						Action:    validatePlugin,
						Usage:     "Validate plugin manifest and common checks",
						UsageText: "bitcart-cli plugin validate <path>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "schema",
								Usage: "Supply custom schema URL",
								Value: schemaURL,
							},
						},
					},
					{
						Name:      "package",
						Action:    packagePlugin,
						Usage:     "Package plugin from its directory",
						UsageText: "bitcart-cli plugin package [command options] <path>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "no-strip",
								Usage: "Don't strip unneccesary files from the package (i.e. node_modules)",
								Value: false,
							},
						},
					},
				},
			},
		},
	}
	if updatesEnabled() {
		app.Commands = append(app.Commands, &cli.Command{
			Name:  "update",
			Usage: "CLI update operations",
			Commands: []*cli.Command{
				{
					Name:   "check",
					Action: updateCLI,
					Usage:  "Check if there are any updates available",
				},
				{
					Name:   "install",
					Action: updateCLI,
					Usage:  "Update the tool to the latest version",
				},
			},
		})
	}
	godotenv.Load(envFile) // nolint:errcheck
	checkErr(app.Run(context.Background(), os.Args))
}
