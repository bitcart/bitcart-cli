package main

import (
	"bytes"
	"embed"
	"path/filepath"
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

func basicPluginCreate(answers *BasicCreatePluginAnswers) []huh.Field {
	return []huh.Field{
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
		huh.NewMultiSelect[string]().
			Title("Select which components will your plugin update").
			Options(huh.NewOptions("backend", "admin", "store", "docker")...).
			Value(&answers.ComponentTypes),
	}
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
