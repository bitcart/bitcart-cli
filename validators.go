package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

func validateFileExists(path string) bool {
	return exists(path)
}

func directoryValidator(val string) error {
	statResult, err := os.Stat(val)
	if os.IsNotExist(err) {
		return errors.New("Directory does not exist")
	}
	if err != nil {
		return err
	}
	if !statResult.IsDir() {
		return errors.New("Path is not a directory")
	}
	return nil
}

func backendDirectoryValidator(val string) error {
	if !validateFileExists(filepath.Join(val, "gunicorn.conf.py")) {
		return errors.New(
			"Directory does not look to be a cloned https://github.com/bitcart/bitcart repository",
		)
	}
	return nil
}

func dockerDirectoryValidator(val string) error {
	if !validateFileExists(filepath.Join(val, "setup.sh")) {
		return errors.New(
			"Directory does not look to be a cloned https://github.com/bitcart/bitcart-docker repository",
		)
	}
	return nil
}

func frontendDirectoryValidator(val string) error {
	if !validateFileExists(filepath.Join(val, "nuxt.config.js")) {
		return errors.New("Directory does not look to be a frontend repository")
	}
	return nil
}

func validateFrontend(componentType string, path string) {
	for _, file := range []string{"index.js", "package.json", "config/index.js"} {
		if !validateFileExists(filepath.Join(path, file)) {
			log.Fatalf("Plugin's %s component %s does not include %s", componentType, path, file)
		}
	}
}
