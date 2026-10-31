package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/briandowns/spinner"
	"github.com/creativeprojects/go-selfupdate"
)

type Options struct {
	Current *semver.Version
	Found   bool
	Latest  *selfupdate.Release

	updater   *selfupdate.Updater
	githubAPI string
	slug      string
}

var releaseArch = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
	"386":   "i686",
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

func checkForUpdates(ctx context.Context, opts *Config) error {
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
			ctx,
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

func CheckForUpdates(ctx context.Context, githubAPI, slug, current string) (*Options, error) {
	var (
		err   error
		check *Options
	)
	currentVersion, err := semver.StrictNewVersion(current)
	if err != nil {
		return nil, errors.New("Failed to parse current version: " + err.Error())
	}
	check = &Options{
		Current: currentVersion,

		githubAPI: githubAPI,
		slug:      slug,
	}
	err = checkFromSource(ctx, check)
	return check, err
}

func checkFromSource(ctx context.Context, check *Options) error {
	sourceConfig := selfupdate.GitHubConfig{}
	if check.githubAPI != "https://api.github.com" {
		sourceConfig.EnterpriseBaseURL = check.githubAPI
	}
	source, err := selfupdate.NewGitHubSource(sourceConfig)
	if err != nil {
		return err
	}
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Arch:      releaseArch[runtime.GOARCH],
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return err
	}
	check.updater = updater
	err = latestRelease(ctx, check)
	return err
}

func latestRelease(ctx context.Context, opts *Options) error {
	latest, found, err := opts.updater.DetectLatest(ctx, selfupdate.ParseSlug(opts.slug))
	opts.Latest = latest
	opts.Found = found
	if err != nil {
		return errors.New(`Failed to query the GitHub API for updates.
This is most likely due to GitHub rate-limiting on unauthenticated requests.
To have the bitcart-cli make authenticated requests please:
  1. Generate a token at https://github.com/settings/tokens
  2. Set the token in the GITHUB_TOKEN environment variable.
Instructions for generating a token can be found at:
https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
We call the GitHub releases API to look for new releases.
More information about that API can be found here: https://developer.github.com/v3/repos/releases/
` + err.Error())
	}
	return nil
}

func IsLatestVersion(opts *Options) bool {
	if opts.Current == nil || opts.Latest == nil {
		return true
	}
	return opts.Latest.LessOrEqual(opts.Current.String())
}

func ReportVersion(opts *Options) string {
	return strings.Join([]string{
		fmt.Sprintf("You are running %s", opts.Current),
		fmt.Sprintf("A new release is available (%s)", opts.Latest.Version()),
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

func InstallLatest(ctx context.Context, opts *Options) (string, error) {
	release, err := opts.updater.UpdateSelf(
		ctx,
		opts.Current.String(),
		selfupdate.ParseSlug(opts.slug),
	)
	if err != nil {
		return "", errors.New("Failed to install update: " + err.Error())
	}
	return fmt.Sprintf("Updated to %s", release.Version()), nil
}
