package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	pivnet "github.com/pivotal-cf/go-pivnet"
	"github.com/pivotal-cf/go-pivnet/logshim"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
)

//go:generate counterfeiter -o ./fakes/pivnet_releases_service.go --fake-name PivnetReleasesService . PivnetReleasesService
type PivnetReleasesService interface {
	List(productSlug string) ([]pivnet.Release, error)
	Update(productSlug string, release pivnet.Release) (pivnet.Release, error)
}

type Publish struct {
	Options struct {
		Kilnfile    string `short:"f" long:"file" default:"Kilnfile" description:"path to Kilnfile"`
		Version     string `short:"v" long:"version-file" default:"version" description:"path to version file"`
		PivnetToken string `short:"t" long:"pivnet-token" description:"pivnet refresh token" required:"true"`
		PivnetHost  string `long:"pivnet-host" default:"https://network.pivotal.io" description:"pivnet host"`
	}

	Pivnet PivnetReleasesService

	FS  billy.Filesystem
	Now func() time.Time

	OutLogger, ErrLogger *log.Logger
}

func NewPublish(outLogger, errLogger *log.Logger, fs billy.Filesystem) Publish {
	return Publish{
		OutLogger: outLogger,
		ErrLogger: errLogger,
		FS:        fs,
	}
}

func (publish Publish) Execute(args []string) error {
	defer func() {
		if r := recover(); r != nil {
			publish.ErrLogger.Println(r)
			os.Exit(1)
		}
	}()
	_, err := jhanda.Parse(&publish.Options, args)
	if err != nil {
		return err
	}

	if publish.Now == nil {
		publish.Now = time.Now
	}

	if publish.Pivnet == nil {
		config := pivnet.ClientConfig{
			Host:      publish.Options.PivnetHost,
			UserAgent: "kiln",
		}

		tokenService := pivnet.NewAccessTokenOrLegacyToken(publish.Options.PivnetToken, publish.Options.PivnetHost)

		logger := logshim.NewLogShim(publish.OutLogger, publish.ErrLogger, false)
		client := pivnet.NewClient(tokenService, config, logger)
		publish.Pivnet = client.Releases
	}

	versionFile, err := publish.FS.Open(publish.Options.Version)
	if err != nil {
		return err
	}
	defer versionFile.Close()

	versionBuf, err := ioutil.ReadAll(versionFile)
	if err != nil {
		return err
	}

	version, err := semver.NewVersion(strings.TrimSpace(string(versionBuf)))
	if err != nil {
		return err
	}

	file, err := publish.FS.Open(publish.Options.Kilnfile)
	if err != nil {
		return err
	}
	defer file.Close()

	var kilnfile Kilnfile
	if err := yaml.NewDecoder(file).Decode(&kilnfile); err != nil {
		return fmt.Errorf("could not parse Kilnfile: %s", err)
	}

	publish.OutLogger.Printf("Requesting list of releases for %s", kilnfile.Slug)
	releases, err := publish.Pivnet.List(kilnfile.Slug)
	if err != nil {
		return err
	}

	vs := version.String()
	var release pivnet.Release
	for _, r := range releases {
		if r.Version == vs {
			release = r
			break
		}
	}

	if release.Version == "" {
		return fmt.Errorf("release with version %s not found on %s", vs, publish.Options.PivnetHost)
	}

	window, err := kilnfile.ReleaseWindow(publish.Now())
	if err != nil {
		return err
	}

	release.Version, err = publish.DetermineVersion(releases, window, version)
	if err != nil {
		return err
	}
	release.ReleaseType = releaseType(window, version)

	if _, err := publish.Pivnet.Update(kilnfile.Slug, release); err != nil {
		return err
	}

	return nil
}

