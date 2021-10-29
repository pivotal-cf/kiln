package commands

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/fs"
	"io/ioutil"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type OptionsParseFunc func([]string, options.StandardOptionsEmbedder) (cargo.Kilnfile, cargo.KilnfileLock, []string, error)

type kilnfileReciever interface {
	KilnExecute(args []string, fn OptionsParseFunc) error
}

type kilnfileLockReturner interface {
	KilnExecute(args []string, fn OptionsParseFunc) (cargo.KilnfileLock, error)
}

type Kiln struct {
	Wrapped interface {
		Usage() jhanda.Usage
	}
	KilnfileStore KilnfileStorer
	StatFn        func(name string) (fs.FileInfo, error)
}

func (cmd Kiln) Execute(args []string) error {
	if cmd.KilnfileStore == nil {
		cmd.KilnfileStore = KilnfileStore{}
	}
	if cmd.StatFn == nil {
		cmd.StatFn = os.Stat
	}

	var kilnfileLockPath string

	parseOps := func(arguments []string, ops options.StandardOptionsEmbedder) (cargo.Kilnfile, cargo.KilnfileLock, []string, error) {
		argsAfterFlags, err := options.FlagsWithDefaults(ops, arguments, cmd.StatFn)
		if err != nil {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, argsAfterFlags, err
		}

		kilnfile, kilnfileLock, err := cmd.KilnfileStore.Load(ops)
		if err != nil {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, argsAfterFlags, fmt.Errorf("error loading Kilnfiles: %w", err)
		}

		kilnfileLockPath = ops.EmbeddedStandardOptions().KilnfileLockPath()

		return kilnfile, kilnfileLock, argsAfterFlags, nil
	}

	switch c := cmd.Wrapped.(type) {
	case kilnfileReciever:
		return c.KilnExecute(args, parseOps)
	case kilnfileLockReturner:
		updatedLock, err := c.KilnExecute(args, parseOps)
		if err != nil {
			return err
		}
		return cmd.KilnfileStore.SaveLock(kilnfileLockPath, updatedLock)
	default:
		return errors.New("command not implemented")
	}
}

func (cmd Kiln) Usage() jhanda.Usage {
	return cmd.Wrapped.Usage()
}

//counterfeiter:generate -o ./fakes/variables_service.go --fake-name VariablesService . VariablesService
type VariablesService interface {
	FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
}

type KilnfileStore struct {
	FS billy.Basic
	VS VariablesService
}

//counterfeiter:generate -o ./fakes/kilnfile_storer.go --fake-name KilnfileStorer . KilnfileStorer

type KilnfileStorer interface {
	Load(flags options.StandardOptionsEmbedder) (cargo.Kilnfile, cargo.KilnfileLock, error)
	SaveLock(p string, l cargo.KilnfileLock) error
}

// Load parses and interpolates the Kilnfile and parsed the Kilnfile.lock.
func (s KilnfileStore) Load(std options.StandardOptionsEmbedder) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	s.FS, s.VS = kilnfileLoadingDefaults(s.FS, s.VS)

	o := std.EmbeddedStandardOptions()

	templateVariables, err := s.loadVariables(o)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	kilnfileFP, err := s.FS.Open(o.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	defer func() {
		_ = kilnfileFP.Close()
	}()

	kilnfile, err := cargo.InterpolateAndParseKilnfile(kilnfileFP, templateVariables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	lockFP, err := s.FS.Open(o.KilnfileLockPath())
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	defer func() {
		_ = lockFP.Close()
	}()
	lockBuf, err := ioutil.ReadAll(lockFP)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	var lock cargo.KilnfileLock
	err = yaml.Unmarshal(lockBuf, &lock)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	return kilnfile, lock, nil
}

func (s KilnfileStore) loadVariables(std options.Standard) (map[string]interface{}, error) {
	templateVariables, err := s.VS.FromPathsAndPairs(std.VariableFiles, std.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template variables: %s", err)
	}
	return templateVariables, nil
}

func (s KilnfileStore) SaveLock(kilnfileLockPath string, l cargo.KilnfileLock) error {
	s.FS, _ = kilnfileLoadingDefaults(s.FS, nil)

	updatedLockFileYAML, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockFile, err := s.FS.Create(kilnfileLockPath) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
	}

	return nil
}

func kilnfileLoadingDefaults(fs billy.Basic, vs VariablesService) (billy.Basic, VariablesService) {
	if fs == nil {
		fs = osfs.New("")
	}
	if vs == nil {
		vs = baking.NewTemplateVariablesService(fs)
	}
	return fs, vs
}
