package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

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

	indexRecords, errList := component.GetBoshReleaseRepositoryIndex(context.Background())
	for _, err := range errList {
		log.Println(err)
	}
	checkAndLogDuplicateNames(indexRecords)

	result, err := yaml.Marshal(indexRecords)
	if err != nil {
		panic(err)
	}

	var out io.Writer = os.Stdout
	if len(os.Args) > 1 {
		f, err := os.Create(os.Args[1])
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = f.Close()
		}()
		out = f
	}

	_, err = out.Write(result)
	if err != nil {
		panic(fmt.Errorf("failed to write to output: %w", err))
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
