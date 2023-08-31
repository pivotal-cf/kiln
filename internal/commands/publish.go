package commands

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/go-pivnet/v7"
	"github.com/pivotal-cf/go-pivnet/v7/logshim"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	publishDateFormat = "2006-01-02"
	oslFileType       = "Open Source License"
)

//counterfeiter:generate -o ./fakes/pivnet_releases_service.go --fake-name PivnetReleasesService . PivnetReleasesService
type PivnetReleasesService interface {
	List(productSlug string, _ ...pivnet.QueryParameter) ([]pivnet.Release, error)
	Update(productSlug string, release pivnet.Release) (pivnet.Release, error)
}

//counterfeiter:generate -o ./fakes/pivnet_product_files_service.go --fake-name PivnetProductFilesService . PivnetProductFilesService
type PivnetProductFilesService interface {
	List(productSlug string) ([]pivnet.ProductFile, error)
	AddToRelease(productSlug string, releaseID int, productFileID int) error
}

//counterfeiter:generate -o ./fakes/pivnet_user_groups_service.go --fake-name PivnetUserGroupsService . PivnetUserGroupsService
type PivnetUserGroupsService interface {
	List() ([]pivnet.UserGroup, error)
	AddToRelease(productSlug string, releaseID int, userGroupID int) error
}

//counterfeiter:generate -o ./fakes/pivnet_release_upgrade_paths_service.go --fake-name PivnetReleaseUpgradePathsService . PivnetReleaseUpgradePathsService
type PivnetReleaseUpgradePathsService interface {
	Get(productSlug string, releaseID int) ([]pivnet.ReleaseUpgradePath, error)
}

//counterfeiter:generate -o ./fakes/pivnet_release_dependencies_service.go --fake-name PivnetReleaseDependenciesService . PivnetReleaseDependenciesService
type PivnetReleaseDependenciesService interface {
	List(productSlug string, releaseID int) ([]pivnet.ReleaseDependency, error)
}

