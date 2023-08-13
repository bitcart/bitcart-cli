package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bitcart/go-github-selfupdate/selfupdate"
	"github.com/blang/semver"
	"github.com/briandowns/spinner"
)

type Options struct {
	Current semver.Version
	Found   bool
	Latest  *selfupdate.Release

	updater   *selfupdate.Updater
	githubAPI string
	slug      string
}

// hoursBeforeCheck is used to configure the delay between auto-update checks
var hoursBeforeCheck = 28

func ShouldCheckForUpdates(upd *UpdateCheck) bool {
	diff := time.Since(upd.LastUpdateCheck)
	return diff.Hours() >= float64(hoursBeforeCheck)
}

func updatesEnabled() bool {
	return Version != "dev" && Version != "docker"
}

func skipUpdateByDefault() bool {
	return !updatesEnabled() || os.Getenv("CI") == "true" ||
		os.Getenv("BITCART_CLI_SKIP_UPDATE_CHECK") == "true"
}

func checkForUpdates(opts *Config) error {
	if opts.SkipUpdateCheck {
		return nil
	}
	updateCheck := &UpdateCheck{
		LastUpdateCheck: time.Time{},
	}
	updateCheck.Load()
	if ShouldCheckForUpdates(updateCheck) {
		log := log.New(os.Stderr, "", 0)
		slug := "bitcart/bitcart-cli"
		spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		spr.Writer = os.Stderr
		spr.Suffix = " Checking for updates..."
		spr.Start()
		check, err := CheckForUpdates(
			opts.GitHubAPI,
			slug,
			Version,
		)
		if err != nil {
			spr.Stop()
			return err
		}
		if !check.Found {
			spr.Suffix = "No updates found."
			time.Sleep(300 * time.Millisecond)
			spr.Stop()
			updateCheck.LastUpdateCheck = time.Now()
			updateCheck.WriteToDisk()
			return nil
		}
		if IsLatestVersion(check) {
			spr.Suffix = "Already up-to-date."
			time.Sleep(300 * time.Millisecond)
			spr.Stop()
			updateCheck.LastUpdateCheck = time.Now()
			updateCheck.WriteToDisk()
			return nil
		}
		spr.Stop()
		log.Println(ReportVersion(check))
		log.Println(HowToUpdate(check))
		log.Println("") // Print a new-line after all of that
		updateCheck.LastUpdateCheck = time.Now()
		updateCheck.WriteToDisk()
		return nil
	}
	return nil
}

func CheckForUpdates(githubAPI, slug, current string) (*Options, error) {
	var (
		err   error
		check *Options
	)
	currentVersion, err := semver.Parse(current)
	if err != nil {
		return nil, errors.New("Failed to parse current version: " + err.Error())
	}
	check = &Options{
		Current: currentVersion,

		githubAPI: githubAPI,
		slug:      slug,
	}
	err = checkFromSource(check)
	return check, err
}

func checkFromSource(check *Options) error {
	updateConfig := selfupdate.Config{}
	if check.githubAPI != "https://api.github.com" {
		updateConfig.EnterpriseBaseURL = check.githubAPI
	}
	updater, err := selfupdate.NewUpdater(updateConfig)
	if err != nil {
		return err
	}
	check.updater = updater
	err = latestRelease(check)
	return err
}

func latestRelease(opts *Options) error {
	latest, found, err := opts.updater.DetectLatest(opts.slug)
	opts.Latest = latest
	opts.Found = found
	if err != nil {
		return errors.New(`Failed to query the GitHub API for updates.
This is most likely due to GitHub rate-limiting on unauthenticated requests.
To have the bitcart-cli make authenticated requests please:
  1. Generate a token at https://github.com/settings/tokens
  2. Set the token by either adding it to your ~/.gitconfig or
     setting the GITHUB_TOKEN environment variable.
Instructions for generating a token can be found at:
https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
We call the GitHub releases API to look for new releases.
More information about that API can be found here: https://developer.github.com/v3/repos/releases/
` + err.Error())
	}
	return nil
}

func IsLatestVersion(opts *Options) bool {
	if opts.Current.String() == "" || opts.Latest == nil {
		return true
	}
	return opts.Latest.Version.Equals(opts.Current)
}

func ReportVersion(opts *Options) string {
	return strings.Join([]string{
		fmt.Sprintf("You are running %s", opts.Current),
		fmt.Sprintf("A new release is available (%s)", opts.Latest.Version),
	}, "\n")
}

func HowToUpdate(opts *Options) string {
	switch Version {
	case "docker", "dev":
		return "Automatic updates are disabled"
	default:
		return "You can update with `bitcart-cli update install`"
	}
}

func InstallLatest(opts *Options) (string, error) {
	release, err := opts.updater.UpdateSelf(opts.Current, opts.slug)
	if err != nil {
		return "", errors.New("Failed to install update: " + err.Error())
	}
	return fmt.Sprintf("Updated to %s", release.Version), nil
}
