package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/go-git/go-billy/v5/osfs"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
)

func smartPrint(text string) {
	text = strings.TrimRight(text, "\r\n")
	fmt.Println(text)
}

func exitErr(err string) {
	smartPrint(err)
	os.Exit(1)
}

func checkErr(err error) {
	if err != nil {
		exitErr("Error: " + err.Error())
	}
}

func jsonEncode(data interface{}) string {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(data)
	checkErr(err)
	return string(buf.String())
}

func jsonDecodeBytes(data []byte) map[string]interface{} {
	var result map[string]interface{}
	err := json.Unmarshal(data, &result)
	checkErr(err)
	return result
}

func isBlank(str string) bool {
	for _, r := range str {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func removeBlankLines(reader io.Reader, writer io.Writer) {
	breader := bufio.NewReader(reader)
	bwriter := bufio.NewWriter(writer)
	for {
		line, err := breader.ReadString('\n')
		if !isBlank(line) {
			_, err := bwriter.WriteString(line)
			checkErr(err)
		}
		if err != nil {
			break
		}
	}
	bwriter.Flush()
}

func getCacheDir() string {
	baseDir, err := os.UserCacheDir()
	checkErr(err)
	cacheDir := filepath.Join(baseDir, "bitcart-cli")
	createIfNotExists(cacheDir, os.ModePerm)
	return cacheDir
}

func parseVersionFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		exitErr("Invalid version string provided. Only bitcart-hosted schema URLs are supported")
	}
	return parts[len(parts)-2]
}

func prepareSchema(url string) *jsonschema.Schema {
	cacheDir := getCacheDir()
	schemaPath := filepath.Join(cacheDir, "plugin.schema.json")
	versionFile := filepath.Join(cacheDir, "schema.version")
	schemaVersion := parseVersionFromURL(url)
	version, versionErr := os.ReadFile(versionFile)
	if statResult, err := os.Stat(schemaPath); os.IsNotExist(err) ||
		time.Since(
			statResult.ModTime().AddDate(0, 0, 7),
		) > time.Since(
			time.Now(),
		) || (versionErr == nil && string(version) != schemaVersion) {
		resp, err := http.Get(url)
		checkErr(err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		checkErr(err)
		checkErr(os.WriteFile(schemaPath, data, os.ModePerm))
		checkErr(
			os.WriteFile(
				filepath.Join(cacheDir, "schema.version"),
				[]byte(schemaVersion),
				os.ModePerm,
			),
		)
	}
	sch, err := jsonschema.Compile(schemaPath)
	checkErr(err)
	return sch
}

func readManifest(path string) interface{} {
	manifestPath := filepath.Join(path, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	checkErr(err)
	var manifest interface{}
	checkErr(json.Unmarshal(data, &manifest))
	return manifest
}

func getOutputDirectory(componentType string, author string, name string) string {
	if componentType == "docker" {
		return filepath.Join("compose/plugins/docker", author+"_"+name)
	}
	if componentType != "backend" {
		author = "@" + author
	}
	return filepath.Join("modules", author, name)
}

type installationProcessor func(string, string, string)

func iterateInstallations(path string, manifest map[string]interface{}, fn installationProcessor) {
	for _, installData := range manifest["installs"].([]interface{}) {
		installData := installData.(map[string]interface{})
		componentPath := filepath.Join(path, installData["path"].(string))
		componentName := filepath.Base(componentPath)
		installType := installData["type"].(string)
		fn(componentPath, componentName, installType)
	}
}

func setField(v interface{}, name string, value string) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		exitErr("v must be pointer to struct")
	}
	rv = rv.Elem()
	fv := rv.FieldByName(name)
	if !fv.IsValid() {
		exitErr(fmt.Sprintf("not a field name: %s", name))
	}
	if !fv.CanSet() {
		exitErr(fmt.Sprintf("cannot set field %s", name))
	}
	if fv.Kind() != reflect.String {
		exitErr(fmt.Sprintf("%s is not a string field", name))
	}
	fv.SetString(value)
}

func getComponentConfigEntry(componentType string) *string {
	switch componentType {
	case "backend":
		return &rootOptions.BitcartDirectory
	case "admin":
		return &rootOptions.BitcartAdminDirectory
	case "store":
		return &rootOptions.BitcartStoreDirectory
	case "docker":
		return &rootOptions.BitcartDockerDirectory
	}
	return nil
}

type RejectByNameFunc func(path string) bool

func rejectGitignored(targets []string) (RejectByNameFunc, error) {
	var patterns []gitignore.Pattern
	fs := osfs.New("/")
	for _, target := range targets {
		parts, err := pathToArray(target)
		if err != nil {
			return nil, err
		}
		patternsNow, err := gitignore.ReadPatterns(fs, parts)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, patternsNow...)
	}
	matcher := gitignore.NewMatcher(patterns)
	return func(filename string) bool {
		isDir := isDir(filename)
		p, err := pathToArray(filename)
		if err != nil {
			return false
		}
		return matcher.Match(p, isDir)
	}, nil
}

func isDir(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return false
	}
	isDir := fileInfo.IsDir()
	return isDir
}

func pathToArray(path string) ([]string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(absolute, string(filepath.Separator))
	result := []string{}
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result, nil
}
