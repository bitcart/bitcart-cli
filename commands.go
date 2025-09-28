package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v3"
	"golang.org/x/exp/slices"
)

type ComponentType struct {
	Type string
	Path string
}

type BasicCreatePluginAnswers struct {
	Name           string
	Author         string
	Description    string
	ComponentTypes []string
	FinalTypes     []ComponentType
}

func createInitPyFile(dir string) {
	createIfNotExists(dir, os.ModePerm)
	initPyPath := filepath.Join(dir, "__init__.py")
	if !exists(initPyPath) {
		checkErr(os.WriteFile(initPyPath, []byte(""), os.ModePerm))
	}
}

func removeOrgInitIfNoPlugins(orgPath string) {
	entries, err := os.ReadDir(orgPath)
	if err != nil {
		return
	}
	hasPluginDirs := false
	onlyAllowed := true
	for _, e := range entries {
		name := e.Name()
		if name == "__pycache__" || name == "__init__.py" {
			continue
		}
		onlyAllowed = false
		fi, statErr := e.Info()
		if statErr != nil {
			hasPluginDirs = true
			break
		}
		mode := fi.Mode()
		if mode.IsDir() || (mode&os.ModeSymlink != 0) {
			hasPluginDirs = true
			break
		}
	}
	if hasPluginDirs {
		return
	}
	pycachePath := filepath.Join(orgPath, "__pycache__")
	if exists(pycachePath) {
		checkErr(os.RemoveAll(pycachePath))
	}
	initPy := filepath.Join(orgPath, "__init__.py")
	if exists(initPy) {
		checkErr(os.Remove(initPy))
	}
	if onlyAllowed {
		checkErr(os.Remove(orgPath))
	}
}

