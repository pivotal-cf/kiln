package cargo

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"gopkg.in/yaml.v2"
)

func TestComponentLock_yaml_marshal_order(t *testing.T) {
	const validComponentLockYaml = `name: fake-component-name
sha1: fake-component-sha1
version: fake-version
remote_source: fake-source
remote_path: fake/path/to/fake-component-name
`
	damnit := NewWithT(t)

	cl, err := yaml.Marshal(BOSHReleaseLock{
		Name:         "fake-component-name",
		Version:      "fake-version",
		SHA1:         "fake-component-sha1",
		RemoteSource: "fake-source",
		RemotePath:   "fake/path/to/fake-component-name",
	})

	damnit.Expect(err).NotTo(HaveOccurred())
	damnit.Expect(string(cl)).To(Equal(validComponentLockYaml))
}

func TestKilnfileLock_UpdateReleaseLockWithName(t *testing.T) {
	type args struct {
		name string
		lock BOSHReleaseLock
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
				Releases: []BOSHReleaseLock{
					{Name: "banana"},
				},
			},
			KilnfileResult: KilnfileLock{
				Releases: []BOSHReleaseLock{
					{Name: "orange", Version: "some-version"},
				},
			},
			args: args{
				name: "banana", lock: BOSHReleaseLock{Name: "orange", Version: "some-version"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.KilnfileLock.UpdateReleaseLockWithName(tt.args.name, tt.args.lock); tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.KilnfileResult, tt.KilnfileLock)
		})
	}
}
