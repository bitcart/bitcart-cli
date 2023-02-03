package main

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	// Host                   string `yaml:"host"`
	// Token                  string `yaml:"token"`
	BitcartDirectory       string `yaml:"bitcart_directory"`
	BitcartAdminDirectory  string `yaml:"bitcart_admin_directory"`
	BitcartStoreDirectory  string `yaml:"bitcart_store_directory"`
	BitcartDockerDirectory string `yaml:"bitcart_docker_directory"`
	FileUsed               string `yaml:"-"`
}

type UpdateCheck struct {
	LastUpdateCheck time.Time `yaml:"last_update_check"`
	FileUsed        string    `yaml:"-"`
}

func (upd *UpdateCheck) Load() {
	path := filepath.Join(SettingsPath(), updateCheckFilename())
	ensureSettingsFileExists(path)
	upd.FileUsed = path
	content, err := ioutil.ReadFile(path)
	checkErr(err)
	checkErr(yaml.Unmarshal(content, &upd))
}

func (upd *UpdateCheck) WriteToDisk() {
	enc, err := yaml.Marshal(&upd)
	checkErr(err)
	checkErr(ioutil.WriteFile(upd.FileUsed, enc, 0600))
}

func (cfg *Config) Load() {
	cfg.LoadFromDisk()
	cfg.LoadFromEnv("bitcart_cli")
}

func (cfg *Config) LoadFromDisk() {
	path := filepath.Join(SettingsPath(), configFilename())
	ensureSettingsFileExists(path)
	cfg.FileUsed = path
	content, err := ioutil.ReadFile(path)
	checkErr(err)
	checkErr(yaml.Unmarshal(content, &cfg))
}

func (cfg *Config) WriteToDisk() {
	enc, err := yaml.Marshal(&cfg)
	checkErr(err)
	checkErr(ioutil.WriteFile(cfg.FileUsed, enc, 0600))
}

func (cfg *Config) LoadFromEnv(prefix string) {
	for _, field := range []string{"bitcart_directory", "bitcart_admin_directory", "bitcart_store_directory", "bitcart_docker_directory"} {
		name := strings.Join([]string{prefix, field}, "_")
		if value := os.Getenv(strings.ToUpper(name)); value != "" {
			setField(cfg, field, value)
		}
	}
}

func ReadFromEnv(prefix, field string) string {
	name := strings.Join([]string{prefix, field}, "_")
	return os.Getenv(strings.ToUpper(name))
}

func updateCheckFilename() string {
	return "update_check.yml"
}

func configFilename() string {
	return "config.yml"
}

func SettingsPath() string {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".bitcart-cli")
}

func ensureSettingsFileExists(path string) {
	_, err := os.Stat(path)
	if err == nil {
		return
	}
	if !os.IsNotExist(err) {
		checkErr(err)
	}
	dir := filepath.Dir(path)
	checkErr(os.MkdirAll(dir, 0700))
	_, err = os.Create(path)
	checkErr(err)
	checkErr(os.Chmod(path, 0600))
}
