package cargo

import (
	"github.com/Masterminds/semver/v3"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBOSHReleaseTarballLock_yaml_marshal_order(t *testing.T) {
	const validBOSHReleaseTarballLockYaml = `name: fake-component-name
sha1: fake-component-sha1
version: fake-version
remote_source: fake-source
remote_path: fake/path/to/fake-component-name
`
	damnit := NewWithT(t)

	cl, err := yaml.Marshal(BOSHReleaseTarballLock{
		Name:         "fake-component-name",
		Version:      "fake-version",
		SHA1:         "fake-component-sha1",
		RemoteSource: "fake-source",
		RemotePath:   "fake/path/to/fake-component-name",
	})

	damnit.Expect(err).NotTo(HaveOccurred())
	damnit.Expect(string(cl)).To(Equal(validBOSHReleaseTarballLockYaml))
}

func TestKilnfileLock_UpdateBOSHReleaseTarballLockWithName(t *testing.T) {
	type args struct {
		name string
		lock BOSHReleaseTarballLock
	}
	tests := []struct {
		name                         string
		KilnfileLock, KilnfileResult KilnfileLock
		args                         args
		wantErr                      bool
	}{
		{name: "empty inputs", wantErr: true},

		{
			name: "lock with name found",
			KilnfileLock: KilnfileLock{
				Releases: []BOSHReleaseTarballLock{
					{Name: "banana"},
				},
			},
			KilnfileResult: KilnfileLock{
				Releases: []BOSHReleaseTarballLock{
					{Name: "orange", Version: "some-version"},
				},
			},
			args: args{
				name: "banana", lock: BOSHReleaseTarballLock{Name: "orange", Version: "some-version"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.KilnfileLock.UpdateBOSHReleaseTarballLockWithName(tt.args.name, tt.args.lock); tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.KilnfileResult, tt.KilnfileLock)
		})
	}
}

func TestBoshReleaseTarballSpecification_VersionConstraints(t *testing.T) {

	tests := []struct {
		name                                string
		boshReleaseTarball                  BOSHReleaseTarballSpecification
		constraintsResult, constraintsEmpty semver.Constraints
		version                             string
		wantErr                             bool
	}{
		{
			name:               "empty version",
			boshReleaseTarball: BOSHReleaseTarballSpecification{},
			version:            "*",
			wantErr:            false,
		},
		{
			name: "with valid semver version",
			boshReleaseTarball: BOSHReleaseTarballSpecification{
				Version: "~>1.2.3",
			},
			version: "~>1.2.3",
			wantErr: false,
		},
		{
			name: "with valid semver version",
			boshReleaseTarball: BOSHReleaseTarballSpecification{
				Version: "fdkjdfj",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, err := tt.boshReleaseTarball.VersionConstraints()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				constraintsResult, _ := semver.NewConstraint(tt.version)
				assert.NoError(t, err)
				assert.Equal(t, constraint, constraintsResult)
			}
		})
	}
}
func TestBoshReleaseTarballLock_ParseVersion(t *testing.T) {
	tests := []struct {
		name               string
		boshReleaseTarball BOSHReleaseTarballLock
		wantErr            bool
		version            string
	}{
		{
			name:               "empty version",
			boshReleaseTarball: BOSHReleaseTarballLock{},
			wantErr:            true,
		},
		{
			name: "with valid semver version",
			boshReleaseTarball: BOSHReleaseTarballLock{
				Version: "3.0.0",
			},
			version: "3.0.0",
			wantErr: false,
		},
		{
			name: "with invalid semver version",
			boshReleaseTarball: BOSHReleaseTarballLock{
				Version: "fdkjdfj",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, err := tt.boshReleaseTarball.ParseVersion()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				constraintsResult, _ := semver.NewVersion(tt.version)
				assert.NoError(t, err)
				assert.Equal(t, constraint, constraintsResult)
			}
		})
	}
}
func TestBoshReleaseTarballLock_String(t *testing.T) {
	tests := []struct {
		name               string
		boshReleaseTarball BOSHReleaseTarballLock
	}{
		{
			name: "with valid Bosh Release tarball lock",
			boshReleaseTarball: BOSHReleaseTarballLock{
				Version:         "3.0.0",
				Name:            "some-name",
				SHA1:            "some-sha",
				StemcellOS:      "some-stemcell",
				StemcellVersion: "some-version",
				RemoteSource:    "some-remote-source",
				RemotePath:      "some-remote-path",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.boshReleaseTarball.String()
			expected := "some-name 3.0.0  some-stemcell some-version some-remote-source some-remote-path "
			assert.Equal(t, expected, result)
		})
	}
}

func TestStemcell_ProductSlug(t *testing.T) {
	for _, tt := range []struct {
		Name                     string
		Stemcell                 Stemcell
		ExpSlug, ExpErrSubstring string
	}{
		{
			Name:     "when using known os ubuntu-xenial",
			Stemcell: Stemcell{OS: "ubuntu-xenial"},
			ExpSlug:  "stemcells-ubuntu-xenial",
		},
		{
			Name:     "when using known os ubuntu-jammy",
			Stemcell: Stemcell{OS: "ubuntu-jammy"},
			ExpSlug:  "stemcells-ubuntu-jammy",
		},
		{
			Name:     "when using known os windows2019",
			Stemcell: Stemcell{OS: "windows2019"},
			ExpSlug:  "stemcells-windows-server",
		},
		{
			Name:            "when slug is not set",
			Stemcell:        Stemcell{OS: "orange"},
			ExpErrSubstring: "stemcell slug not set",
		},
		{
			Name:     "when slug is set",
			Stemcell: Stemcell{TanzuNetSlug: "naval-orange"},
			ExpSlug:  "naval-orange",
		},
		{
			Name:     "when slug is set and os is a known value windows2019",
			Stemcell: Stemcell{OS: "windows2019", TanzuNetSlug: "naval-orange"},
			ExpSlug:  "naval-orange",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			productSlug, err := tt.Stemcell.ProductSlug()
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.ExpSlug, productSlug)
			}
		})
	}
}
