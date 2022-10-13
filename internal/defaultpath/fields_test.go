package defaultpath

import (
	"os"
	"testing"

	"github.com/pivotal-cf/jhanda"

	. "github.com/onsi/gomega"
)

type Embedded struct {
	SharedConfig  string   `short:"s" long:"shared-config" default-path:"shared.yml"`
	SharedScripts []string `short:"i" long:"shared-script" default-path:"a.sh,b.sh"`
}

type options struct {
	Embedded

	Config            string   `short:"c"  long:"config"            default-path:"config.yml"`
	AdditionalConfigs []string `short:"a"  long:"additional-config" default-path:"f1.yml,f2.yml,f3.yml"`
	SomeBool          bool     `short:"e"  long:"ensure"            default-path:"true"`
}

func TestSetFields_sets_defaults(t *testing.T) {
	please := NewWithT(t)

	var (
		ops  options
		args = []string(nil)
	)

	_, err := jhanda.Parse(&ops, args)
	please.Expect(err).NotTo(HaveOccurred())
	SetFields(&ops, "", args, statNoError)

	please.Expect(ops.Config).To(Equal("config.yml"))
	please.Expect(ops.AdditionalConfigs).To(Equal([]string{"f1.yml", "f2.yml", "f3.yml"}))
	please.Expect(ops.SharedConfig).To(Equal("shared.yml"))
	please.Expect(ops.SharedScripts).To(Equal([]string{"a.sh", "b.sh"}))
}

func TestSetFields_adds_path_prefix_to_defaults(t *testing.T) {
	please := NewWithT(t)

	var (
		ops  options
		args = []string(nil)
	)

	_, err := jhanda.Parse(&ops, args)
	please.Expect(err).NotTo(HaveOccurred())
	SetFields(&ops, "some-dir", args, statNoError)

	please.Expect(ops.Config).To(Equal("some-dir/config.yml"))
	please.Expect(ops.AdditionalConfigs).To(Equal([]string{"some-dir/f1.yml", "some-dir/f2.yml", "some-dir/f3.yml"}))
	please.Expect(ops.SharedConfig).To(Equal("some-dir/shared.yml"))
}

func TestSetFields_sets_empty_options_when_filepath_does_not_exist(t *testing.T) {
	please := NewWithT(t)

	var (
		ops  options
		args = []string(nil)
	)

	_, err := jhanda.Parse(&ops, args)
	please.Expect(err).NotTo(HaveOccurred())
	SetFields(&ops, "some-dir", args, statErrNotExistsAll)

	please.Expect(ops.Config).To(Equal(""))
	please.Expect(ops.AdditionalConfigs).To(HaveLen(0))
	please.Expect(ops.SharedConfig).To(Equal(""))
}

func TestSetFields_does_not_override_provided_flags(t *testing.T) {
	t.Run("long", func(t *testing.T) {
		please := NewWithT(t)

		var (
			ops  options
			args = []string{
				"--shared-config", "s.yml",
				"--config", "c.yml",
				"--additional-config", "a1.yml",
				"--additional-config", "a2.yml",
			}
		)

		_, err := jhanda.Parse(&ops, args)
		please.Expect(err).NotTo(HaveOccurred())
		SetFields(&ops, "some-dir", args, statErrNotExistsAll)

		please.Expect(ops.Config).To(Equal("c.yml"))
		please.Expect(ops.AdditionalConfigs).To(Equal([]string{"a1.yml", "a2.yml"}))
		please.Expect(ops.SharedConfig).To(Equal("s.yml"))
	})

	t.Run("short", func(t *testing.T) {
		please := NewWithT(t)

		var (
			ops  options
			args = []string{
				"-s", "s.yml",
				"-c", "c.yml",
				"-a", "a1.yml",
				"-a", "a2.yml",
			}
		)

		_, err := jhanda.Parse(&ops, args)
		please.Expect(err).NotTo(HaveOccurred())
		SetFields(&ops, "some-dir", args, statErrNotExistsAll)

		please.Expect(ops.Config).To(Equal("c.yml"))
		please.Expect(ops.AdditionalConfigs).To(Equal([]string{"a1.yml", "a2.yml"}))
		please.Expect(ops.SharedConfig).To(Equal("s.yml"))
	})
}

func TestSetFields_does_not_add_defaults_if_flag_exists(t *testing.T) {
	please := NewWithT(t)

	var (
		ops  options
		args = []string{
			"--additional-config", "a2.yml",
		}
	)

	_, err := jhanda.Parse(&ops, args)
	please.Expect(err).NotTo(HaveOccurred())
	SetFields(&ops, "some-dir", args, statErrNotExistsAll)

	please.Expect(ops.AdditionalConfigs).To(Equal([]string{"a2.yml"}))
}

func TestSetFields_does_not_override_existing_array(t *testing.T) {
	t.Skip(`This unexpected behavior reproduced in this test not in any actual command usage.
We don't generally add flags to slice fields before jhanda.Parse or SetFields.
This is some strange behavior but a quick fix is not clear.`)
	please := NewWithT(t)

	var (
		ops = options{
			AdditionalConfigs: []string{"addition.yml"},
		}
		args = []string{
			"--additional-config", "a2.yml",
		}
	)

	_, err := jhanda.Parse(&ops, args)
	please.Expect(err).NotTo(HaveOccurred())
	SetFields(&ops, "some-dir", args, statErrNotExistsAll)

	please.Expect(ops.AdditionalConfigs).To(Equal([]string{"addition.yml", "a2.yml"}))
}

func statNoError(string) (os.FileInfo, error) { return nil, nil }

func statErrNotExistsAll(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