func (publish Publish) DetermineVersion(releases []pivnet.Release, window string, version *semver.Version) (string, error) {
	if version.Patch() > 0 {
		publishableVersion, _ := version.SetPrerelease("")
		return publishableVersion.String(), nil
	}

	if window == "ga" {
		v, _ := version.SetPrerelease("")
		return v.String(), nil
	}

	// To allow testing times other than current time
	if publish.Now == nil {
		publish.Now = time.Now
	}

	var filteredVersions []*semver.Version
	for _, release := range releases {
		v, err := semver.NewVersion(release.Version)
		if err != nil {
			continue
		}
		if v.Major() == version.Major() && v.Minor() == version.Minor() && v.Patch() == version.Patch() {
			if pre := v.Prerelease(); !strings.HasPrefix(pre, window) {
				continue
			}
			filteredVersions = append(filteredVersions, v)
		}
	}

	if len(filteredVersions) == 0 {
		v, err := version.SetPrerelease(window + ".1")
		return v.String(), err
	}

	sort.Sort(sort.Reverse(semver.Collection(filteredVersions)))

	maxPublished := filteredVersions[0]
	segments := strings.Split(maxPublished.Prerelease(), ".")
	if len(segments) < 2 {
		return "", fmt.Errorf("expected prerelease to have a dot (%s)", maxPublished)
	}

	n, err := strconv.Atoi(segments[len(segments)-1])
	if err != nil {
		return "", fmt.Errorf("release has malformed prelease version (%s): %s", maxPublished, err)
	}

	pubVer, _ := maxPublished.SetPrerelease(strings.Join(segments[:len(segments)-1], ".") + "." + strconv.Itoa(n+1))

	return pubVer.String(), nil
}

const PublishDateFormat = "2006-01-02"

type Date struct {
	time.Time
}

// UnmarshalYAML parses a date in "YYYY-MM-DD" format
func (d *Date) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	now, err := time.ParseInLocation(PublishDateFormat, str, time.UTC)
	if err != nil {
		return err
	}

	d.Time = now
	return nil
}

type Kilnfile struct {
	Slug         string `yaml:"slug"`
	PublishDates struct {
		Alpha Date `yaml:"alpha"`
		Beta  Date `yaml:"beta"`
		RC    Date `yaml:"rc"`
		GA    Date `yaml:"ga"`
	} `yaml:"publish_dates"`
}

// ReleaseWindow determines the release window based on the current time.
func (kilnfile Kilnfile) ReleaseWindow(currentTime time.Time) (string, error) {
	gaDate := kilnfile.PublishDates.GA
	if currentTime.Equal(gaDate.Time) || currentTime.After(gaDate.Time) {
		return "ga", nil
	}

	rcDate := kilnfile.PublishDates.RC
	if currentTime.Equal(rcDate.Time) || currentTime.After(rcDate.Time) {
		return "rc", nil
	}

	betaDate := kilnfile.PublishDates.Beta
	if currentTime.Equal(betaDate.Time) || currentTime.After(betaDate.Time) {
		return "beta", nil
	}

	alphaDate := kilnfile.PublishDates.Alpha
	if currentTime.Equal(alphaDate.Time) || currentTime.After(alphaDate.Time) {
		return "alpha", nil
	}

	return "internal", nil
}

func releaseType(window string, v *semver.Version) pivnet.ReleaseType {
	switch window {
	case "rc":
		return "Release Candidate"
	case "beta":
		return "Beta Release"
	case "alpha":
		return "Alpha Release"
	case "ga":
		switch {
		case v.Minor() == 0 && v.Patch() == 0 && v.Prerelease() == "":
			return "Major Release"
		case v.Patch() == 0 && v.Prerelease() == "":
			return "Minor Release"
		case v.Prerelease() == "":
			return "Maintenance Release"
		}
		fallthrough
	case "internal":
		fallthrough
	default:
		return "Developer Release"
	}
}

// Usage writes helpful information.
func (publish Publish) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command prints helpful usage information.",
		ShortDescription: "prints this usage information",
		Flags:            publish.Options,
	}
}
