package fetcher

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

var repos = []string{
	"cloudfoundry",
	"pivotal-cf",
	"cloudfoundry-incubator",
	"pivotal-cf-experimental",
	"bosh-packages",
	"cppforlife",
	"vito",
	"flavorjones",
	"xoebus",
	"dpb587",
	"jamlo",
	"concourse",
	"cf-platform-eng",
	"starkandwayne",
	"cloudfoundry-community",
	"vmware",
	"DataDog",
	"Dynatrace",
	"SAP",
	"hybris",
	"minio",
	"rakutentech",
	"frodenas",
}

var suffixes = []string{
	"-release",
	"-boshrelease",
	"-bosh-release",
	"",
}

func ReleaseExistOnBoshio(name string) bool {
	resp, err := http.Get("https://bosh.io/api/v1/releases/github.com/" + name)
	if err != nil {
		fmt.Errorf("Bosh.io API is down with error: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) == "null" {
		return false
	} else {
		return true
	}
}

type BOSHIOReleaseSource struct {
	logger *log.Logger
}

func NewBOSHIOReleaseSource(logger *log.Logger) BOSHIOReleaseSource {
	return BOSHIOReleaseSource{logger}
}

func (r BOSHIOReleaseSource) Configure(assets cargo.Assets) {
	return
}

func (r BOSHIOReleaseSource) GetMatchedReleases(assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	matchedBOSHIOReleases := make(map[cargo.CompiledRelease]string)
	releases := assetsLock.Releases

	missingReleases := make([]cargo.CompiledRelease, 0)

	for _, release := range releases {
		compRelease := cargo.CompiledRelease{
			Name:            release.Name,
			Version:         release.Version,
			StemcellOS:      assetsLock.Stemcell.OS,
			StemcellVersion: assetsLock.Stemcell.Version,
		}
		exists := false
	found:
		for _, repo := range repos {
			for _, suf := range suffixes {
				fullName := repo + "/" + release.Name + suf
				exists = ReleaseExistOnBoshio(fullName)
				if exists {
					matchedBOSHIOReleases[compRelease] = "https://bosh.io/api/v1/releases/github.com/" + fullName
					break found
				}
			}
		}
		if !exists {
			missingReleases = append(missingReleases, compRelease)
		}
	}

	return matchedBOSHIOReleases, missingReleases, nil //no foreseen error to return to a higher level
}

func (r BOSHIOReleaseSource) DownloadReleases(releaseDir string, matchedBOSHObjects map[cargo.CompiledRelease]string, downloadThreads int) error {
	return nil
}
