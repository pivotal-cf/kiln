package cargo

import (
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

func TestKilnfile_Glaze(t *testing.T) {
	t.Run("locks versions to the value in the lock", func(t *testing.T) {
		kf := Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "banana"},
				{Name: "orange", Version: "~ 8.0"},
			},
			Stemcell: Stemcell{
				OS: "alpine",
			},
		}
		kl := KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "banana", Version: "1.2.3"},
				{Name: "orange", Version: "8.0.8"},
			},
			Stemcell: Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		}

		require.NoError(t, kf.Glaze(kl))

		assert.Equal(t, Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "banana", Version: "1.2.3"},
				{Name: "orange", Version: "8.0.8"},
			},
			Stemcell: Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		}, kf)
	})

	t.Run("when float always is true", func(t *testing.T) {
		kf := Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "orange", Version: "<3", FloatAlways: true},
			},
			Stemcell: Stemcell{
				OS: "alpine",
			},
		}
		kl := KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "orange", Version: "2.3.4"},
			},
			Stemcell: Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		}

		require.NoError(t, kf.Glaze(kl))

		assert.Equal(t, Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "orange", Version: "<3", FloatAlways: true},
			},
			Stemcell: Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		}, kf, "it does not alter the versio constraint")
	})
}

func TestKilnfile_DeGlaze(t *testing.T) {
	t.Run("when de glazing", func(t *testing.T) {
		kf := Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "mango", Version: "1.2.3", DeGlazeBehavior: LockMajor},
				{Name: "orange", Version: "8.0.8", DeGlazeBehavior: LockMinor},
				{Name: "papaya", Version: "4.2.0", DeGlazeBehavior: LockPatch},
				{Name: "peach", Version: "9.8.7", DeGlazeBehavior: LockNone},
			},
		}
		kl := KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "mango", Version: "1.2.3"},
				{Name: "orange", Version: "8.0.8"},
				{Name: "papaya", Version: "4.2.0"},
				{Name: "peach", Version: "9.8.7"},
			},
		}

		require.NoError(t, kf.DeGlaze(kl))

		assert.Equal(t, Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "mango", Version: "~1", DeGlazeBehavior: LockMajor},
				{Name: "orange", Version: "~8.0", DeGlazeBehavior: LockMinor},
				{Name: "papaya", Version: "4.2.0", DeGlazeBehavior: LockPatch},
				{Name: "peach", Version: "*", DeGlazeBehavior: LockNone},
			},
		}, kf)
	})

	t.Run("when float_always is true", func(t *testing.T) {
		kf := Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "mango", Version: "1.2.3", DeGlazeBehavior: LockMajor, FloatAlways: true},
			},
		}
		kl := KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "mango", Version: "1.2.3"},
			},
		}

		require.NoError(t, kf.DeGlaze(kl))

		assert.Equal(t, Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "mango", Version: "~1", DeGlazeBehavior: LockMajor, FloatAlways: true},
			},
		}, kf, "it should change the constraint")
	})
}

func TestDeGlazeBehavior_TextUnmarshalMarshaller(t *testing.T) {
	for _, tt := range []DeGlazeBehavior{
		LockNone,
		LockPatch,
		LockMinor,
		LockMajor,
	} {
		t.Run(tt.String(), func(t *testing.T) {
			text, err := tt.MarshalText()
			assert.NoError(t, err)
			var out DeGlazeBehavior
			assert.NoError(t, out.UnmarshalText(text))
			assert.Equal(t, tt, out)

			type Structure struct {
				B DeGlazeBehavior `yaml:"b"`
			}

			buf, err := yaml.Marshal(Structure{
				B: tt,
			})
			assert.Contains(t, string(buf), tt.String())
			require.NoError(t, err)
			var fromYAML Structure
			require.NoError(t, yaml.Unmarshal(buf, &fromYAML))
			assert.Equal(t, tt, fromYAML.B)
		})
	}

	t.Run("marshal text unknown", func(t *testing.T) {
		unknown := DeGlazeBehavior(101)
		text, err := unknown.MarshalText()
		assert.Error(t, err)
		assert.Zero(t, text)
	})
	t.Run("unmarshal text unknown", func(t *testing.T) {
		var value DeGlazeBehavior
		err := value.UnmarshalText([]byte("banana"))
		assert.Error(t, err)
		assert.Zero(t, value)
	})

	t.Run("marshal yaml unknown", func(t *testing.T) {
		unknown := DeGlazeBehavior(101)
		_, err := yaml.Marshal(unknown)
		assert.Error(t, err)
	})
	t.Run("unmarshal yaml unknown", func(t *testing.T) {
		var out DeGlazeBehavior
		err := yaml.Unmarshal([]byte("banana"), &out)
		assert.Error(t, err)
	})

	t.Run("wrong yaml type", func(t *testing.T) {
		var out DeGlazeBehavior
		err := yaml.Unmarshal([]byte("{}"), &out)
		assert.Error(t, err)
	})
}

func Test_deGlazeBOSHReleaseTarballSpecification(t *testing.T) {
	for _, tt := range []struct {
		Name           string
		InSpec         BOSHReleaseTarballSpecification
		InLock         BOSHReleaseTarballLock
		Out            BOSHReleaseTarballSpecification
		ErrorSubstring string
	}{
		{
			Name:   "zero value behavior",
			InSpec: BOSHReleaseTarballSpecification{},
			InLock: BOSHReleaseTarballLock{Version: "1.2.3"},
			Out:    BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMajor, Version: "~1"},
		},
		{
			Name:   "LockNone",
			InSpec: BOSHReleaseTarballSpecification{DeGlazeBehavior: LockNone},
			InLock: BOSHReleaseTarballLock{Version: "1.2.3"},
			Out:    BOSHReleaseTarballSpecification{DeGlazeBehavior: LockNone, Version: "*"},
		},
		{
			Name:   "LockPatch",
			InSpec: BOSHReleaseTarballSpecification{DeGlazeBehavior: LockPatch},
			InLock: BOSHReleaseTarballLock{Version: "1.2.3"},
			Out:    BOSHReleaseTarballSpecification{DeGlazeBehavior: LockPatch, Version: "1.2.3"},
		},
		{
			Name:   "LockMinor",
			InSpec: BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMinor},
			InLock: BOSHReleaseTarballLock{Version: "1.2.3"},
			Out:    BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMinor, Version: "~1.2"},
		},
		{
			Name:   "LockMajor",
			InSpec: BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMajor},
			InLock: BOSHReleaseTarballLock{Version: "1.2.3"},
			Out:    BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMajor, Version: "~1"},
		},

		{
			Name:           "not a valid version in Lock",
			InSpec:         BOSHReleaseTarballSpecification{DeGlazeBehavior: LockMajor},
			InLock:         BOSHReleaseTarballLock{Version: "lemon"},
			ErrorSubstring: "Invalid Semantic Version",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			out, err := deGlazeBOSHReleaseTarballSpecification(tt.InSpec, tt.InLock)
			if tt.ErrorSubstring != "" {
				require.ErrorContains(t, err, tt.ErrorSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Out, out)
			}
		})
	}
}
