package main

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"charm.land/huh/v2"
)

//go:embed plugin/*
var pluginsData embed.FS

var schemaURL = "https://bitcart.ai/schemas/plugin/1.3.0/plugin.schema.json"
var envFile = "conf/.env"

var COINS = map[string]string{
	"btc":   "5000",
	"ltc":   "5001",
	"eth":   "5002",
	"bch":   "5004",
	"xrg":   "5005",
	"bnb":   "5006",
	"matic": "5008",
	"trx":   "5009",
	"grs":   "5010",
	"xmr":   "5011",
}

var componentData = map[string]any{
	"backend": map[string]any{
		"name":      "bitcart",
		"validator": backendDirectoryValidator,
	},
	"admin": map[string]any{
		"name":      "bitcart-admin",
		"validator": frontendDirectoryValidator,
	},
	"store": map[string]any{
		"name":      "bitcart-store",
		"validator": frontendDirectoryValidator,
	},
	"docker": map[string]any{
		"name":      "bitcart-docker",
		"validator": dockerDirectoryValidator,
	},
}

func basicPluginCreate(answers *BasicCreatePluginAnswers) []*huh.Group {
	return []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Enter the name for your plugin").
				Value(&answers.Name).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Title("Enter the author name for your plugin").
				Value(&answers.Author).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Title("Enter plugin description (optional)").
				Value(&answers.Description),
		).Title("Plugin details"),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select which components will your plugin update").
				Options(huh.NewOptions("backend", "admin", "store", "docker")...).
				Value(&answers.ComponentTypes),
		).Title("Components"),
	}
}

func componentGroup(componentType string, name *string, answers *BasicCreatePluginAnswers) *huh.Group {
	component := componentData[componentType].(map[string]any)
	fields := []huh.Field{
		huh.NewInput().
			Title(fmt.Sprintf("Enter name of your %s component (i.e. name of the subfolder)", componentType)).
			Value(name).
			Validate(huh.ValidateNotEmpty()),
	}
	if repositoryPath := getComponentConfigEntry(componentType); *repositoryPath == "" {
		fields = append(fields, huh.NewInput().
			Title(fmt.Sprintf("Enter the path to cloned %s repository", component["name"].(string))).
			Value(repositoryPath).
			Validate(allOf(huh.ValidateNotEmpty(), directoryValidator, component["validator"].(func(string) error))))
	}
	return huh.NewGroup(fields...).
		Title(strings.ToUpper(componentType[:1]) + componentType[1:] + " component").
		WithHideFunc(func() bool { return !slices.Contains(answers.ComponentTypes, componentType) })
}

func copyFileContents(src, dst string) {
	fileContent, err := pluginsData.ReadFile(src)
	checkErr(err)
	copyData(fileContent, dst)
}

func executeTemplate(templatePath string, data any, stripSpaces bool) []byte {
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(template.FuncMap{
		"IsLast": func(i, size int) bool { return i == size-1 },
	}).ParseFS(pluginsData, templatePath)
	checkErr(err)
	buf := new(bytes.Buffer)
	checkErr(tmpl.ExecuteTemplate(buf, filepath.Base(templatePath), &data))
	if !stripSpaces {
		return buf.Bytes()
	}
	newBuf := new(bytes.Buffer)
	removeBlankLines(bytes.NewReader(buf.Bytes()), newBuf)
	return newBuf.Bytes()
}