type Publish struct {
	Options struct {
		Kilnfile            string `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Version             string `short:"v" long:"version-file" default:"version" description:"path to version file"`
		PivnetToken         string `short:"t" long:"pivnet-token" description:"pivnet refresh token" required:"true"`
		PivnetHost          string `long:"pivnet-host" default:"https://network.pivotal.io" description:"pivnet host"`
		IncludesSecurityFix bool   `long:"security-fix" description:"the release includes security fixes"`
		Window              string `long:"window" required:"true"`
	}

	PivnetReleaseService             PivnetReleasesService
	PivnetProductFilesService        PivnetProductFilesService
	PivnetUserGroupsService          PivnetUserGroupsService
	PivnetReleaseUpgradePathsService PivnetReleaseUpgradePathsService
	PivnetReleaseDependenciesService PivnetReleaseDependenciesService

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

func (p Publish) Execute(args []string) error {
	defer p.recoverFromPanic()

	kilnfile, buildVersion, err := p.parseArgsAndSetup(args)
	if err != nil {
		return err
	}

	err = p.updateReleaseOnPivnet(kilnfile, buildVersion)
	if err != nil {
		return fmt.Errorf("failed to publish tile: %s", err)
	} else {
		p.OutLogger.Println("Successfully published tile.")
	}
	return nil
}

func (p Publish) recoverFromPanic() func() {
	return func() {
		if r := recover(); r != nil {
			p.ErrLogger.Println(r)
			os.Exit(1)
		}
	}
}

func (p *Publish) parseArgsAndSetup(args []string) (cargo.Kilnfile, *semver.Version, error) {
	_, err := jhanda.Parse(&p.Options, args)
	if err != nil {
		return cargo.Kilnfile{}, nil, err
	}

	if p.Now == nil {
		p.Now = time.Now
	}

	if p.PivnetReleaseService == nil || p.PivnetProductFilesService == nil || p.PivnetUserGroupsService == nil || p.PivnetReleaseUpgradePathsService == nil || p.PivnetReleaseDependenciesService == nil {
		config := pivnet.ClientConfig{
			Host:      p.Options.PivnetHost,
			UserAgent: "kiln",
		}

		tokenService := pivnet.NewAccessTokenOrLegacyToken(p.Options.PivnetToken, p.Options.PivnetHost, false)

		logger := logshim.NewLogShim(p.OutLogger, p.ErrLogger, false)
		client := pivnet.NewClient(tokenService, config, logger)
		client.HTTP.Timeout = 60 * 5 * time.Second

		if p.PivnetReleaseService == nil {
			p.PivnetReleaseService = client.Releases
		}

		if p.PivnetProductFilesService == nil {
			p.PivnetProductFilesService = client.ProductFiles
		}

		if p.PivnetUserGroupsService == nil {
			p.PivnetUserGroupsService = client.UserGroups
		}

		if p.PivnetReleaseUpgradePathsService == nil {
			p.PivnetReleaseUpgradePathsService = client.ReleaseUpgradePaths
		}

		if p.PivnetReleaseDependenciesService == nil {
			p.PivnetReleaseDependenciesService = client.ReleaseDependencies
		}
	}

	versionFile, err := p.FS.Open(p.Options.Version)
	if err != nil {
		return cargo.Kilnfile{}, nil, err
	}
	defer closeAndIgnoreError(versionFile)

	versionBuf, err := io.ReadAll(versionFile)
	if err != nil {
		return cargo.Kilnfile{}, nil, err
	}

	version, err := semver.NewVersion(strings.TrimSpace(string(versionBuf)))
	if err != nil {
		return cargo.Kilnfile{}, nil, err
	}

	file, err := p.FS.Open(p.Options.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, nil, err
	}
	defer closeAndIgnoreError(file)

	var kilnfile cargo.Kilnfile
	if err := yaml.NewDecoder(file).Decode(&kilnfile); err != nil {
		return cargo.Kilnfile{}, nil, fmt.Errorf("could not parse Kilnfile: %s", err)
	}

	window := p.Options.Window
	if window != "ga" && window != "rc" && window != "beta" && window != "alpha" {
		return cargo.Kilnfile{}, nil, fmt.Errorf("unknown window: %q", window)
	}

	return kilnfile, version, nil
}

func (p Publish) updateReleaseOnPivnet(kilnfile cargo.Kilnfile, buildVersion *semver.Version) error {
	p.OutLogger.Printf("Requesting list of releases for %s", kilnfile.Slug)

	window := p.Options.Window

	rv, err := ReleaseVersionFromBuildVersion(buildVersion, window)
	if err != nil {
		return err
	}

	releaseType := releaseType(window, p.Options.IncludesSecurityFix, rv)

	var releases releaseSet
	releases, err = p.PivnetReleaseService.List(kilnfile.Slug)
	if err != nil {
		return err
	}

	versionToPublish, err := p.determineVersion(releases, rv)
	if err != nil {
		return err
	}

	_, err = releases.Find(versionToPublish.String())
	if err == nil {
		return fmt.Errorf("release %s already exists", versionToPublish.String())
	}

	release, err := releases.Find(buildVersion.String())
	if err != nil {
		return err
	}

	licenseFileNames, err := p.attachLicenseFiles(kilnfile.Slug, release.ID, versionToPublish)
	if err != nil {
		if len(licenseFileNames) > 0 {
			p.ErrLogger.Println("Attached the following license files before failure:")
			p.printLicenseFiles(licenseFileNames, p.ErrLogger)
		}
		return err
	}

	upgradePaths, err := p.PivnetReleaseUpgradePathsService.Get(kilnfile.Slug, release.ID)
	if err != nil {
		return err
	}

	if len(upgradePaths) == 0 {
		return fmt.Errorf("no upgrade paths set for %s", release.Version)
	}

	dependencies, err := p.PivnetReleaseDependenciesService.List(kilnfile.Slug, release.ID)
	if err != nil {
		return err
	}

	if len(dependencies) == 0 {
		return fmt.Errorf("no dependencies set for %s", release.Version)
	}

	endOfSupportDate, err := p.eogsDate(rv, releases)
	if err != nil {
		return err
	}

	var availability string
	if rv.IsGA() {
		availability = "All Users"
	} else {
		availability = "Selected User Groups Only"
	}

	releaseDate := p.Now().Format(publishDateFormat)
	updatedRelease, err := p.updateRelease(release, kilnfile.Slug, versionToPublish.String(), releaseType, releaseDate, endOfSupportDate, availability, licenseFileNames)
	if err != nil {
		return err
	}

	err = p.addUserGroups(rv, updatedRelease, kilnfile)
	if err != nil {
		return err
	}

	return nil
}

func (p Publish) eogsDate(rv *releaseVersion, releases releaseSet) (string, error) {
	if rv.IsGA() {
		sameMajorAndMinor, err := rv.MajorMinorConstraint()
		if err != nil {
			return "", err
		}

		lastPatchRelease, matchExists, err := releases.FindLatest(sameMajorAndMinor)
		if err != nil {
			return "", err
		}

		if !matchExists {
			return endOfSupportFor(p.Now()), nil
		} else if lastPatchRelease.EndOfSupportDate != "" {
			return lastPatchRelease.EndOfSupportDate, nil
		} else {
			return "", fmt.Errorf("previously published release %q does not have an End of General Support date", lastPatchRelease.Version)
		}
	}
	return "", nil
}

func (p Publish) printLicenseFiles(licenseFileNames []string, logger *log.Logger) {
	for _, licenseFileName := range licenseFileNames {
		logger.Printf("  License file: %s\n", licenseFileName)
	}
}

func (p Publish) updateRelease(release pivnet.Release, slug, version string, releaseType pivnet.ReleaseType, releaseDate, endOfSupportDate, availability string, licenseFileNames []string) (pivnet.Release, error) {
	p.OutLogger.Println("Updating product record on PivNet...")
	p.OutLogger.Printf("  Version: %s\n", version)
	p.OutLogger.Printf("  Release date: %s\n", releaseDate)
	p.OutLogger.Printf("  Release type: %s\n", releaseType)
	if endOfSupportDate != "" {
		p.OutLogger.Printf("  EOGS date: %s\n", endOfSupportDate)
	}
	p.OutLogger.Printf("  Availability: %s\n", availability)
	if len(licenseFileNames) > 0 {
		p.printLicenseFiles(licenseFileNames, p.OutLogger)
	} else {
		p.OutLogger.Printf("  License file: None, pre-GA release")
	}
	release.Version = version
	release.ReleaseType = releaseType
	release.ReleaseDate = releaseDate
	release.EndOfSupportDate = endOfSupportDate
	release.Availability = availability
	updatedRelease, err := p.PivnetReleaseService.Update(slug, release)
	if err != nil {
		return pivnet.Release{}, err
	}
	return updatedRelease, nil
}

func (p Publish) addUserGroups(rv *releaseVersion, release pivnet.Release, kilnfile cargo.Kilnfile) error {
	if rv.IsGA() {
		return nil
	}

	var (
		err           error
		allUserGroups []pivnet.UserGroup
		errorCount    int
	)
listUserGroupsRetryLoop:
	for {
		allUserGroups, err = p.PivnetUserGroupsService.List()
		if err != nil && errorCount < 5 {
			if !strings.HasSuffix(err.Error(), "net/http: request canceled (Client.Timeout exceeded while awaiting headers)") {
				return err
			}
			errorCount++
			p.ErrLogger.Printf("failed list user groups with error: %s", err)
			p.OutLogger.Printf("retrying list command (%d retries so far)", errorCount)
			continue listUserGroupsRetryLoop
		}
		break listUserGroupsRetryLoop
	}

	p.OutLogger.Println("Granting access to groups...")
	for _, userGroupName := range kilnfile.PreGaUserGroups {
		p.OutLogger.Printf("  - %s\n", userGroupName)
		groupFound := false
		for _, userGroup := range allUserGroups {
			if userGroup.Name == userGroupName {
				err := p.PivnetUserGroupsService.AddToRelease(kilnfile.Slug, release.ID, userGroup.ID)
				if err != nil {
					return err
				}
				groupFound = true
				break
			}
		}
		if !groupFound {
			return fmt.Errorf("no matching user group %q on Pivnet", userGroupName)
		}
	}
	return nil
}

func endOfSupportFor(publishDate time.Time) string {
	monthWithOverflow := publishDate.Month() + 10
	month := ((monthWithOverflow - 1) % 12) + 1
	yearDelta := int((monthWithOverflow - 1) / 12)
	startOfTenthMonth := time.Date(publishDate.Year()+yearDelta, month, 1, 0, 0, 0, 0, publishDate.Location())
	endOfNinthMonth := startOfTenthMonth.Add(-24 * time.Hour)
	return endOfNinthMonth.Format(publishDateFormat)
}

func (p Publish) attachLicenseFiles(slug string, releaseID int, version *releaseVersion) ([]string, error) {
	var attachedLicenseFileNames []string

	if !version.IsGA() {
		return nil, nil
	}

	productFiles, err := p.PivnetProductFilesService.List(slug)
	if err != nil {
		return nil, err
	}

	licenseFiles := findMatchingLicenseFiles(productFiles, version)
	if len(licenseFiles) == 0 {
		return nil, errors.New("required license file doesn't exist on Pivnet")
	}

	for _, licenseFile := range licenseFiles {
		if err := p.PivnetProductFilesService.AddToRelease(slug, releaseID, licenseFile.ID); err != nil {
			return attachedLicenseFileNames, err
		}
		attachedLicenseFileNames = append(attachedLicenseFileNames, licenseFile.Name)
	}
	return attachedLicenseFileNames, nil
}

func findMatchingLicenseFiles(productFiles []pivnet.ProductFile, version *releaseVersion) []pivnet.ProductFile {
	var matchingLicenseFiles []pivnet.ProductFile

	for _, file := range productFiles {
		if file.FileType == oslFileType && file.FileVersion == version.MajorAndMinor() {
			matchingLicenseFiles = append(matchingLicenseFiles, file)
		}
	}
	return matchingLicenseFiles
}

func (p Publish) determineVersion(releases releaseSet, version *releaseVersion) (*releaseVersion, error) {
	if version.IsGA() {
		return version, nil
	}

	constraint, err := version.PrereleaseVersionsConstraint()
	if err != nil {
		return nil, fmt.Errorf("determineVersion: error building prerelease version constraint: %w", err)
	}

	latestRelease, previousReleaseExists, err := releases.FindLatest(constraint)
	if err != nil {
		return nil, fmt.Errorf("determineVersion: error finding the latest release: %w", err)
	}
	if !previousReleaseExists {
		return version, nil
	}

	maxPublishedVersion, err := ReleaseVersionFromPublishedVersion(latestRelease.Version)
	if err != nil {
		return nil, fmt.Errorf("determineVersion: error parsing release version: %w", err)
	}

	version, err = version.SetPrereleaseVersion(maxPublishedVersion.PrereleaseVersion() + 1)
	if err != nil {
		return nil, err
	}

	return version, nil
}

func releaseType(window string, includesSecurityFix bool, v *releaseVersion) pivnet.ReleaseType {
	switch window {
	case "rc":
		return "Release Candidate"
	case "beta":
		return "Beta Release"
	case "alpha":
		return "Alpha Release"
	case "ga":
		switch {
		case v.IsMajor():
			return "Major Release"
		case v.IsMinor():
			return "Minor Release"
		default:
			if includesSecurityFix {
				return "Security Release"
			} else {
				return "Maintenance Release"
			}
		}
	default:
		return "Developer Release"
	}
}

// Usage writes helpful information.
func (p Publish) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "UpdateStemcell release date, end of general support date, and license files for a tile on Pivnet.",
		ShortDescription: "publish tile on Pivnet",
		Flags:            p.Options,
	}
}

type releaseSet []pivnet.Release

func (rs releaseSet) Find(version string) (pivnet.Release, error) {
	for _, r := range rs {
		if r.Version == version {
			return r, nil
		}
	}

	return pivnet.Release{}, fmt.Errorf("release with version %s not found", version)
}

func (rs releaseSet) FindLatest(constraint *semver.Constraints) (pivnet.Release, bool, error) {
	var matches []pivnet.Release
	for _, release := range rs {
		v, err := semver.NewVersion(release.Version)
		if err != nil {
			continue
		}

		if constraint.Check(v) {
			matches = append(matches, release)
		}
	}

	if len(matches) == 0 {
		return pivnet.Release{}, false, nil
	}

	sort.Slice(matches, func(i, j int) bool {
		v1 := semver.MustParse(matches[i].Version)
		v2 := semver.MustParse(matches[j].Version)
		return v1.LessThan(v2)
	})

	return matches[len(matches)-1], true, nil
}

type releaseVersion struct {
	semver            semver.Version
	window            string
	prereleaseVersion int
}

func ReleaseVersionFromBuildVersion(baseVersion *semver.Version, window string) (*releaseVersion, error) {
	v2, err := baseVersion.SetPrerelease("")
	if err != nil {
		return nil, fmt.Errorf("ReleaseVersionFromBuildVersion: error clearing prerelease of %q: %w", v2, err)
	}

	rv := &releaseVersion{semver: v2, window: window, prereleaseVersion: 0}

	if window != "ga" {
		rv, err = rv.SetPrereleaseVersion(1)
		if err != nil {
			return nil, fmt.Errorf("ReleaseVersionFromBuildVersion: error setting prerelease of %q to 1: %w", rv, err)
		}
	}
	return rv, nil
}

func ReleaseVersionFromPublishedVersion(versionString string) (*releaseVersion, error) {
	version, err := semver.NewVersion(versionString)
	if err != nil {
		return nil, fmt.Errorf("ReleaseVersionFromPublishedVersion: unable to parse version %q: %w", versionString, err)
	}
	segments := strings.Split(version.Prerelease(), ".")
	if len(segments) != 2 {
		return nil, fmt.Errorf("ReleaseVersionFromPublishedVersion: expected prerelease to have a dot (%q)", version)
	}

	window := segments[0]
	prereleaseVersion, err := strconv.Atoi(segments[len(segments)-1])
	if err != nil {
		return nil, fmt.Errorf("ReleaseVersionFromPublishedVersion: release has malformed prelease version (%s): %w", version, err)
	}

	return &releaseVersion{
		semver:            *version,
		window:            window,
		prereleaseVersion: prereleaseVersion,
	}, nil
}

func (rv releaseVersion) MajorMinorConstraint() (*semver.Constraints, error) {
	return semver.NewConstraint(fmt.Sprintf("~%d.%d.0", rv.semver.Major(), rv.semver.Minor()))
}

func (rv releaseVersion) PrereleaseVersionsConstraint() (*semver.Constraints, error) {
	if rv.IsGA() {
		return nil, fmt.Errorf("can't determine PrereleaseVersionsConstraint for %q, which is GA", rv.semver)
	}
	coreVersion := fmt.Sprintf("%d.%d.%d-%s", rv.semver.Major(), rv.semver.Minor(), rv.semver.Patch(), rv.window)
	constraintStr := fmt.Sprintf(">= %s.0, <= %s.9999", coreVersion, coreVersion)
	return semver.NewConstraint(constraintStr)
}

func (rv releaseVersion) SetPrereleaseVersion(prereleaseVersion int) (*releaseVersion, error) {
	if rv.IsGA() {
		return nil, fmt.Errorf("SetPrereleaseVersion: can't set the prerelease version on a GA version (%q)", rv.String())
	}
	v, err := rv.semver.SetPrerelease(fmt.Sprintf("%s.%d", rv.window, prereleaseVersion))
	if err != nil {
		return nil, fmt.Errorf("SetPrereleaseVersion: couldn't set prerelease: %w", err)
	}
	rv.semver = v
	rv.prereleaseVersion = prereleaseVersion

	return &rv, nil
}

func (rv releaseVersion) IsGA() bool {
	return rv.window == "ga"
}

func (rv releaseVersion) IsMajor() bool {
	return rv.semver.Minor() == 0 && rv.semver.Patch() == 0
}

func (rv releaseVersion) IsMinor() bool {
	return rv.semver.Minor() != 0 && rv.semver.Patch() == 0
}

func (rv releaseVersion) String() string {
	return rv.semver.String()
}

func (rv releaseVersion) MajorAndMinor() string {
	return fmt.Sprintf("%d.%d", rv.semver.Major(), rv.semver.Minor())
}

func (rv releaseVersion) Semver() *semver.Version {
	return &rv.semver
}

func (rv releaseVersion) PrereleaseVersion() int {
	return rv.prereleaseVersion
}
