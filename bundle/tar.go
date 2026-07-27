package bundle

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxTarEntries    = 4096
	maxTarTotalBytes = 32 << 20
)

func safeEntryName(name string) bool {
	if name == "" || path.IsAbs(name) || filepath.IsAbs(name) {
		return false
	}
	clean := path.Clean(name)
	if clean == ".." {
		return false
	}
	if clean == "." {
		return true
	}
	for _, el := range strings.Split(clean, "/") {
		if el == ".." {
			return false
		}
	}
	return true
}

func ValidateAndBrief(tarBytes []byte) (string, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	var brief string
	var count int
	var total int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if !safeEntryName(h.Name) {
			return "", fmt.Errorf("bundle: unsafe tar entry: %q", h.Name)
		}
		count++
		if count > maxTarEntries {
			return "", fmt.Errorf("bundle: too many tar entries (max %d)", maxTarEntries)
		}
		total += h.Size
		if total > maxTarTotalBytes {
			return "", fmt.Errorf("bundle: tar too large (max %d bytes)", maxTarTotalBytes)
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == "SKILL.md" {
			body, err := io.ReadAll(tr)
			if err != nil {
				return "", err
			}
			brief = frontmatterDescription(body)
		}
	}
	return brief, nil
}

func frontmatterDescription(md []byte) string {
	s := strings.TrimLeft(string(md), "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return ""
	}
	rest := s[3:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return ""
	}
	rest = rest[nl+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	lines := strings.Split(rest[:end], "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "description:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(t, "description:"))
		switch val {
		case ">", "|", ">-", "|-":
			var parts []string
			for _, sub := range lines[i+1:] {
				if strings.TrimSpace(sub) == "" {
					continue
				}
				if sub[0] != ' ' && sub[0] != '\t' {
					break
				}
				parts = append(parts, strings.TrimSpace(sub))
			}
			return strings.Join(parts, " ")
		}
		return strings.Trim(val, `"'`)
	}
	return ""
}
