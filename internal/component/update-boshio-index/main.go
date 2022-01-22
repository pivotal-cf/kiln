package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/component"
)

func main() {
	defer func() {
		r := recover()
		if r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "FATAL ERROR: %s\n", r)
			os.Exit(1)
		}
	}()

	var cachedIndex []component.BoshReleaseRepositoryRecord

	var out io.Writer = os.Stdout
	if len(os.Args) > 1 {
		if contents, err := os.ReadFile(os.Args[1]); err == nil {
			_ = yaml.Unmarshal(contents, &cachedIndex)
		}

		f, err := os.Create(os.Args[1])
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = f.Close()
		}()
		out = f
	}

	var indexRecords []component.BoshReleaseRepositoryRecord
	err := getAndParse("https://raw.githubusercontent.com/bosh-io/releases/HEAD/index.yml", &indexRecords)
	if err != nil {
		panic(err)
	}

	hydrateCache(cachedIndex, indexRecords)

	for i, record := range indexRecords {
		if record.URL == "" {
			continue
		}
		if record.Name != "" {
			_, _ = fmt.Fprintf(os.Stderr, "using cached release name %q for %s\n", record.Name, record.URL)
			continue
		}
		_, _ = fmt.Fprintf(os.Stderr, "getting release name for %s\n", record.URL)
		u, err := url.Parse(record.URL)
		if err != nil {
			panic(fmt.Errorf("failed to parse bosh release record url: %w", err))
		}
		var configFinal struct {
			FinalName string `yaml:"final_name"`
			Name      string `yaml:"name"`
		}
		err = getAndParse("https://"+path.Join("raw.githubusercontent.com", u.Path, "HEAD/config/final.yml"), &configFinal)
		if err != nil {
			panic(err)
		}
		if configFinal.FinalName != "" {
			indexRecords[i].Name = configFinal.FinalName
		} else {
			indexRecords[i].Name = configFinal.Name
		}
	}

	checkAndLogDuplicateNames(indexRecords)

	result, err := yaml.Marshal(indexRecords)
	if err != nil {
		panic(err)
	}

	_, err = out.Write(result)
	if err != nil {
		panic(fmt.Errorf("failed to write to output: %w", err))
	}
}

func getAndParse(uri string, data interface{}) error {
	res, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("GET %s failed: %w", uri, err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return fmt.Errorf("GET %s failed: expected status OK (200) got %s (%d)",
			uri,
			http.StatusText(res.StatusCode), res.StatusCode)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	err = yaml.NewDecoder(res.Body).Decode(data)
	if err != nil {
		return fmt.Errorf("failed to parse %s response: %w", uri, err)
	}
	return nil
}

func hydrateCache(previous, updated component.BoshReleaseRepositoryIndex) {
	for i, ur := range updated {
		for _, pr := range previous {
			if ur.URL == pr.URL {
				updated[i].Name = pr.Name
				break
			}
		}
	}
}

func checkAndLogDuplicateNames(index component.BoshReleaseRepositoryIndex) {
	names := make(map[string]string)
	for _, r := range index {
		if r.Name == "" {
			continue
		}
		previousURL, exists := names[r.Name]
		if exists {
			_, _ = fmt.Fprintf(os.Stderr, "%s and %s have the same release name\n", previousURL, r.URL)
			continue
		}
		names[r.Name] = r.URL
	}
}
