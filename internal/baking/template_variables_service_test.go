package baking_test

import (
	"os"

	"github.com/go-git/go-billy/v5/osfs"

	. "github.com/pivotal-cf/kiln/internal/baking"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TemplateVariablesService", func() {
	Describe("FromPathsAndPairs", func() {
		var (
			service TemplateVariablesService
			path    string
		)

		BeforeEach(func() {
			service = NewTemplateVariablesService(osfs.New(""))

			contents := `---
key-1:
  key-2:
  - value-1
  - value-2
key-3: value-3
`

			file, err := os.CreateTemp("", "variables")
			Expect(err).NotTo(HaveOccurred())

			path = file.Name()

			_, err = file.WriteString(contents)
			Expect(err).NotTo(HaveOccurred())

			err = file.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Remove(path)
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses template variables from a collection of files", func() {
			variables, err := service.FromPathsAndPairs([]string{path}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(variables).To(Equal(map[string]any{
				"key-1": map[any]any{
					"key-2": []any{
						"value-1",
						"value-2",
					},
				},
				"key-3": "value-3",
			}))
		})

		It("parses template variables from command-line arguments", func() {
			variables, err := service.FromPathsAndPairs(nil, []string{
				"key-1=value-1",
				"key-2=value-2",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(variables).To(Equal(map[string]any{
				"key-1": "value-1",
				"key-2": "value-2",
			}))
		})

		Context("failure cases", func() {
			Context("when the variable file cannot be read", func() {
				It("returns an error", func() {
					_, err := service.FromPathsAndPairs([]string{"missing.yml"}, nil)
					Expect(err).To(MatchError(ContainSubstring("open missing.yml: no such file or directory")))
				})
			})

			Context("when the variable file contents cannot be unmarshalled", func() {
				It("returns an error", func() {
					err := os.WriteFile(path, []byte("\t\t\t"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					_, err = service.FromPathsAndPairs([]string{path}, nil)
					Expect(err).To(MatchError(ContainSubstring("yaml: found character that cannot start any token")))
				})
			})

			Context("when the command-line variables are malformed", func() {
				It("returns an error", func() {
					_, err := service.FromPathsAndPairs(nil, []string{"garbage"})
					Expect(err).To(MatchError("could not parse variable \"garbage\": expected variable in \"key=value\" form"))
				})
			})
		})
	})
})