func initPlugin(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return cli.ShowSubcommandHelp(cmd)
	}
	path := args.Get(0)
	save := cmd.Bool("save")
	path, err := filepath.Abs(path)
	createIfNotExists(path, os.ModePerm)
	checkErr(err)
	answers := BasicCreatePluginAnswers{}
	checkErr(survey.Ask(basicPluginCreate, &answers))
	if slices.Contains(answers.ComponentTypes, "backend") {
		backendPath := getComponentConfigEntry("backend")
		var componentName string
		checkErr(
			survey.AskOne(
				&survey.Input{
					Message: "Enter name of your backend component (i.e. name of the subfolder)",
				},
				&componentName,
				survey.WithValidator(survey.Required),
			),
		)
		if *backendPath == "" {
			checkErr(
				survey.AskOne(
					&survey.Input{Message: "Enter the path to cloned bitcart repository"},
					backendPath,
					survey.WithValidator(survey.Required),
					survey.WithValidator(directoryValidator),
					survey.WithValidator(backendDirectoryValidator),
				),
			)
		}
		*backendPath, err = filepath.Abs(*backendPath)
		checkErr(err)
		internalPath := filepath.Join(path, "src/backend/"+componentName)
		answers.FinalTypes = append(
			answers.FinalTypes,
			ComponentType{Type: "backend", Path: "src/backend/" + componentName},
		)
		data := struct {
			Name string
		}{Name: componentName}
		createIfNotExists(internalPath, os.ModePerm)
		checkErr(
			os.WriteFile(
				filepath.Join(internalPath, "plugin.py"),
				executeTemplate("plugin/src/backend/plugin.py.tmpl", data, false),
				os.ModePerm,
			),
		)
		createInitPyFile(internalPath)
		safeSymlink(
			internalPath,
			filepath.Join(
				*backendPath,
				getOutputDirectory("backend", answers.Author, componentName),
			),
		)
	}
	if slices.Contains(answers.ComponentTypes, "docker") {
		dockerPath := getComponentConfigEntry("docker")
		var componentName string
		checkErr(
			survey.AskOne(
				&survey.Input{
					Message: "Enter name of your docker component (i.e. name of the subfolder)",
				},
				&componentName,
				survey.WithValidator(survey.Required),
			),
		)
		if *dockerPath == "" {
			checkErr(
				survey.AskOne(
					&survey.Input{Message: "Enter the path to cloned bitcart-docker repository"},
					dockerPath,
					survey.WithValidator(survey.Required),
					survey.WithValidator(directoryValidator),
					survey.WithValidator(dockerDirectoryValidator),
				),
			)
		}
		*dockerPath, err = filepath.Abs(*dockerPath)
		checkErr(err)
		internalPath := filepath.Join(path, "src/docker/"+componentName)
		answers.FinalTypes = append(
			answers.FinalTypes,
			ComponentType{Type: "docker", Path: "src/docker/" + componentName},
		)
		createIfNotExists(internalPath, os.ModePerm)
		safeSymlink(
			internalPath,
			filepath.Join(
				*dockerPath,
				getOutputDirectory("docker", answers.Author, componentName),
			),
		)
	}
	for _, componentType := range []string{"admin", "store"} {
		if slices.Contains(answers.ComponentTypes, componentType) {
			frontendPath := getComponentConfigEntry(componentType)
			var componentName string
			checkErr(
				survey.AskOne(
					&survey.Input{
						Message: fmt.Sprintf(
							"Enter name of your %s component (i.e. name of the subfolder)",
							componentType,
						),
					},
					&componentName,
					survey.WithValidator(survey.Required),
				),
			)
			if *frontendPath == "" {
				checkErr(
					survey.AskOne(
						&survey.Input{
							Message: fmt.Sprintf(
								"Enter the path to cloned %s frontend repository",
								componentType,
							),
						},
						frontendPath,
						survey.WithValidator(survey.Required),
						survey.WithValidator(directoryValidator),
						survey.WithValidator(frontendDirectoryValidator),
					),
				)
			}
			*frontendPath, err = filepath.Abs(*frontendPath)
			checkErr(err)
			answers.FinalTypes = append(
				answers.FinalTypes,
				ComponentType{
					Type: componentType,
					Path: "src/" + componentType + "/" + componentName,
				},
			)
			internalPath := filepath.Join(path, "src/"+componentType+"/"+componentName)
			createIfNotExists(internalPath, os.ModePerm)
			createIfNotExists(filepath.Join(internalPath, "config"), os.ModePerm)
			for _, file := range []string{"index.js", "config/extends.js", "config/routes.js"} {
				copyFileContents(
					filepath.Join("plugin/src/frontend", file),
					filepath.Join(internalPath, file),
				)
			}
			data := struct {
				Author string
				Name   string
			}{Author: answers.Author, Name: componentName}
			checkErr(
				os.WriteFile(
					filepath.Join(internalPath, "package.json"),
					executeTemplate("plugin/src/frontend/package.json.tmpl", data, false),
					os.ModePerm,
				),
			)
			checkErr(
				os.WriteFile(
					filepath.Join(internalPath, "config/index.js"),
					executeTemplate("plugin/src/frontend/config/index.js.tmpl", data, false),
					os.ModePerm,
				),
			)
			safeSymlink(
				internalPath,
				filepath.Join(
					*frontendPath,
					getOutputDirectory(componentType, answers.Author, componentName),
				),
			)
		}
	}
	checkErr(
		os.WriteFile(
			filepath.Join(path, "manifest.json"),
			executeTemplate("plugin/manifest.json.tmpl", answers, true),
			os.ModePerm,
		),
	)
	checkErr(
		os.WriteFile(
			filepath.Join(path, ".gitignore"),
			executeTemplate("plugin/.gitignore.tmpl", answers, true),
			os.ModePerm,
		),
	)
	copyFileContents("plugin/.editorconfig", filepath.Join(path, ".editorconfig"))
	if save {
		rootOptions.WriteToDisk()
	}
	fmt.Println("Plugin created successfully")
	return nil
}

type pluginMoveAction func(string, string)

