package carvel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// canonicalEpoch is the fixed timestamp stamped onto every tar/gzip header
// this package rewrites, mirroring the fixed epoch internal/commands/bake.go
// uses for the outer .pivotal zip.
var canonicalEpoch = time.Unix(0, 0).UTC()

type tarEntry struct {
	header *tar.Header
	data   []byte
}

// canonicalizeBoshRelease rewrites a BOSH release tarball produced by
// `bosh create-release` so that identical inputs always produce identical
// bytes. `bosh create-release` itself stamps the real wall-clock time into
// the gzip header of every nested jobs/*.tgz and packages/*.tgz blob (and
// those blobs embed the real filesystem mtime of the files bosh-cli just
// generated), which makes the blob's sha256 - and therefore release.MF and
// the outer tarball - non-deterministic even when the release's content is
// unchanged. This normalizes timestamps/ownership/ordering in the outer
// tarball and every nested blob, then patches the sha256 checksums recorded
// in release.MF to match the re-packed blobs.
//
// This buffers the whole release (every nested blob, decompressed and
// recompressed) in memory rather than streaming, since archive/tar requires
// each entry's final Size up front and release.MF must be written after
// every blob's canonical checksum is known. For carvel tile releases (a
// handful of job/package blobs wrapping one imgpkg bundle) this is a
// non-issue; it would need reconsidering if this were ever reused for
// releases with many large compiled-package blobs.
func canonicalizeBoshRelease(tarballPath string) error {
	info, err := os.Stat(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to read bosh release tarball: %w", err)
	}
	origMode := info.Mode()

	data, err := os.ReadFile(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to read bosh release tarball: %w", err)
	}

	entries, err := readTarGz(data)
	if err != nil {
		return fmt.Errorf("failed to read bosh release tarball: %w", err)
	}

	newSHA := map[string]string{} // "jobs/<name>", "packages/<name>", or "license" -> new sha256 hex

	for name, entry := range entries {
		if name == "release.MF" || !strings.HasSuffix(name, ".tgz") {
			continue
		}

		// Canonicalize every nested blob, not just jobs/*.tgz and
		// packages/*.tgz — a release can also carry a top-level
		// license.tgz (or other blobs future bosh-cli versions add), and
		// those need the same timestamp/ownership normalization to be
		// reproducible.
		canonicalBlob, err := canonicalizeNestedTarGz(entry.data)
		if err != nil {
			return fmt.Errorf("failed to canonicalize %s: %w", name, err)
		}
		entry.data = canonicalBlob
		entries[name] = entry

		sum := sha256.Sum256(canonicalBlob)
		hexSum := hex.EncodeToString(sum[:])

		switch {
		case strings.HasPrefix(name, "jobs/"):
			newSHA["jobs/"+strings.TrimSuffix(strings.TrimPrefix(name, "jobs/"), ".tgz")] = hexSum
		case strings.HasPrefix(name, "packages/"):
			newSHA["packages/"+strings.TrimSuffix(strings.TrimPrefix(name, "packages/"), ".tgz")] = hexSum
		case name == "license.tgz":
			newSHA["license"] = hexSum
		}
	}

	if manifestEntry, ok := entries["release.MF"]; ok {
		patched, err := patchReleaseManifestChecksums(manifestEntry.data, newSHA)
		if err != nil {
			return fmt.Errorf("failed to patch release.MF: %w", err)
		}
		manifestEntry.data = patched
		entries["release.MF"] = manifestEntry
	}

	out, err := writeTarGz(entries)
	if err != nil {
		return fmt.Errorf("failed to write canonical bosh release tarball: %w", err)
	}

	tmp := tarballPath + ".canonical.tmp"
	if err := os.WriteFile(tmp, out, origMode); err != nil {
		return fmt.Errorf("failed to write canonical bosh release tarball: %w", err)
	}
	if err := os.Rename(tmp, tarballPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace bosh release tarball with canonical version: %w", err)
	}
	return nil
}

func canonicalizeNestedTarGz(data []byte) ([]byte, error) {
	entries, err := readTarGz(data)
	if err != nil {
		return nil, err
	}
	return writeTarGz(entries)
}

func readTarGz(data []byte) (map[string]tarEntry, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	entries := map[string]tarEntry{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.Copy(buf, tr); err != nil {
				return nil, err
			}
		}

		h := *hdr
		entries[hdr.Name] = tarEntry{header: &h, data: buf.Bytes()}
	}
	return entries, nil
}

func writeTarGz(entries map[string]tarEntry) ([]byte, error) {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, name := range names {
		e := entries[name]
		hdr := *e.header
		hdr.Name = name
		hdr.ModTime = canonicalEpoch
		hdr.AccessTime = time.Time{}
		hdr.ChangeTime = time.Time{}
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = ""
		hdr.Gname = ""
		hdr.Size = int64(len(e.data))
		// Machine-specific extended attributes (SELinux labels, macOS
		// xattrs, etc.) ride along in PAX records/Xattrs and would
		// otherwise survive re-serialization untouched, reintroducing
		// non-determinism across build machines (e.g. `carvel rebake` run
		// on different CI infrastructure than the original bake).
		hdr.PAXRecords = nil
		hdr.Xattrs = nil //nolint:staticcheck // deprecated field; zero it defensively alongside PAXRecords
		// Let tar.Writer auto-select the minimal format again, rather than
		// carrying over a PAX format decision the original header made
		// solely because of the attributes just stripped above.
		hdr.Format = tar.FormatUnknown

		if err := tw.WriteHeader(&hdr); err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(e.data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	var gzBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&gzBuf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	gw.ModTime = canonicalEpoch
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return gzBuf.Bytes(), nil
}

// patchReleaseManifestChecksums rewrites the sha1 field of each job/package
// entry in release.MF to match the checksums of the re-packed blobs, using a
// yaml.Node round-trip so the rest of the document (formatting, key order,
// untouched fields such as version/fingerprint/commit_hash) is left as-is.
func patchReleaseManifestChecksums(manifest []byte, newSHA map[string]string) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(manifest, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return manifest, nil
	}
	doc := root.Content[0]

	for _, section := range []string{"jobs", "packages"} {
		seq := mappingValue(doc, section)
		if seq == nil {
			continue
		}
		for _, item := range seq.Content {
			nameNode := mappingValue(item, "name")
			if nameNode == nil {
				continue
			}
			newHash, ok := newSHA[section+"/"+nameNode.Value]
			if !ok {
				continue
			}
			if shaNode := mappingValue(item, "sha1"); shaNode != nil {
				shaNode.Value = "sha256:" + newHash
			}
		}
	}

	if newHash, ok := newSHA["license"]; ok {
		if license := mappingValue(doc, "license"); license != nil {
			if shaNode := mappingValue(license, "sha1"); shaNode != nil {
				shaNode.Value = "sha256:" + newHash
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
