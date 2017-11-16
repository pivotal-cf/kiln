package builder_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataPartsDirectoryReader", func() {
	var (
		filesystem *fakes.Filesystem
		reader     builder.MetadataPartsDirectoryReader
		dirInfo    *fakes.FileInfo
		fileInfo   *fakes.FileInfo
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}

		dirInfo = &fakes.FileInfo{}
		dirInfo.IsDirReturns(true)

		fileInfo = &fakes.FileInfo{}
		fileInfo.IsDirReturns(false)

		filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
			switch root {
			case "/some/variables/path":
				walkFn("/some/variables/path", dirInfo, nil)
				walkFn("/some/variables/path/vars-file-1.yml", fileInfo, nil)
				walkFn("/some/variables/path/vars-file-2.yml", fileInfo, nil)
				return walkFn("/some/variables/path/ignores.any-other-extension", fileInfo, nil)
			default:
				return nil
			}
			return nil
		}
		filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
			switch name {
			case "/some/variables/path/vars-file-1.yml":
				return NewBuffer(bytes.NewBufferString(`---
variables:
- name: variable-1
  type: certificate
- name: variable-2
  type: user
`)), nil
			case "/some/variables/path/vars-file-2.yml":
				return NewBuffer(bytes.NewBufferString(`---
variables:
- name: variable-3
  type: password`)), nil
			default:
				return nil, fmt.Errorf("open %s: no such file or directory", name)
			}
		}
		reader = builder.NewMetadataPartsDirectoryReader(filesystem, "variables")
	})

	Describe("Read", func() {
		It("reads the contents of each yml file in the directory", func() {
			vars, err := reader.Read("/some/variables/path")
			Expect(err).NotTo(HaveOccurred())
			Expect(vars).To(Equal([]interface{}{
				map[interface{}]interface{}{
					"name": "variable-1",
					"type": "certificate",
				},
				map[interface{}]interface{}{
					"name": "variable-2",
					"type": "user",
				},
				map[interface{}]interface{}{
					"name": "variable-3",
					"type": "password",
				},
			}))
		})

		It("references the specified top-level key", func() {
			filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
				switch root {
				case "/some/runtime-configs/path":
					walkFn("/some/runtime-configs/path", dirInfo, nil)
					return walkFn("/some/runtime-configs/path/runtime-config-1.yml", fileInfo, nil)
				default:
					return nil
				}
				return nil
			}
			filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
				switch name {
				case "/some/runtime-configs/path/runtime-config-1.yml":
					return NewBuffer(bytes.NewBufferString(`---
runtime_configs:
- name: runtime-config-1
  runtime_config: the-actual-runtime-config
`)), nil
				default:
					return nil, fmt.Errorf("open %s: no such file or directory", name)
				}
			}
			reader = builder.NewMetadataPartsDirectoryReader(filesystem, "runtime_configs")
			runtimeConfigs, err := reader.Read("/some/runtime-configs/path")
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeConfigs).To(Equal([]interface{}{
				map[interface{}]interface{}{
					"name":           "runtime-config-1",
					"runtime_config": "the-actual-runtime-config",
				},
			}))
		})

		Context("failure cases", func() {
			Context("when there is an error walking the filesystem", func() {
				It("errors", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						err := walkFn("/some/variables/path", dirInfo, errors.New("problem walking filesystem"))
						return err
					}
					_, err := reader.Read("/some/variables/path")
					Expect(err).To(MatchError("problem walking filesystem"))
				})
			})

			Context("when a file cannot be opened", func() {
				It("errors", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						walkFn("/some/variables/path", dirInfo, nil)
						err := walkFn("/some/variables/path/unopenable-file.yml", fileInfo, nil)
						return err
					}
					filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
						return nil, errors.New("cannot open file")
					}
					_, err := reader.Read("/some/variables/path")
					Expect(err).To(MatchError("cannot open file"))
				})
			})

			Context("when there is an error reading from a file", func() {
				It("errors", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						walkFn("/some/variables/path", dirInfo, nil)
						err := walkFn("/some/variables/path/unreadable-file.yml", fileInfo, nil)
						return err
					}

					erroringReader := &fakes.ReadCloser{}
					erroringReader.ReadReturns(0, errors.New("cannot read file"))
					filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
						return erroringReader, nil
					}
					_, err := reader.Read("/some/variables/path")
					Expect(err).To(MatchError("cannot read file"))
					Expect(erroringReader.CloseCallCount()).To(Equal(1))
				})
			})

			Context("when a yaml file is malformed", func() {
				It("errors", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						walkFn("/some/variables/path", dirInfo, nil)
						return walkFn("/some/variables/path/not-well-formed.yml", fileInfo, nil)
					}
					filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
						return NewBuffer(bytes.NewBufferString("not-actually-yaml")), nil
					}
					_, err := reader.Read("/some/variables/path")
					Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
				})
			})

			Context("when a yaml file does not contain the top-level key", func() {
				It("errors", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						switch root {
						case "/some/variables/path":
							walkFn("/some/variables/path", dirInfo, nil)
							return walkFn("/some/variables/path/not-a-vars-file.yml", fileInfo, nil)
						default:
							return nil
						}
						return nil
					}
					filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
						return NewBuffer(bytes.NewBufferString("constants: []")), nil
					}
					reader = builder.NewMetadataPartsDirectoryReader(filesystem, "variables")
					_, err := reader.Read("/some/variables/path")
					Expect(err).To(MatchError(`not a variables file: "/some/variables/path/not-a-vars-file.yml"`))
				})

				It("errors with the correct top-level key", func() {
					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						switch root {
						case "/some/runtime-configs/path":
							walkFn("/some/runtime-configs/path", dirInfo, nil)
							return walkFn("/some/runtime-configs/path/not-a-runtime-configs-file.yml", fileInfo, nil)
						default:
							return nil
						}
						return nil
					}
					filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
						return NewBuffer(bytes.NewBufferString("variables: []")), nil
					}
					reader = builder.NewMetadataPartsDirectoryReader(filesystem, "runtime_configs")
					_, err := reader.Read("/some/runtime-configs/path")
					Expect(err).To(MatchError(`not a runtime_configs file: "/some/runtime-configs/path/not-a-runtime-configs-file.yml"`))
				})
			})
		})
	})
})