func pluginActionBase(path string, save bool, fn pluginMoveAction) {
	path, err := filepath.Abs(path)
	checkErr(err)
	manifest := readManifest(path).(map[string]interface{})
	iterateInstallations(path, manifest, func(componentPath, componentName, installType string) {
		toSave := getComponentConfigEntry(installType)
		if *toSave == "" {
			checkErr(survey.AskOne(&survey.Input{
				Message: fmt.Sprintf(
					"Enter the path to cloned %s repository",
					componentData[installType].(map[string]interface{})["name"].(string),
				),
			}, toSave, survey.WithValidator(survey.Required), survey.WithValidator(directoryValidator), componentData[installType].(map[string]interface{})["validator"].(survey.AskOpt)))
		}
		finalPath := filepath.Join(
			*toSave,
			getOutputDirectory(installType, manifest["author"].(string), componentName),
		)
		var orgPath string
		if installType == "backend" {
			orgPath = filepath.Join(*toSave, "modules", manifest["author"].(string))
			createInitPyFile(orgPath)
		}
		fn(componentPath, finalPath)
		if installType == "backend" {
			removeOrgInitIfNoPlugins(orgPath)
		}
	})
	if save {
		rootOptions.WriteToDisk()
	}
}

func installPlugin(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return cli.ShowSubcommandHelp(cmd)
	}
	path := args.Get(0)
	isDev := cmd.Bool("dev") || args.Get(1) == "--dev" || args.Get(1) == "-D"
	save := cmd.Bool("save")
	pluginActionBase(path, save, func(componentPath, finalPath string) {
		checkErr(os.RemoveAll(finalPath))
		if !isDev {
			copyDirectory(componentPath, finalPath)
		} else {
			safeSymlink(componentPath, finalPath)
		}
	})
	return nil
}

func uninstallPlugin(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return cli.ShowSubcommandHelp(cmd)
	}
	path := args.Get(0)
	save := cmd.Bool("save")
	pluginActionBase(path, save, func(componentPath, finalPath string) {
		checkErr(os.RemoveAll(finalPath))
	})
	return nil
}

func validatePlugin(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return cli.ShowSubcommandHelp(cmd)
	}
	path := args.Get(0)
	url := args.Get(2) // after --schema part
	if url == "" {
		url = cmd.String("schema")
	}
	sch := prepareSchema(url)
	manifest := readManifest(path)
	if err := sch.Validate(manifest); err != nil {
		log.Fatalf("%#v", err)
	}
	iterateInstallations(
		path,
		manifest.(map[string]interface{}),
		func(componentPath, componentName, installType string) {
			switch installType {
			case "backend":
				pluginBase := filepath.Join(componentPath, "plugin.py")
				if !validateFileExists(pluginBase) {
					log.Fatalf(
						"Plugin's backend component %s does not include plugin.py",
						componentPath,
					)
				}
			case "admin":
				validateFrontend("admin", componentPath)
			case "store":
				validateFrontend("store", componentPath)
			}
		},
	)
	fmt.Println("Plugin is valid!")
	return nil
}

func packagePlugin(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return cli.ShowSubcommandHelp(cmd)
	}
	path := args.Get(0)
	manifest := readManifest(path).(map[string]interface{})
	noStrip := cmd.Bool("no-strip") || args.Get(1) == "--no-strip"
	checkExcludeGitignore, err := rejectGitignored([]string{path})
	checkErr(err)
	if !noStrip {
		walker := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if checkExcludeGitignore(path) {
				return os.RemoveAll(path)
			}
			return nil
		}
		checkErr(filepath.Walk(path, walker))
	}
	outPath := filepath.Join(path, manifest["name"].(string)+".bitcart")
	createZip(path, outPath)
	fmt.Println("Plugin packaged to", outPath)
	return nil
}

func updateCLI(ctx context.Context, cmd *cli.Command) error {
	slug := "bitcart/bitcart-cli"
	spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spr.Suffix = " Checking for updates..."
	spr.Start()
	check, err := CheckForUpdates(
		rootOptions.GitHubAPI,
		slug,
		Version,
	)
	spr.Stop()
	checkErr(err)
	if !check.Found {
		fmt.Println("No updates found.")
		return nil
	}
	if IsLatestVersion(check) {
		fmt.Println("Already up-to-date.")
		return nil
	}
	fmt.Println(ReportVersion(check))
	if cmd.Name == "check" {
		fmt.Println(HowToUpdate(check))
		return nil
	}
	spr.Suffix = " Installing update..."
	spr.Restart()
	message, err := InstallLatest(check)
	spr.Stop()
	checkErr(err)
	fmt.Println(message)
	return nil
}
