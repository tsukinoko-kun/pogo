package client

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

var ignoreFileNames = []string{".pogoignore", ".gitignore"}

func getLocalIgnoreFiles(rootDir string) func(yield func(absPath string) bool) {
	return func(yield func(absPath string) bool) {
		_ = filepath.WalkDir(rootDir, func(absPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			fileName := filepath.Base(absPath)
			if slices.Contains(ignoreFileNames, fileName) {
				if !yield(absPath) {
					return filepath.SkipAll
				}
			}
			return nil
		})
	}
}

func getLocalIgnoreMatcherFiltered(rootDir string, include func(relUnixPath string) bool) gitignore.Matcher {
	var patterns []gitignore.Pattern
	patterns = append(patterns, gitignore.ParsePattern(".pogo", nil))
	patterns = append(patterns, gitignore.ParsePattern(".DS_Store", nil))
	patterns = append(patterns, gitignore.ParsePattern(".git/", nil))

	for absPath := range getLocalIgnoreFiles(rootDir) {
		relPath, err := filepath.Rel(rootDir, absPath)
		if err != nil {
			continue
		}
		if !include(filepath.ToSlash(relPath)) {
			continue
		}
		relDir := filepath.Dir(relPath)
		var domain []string
		if relDir != "." {
			domain = strings.Split(relDir, string(os.PathSeparator))
		}
		ignoreFile, err := os.Open(absPath)
		if err != nil {
			continue
		}
		defer ignoreFile.Close()
		scanner := bufio.NewScanner(ignoreFile)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			patterns = append(patterns, gitignore.ParsePattern(line, domain))
		}
	}
	return gitignore.NewMatcher(patterns)
}

func getLocalIgnoreMatcher(rootDir string) gitignore.Matcher {
	var patterns []gitignore.Pattern
	patterns = append(patterns, gitignore.ParsePattern(".pogo", nil))
	patterns = append(patterns, gitignore.ParsePattern(".DS_Store", nil))
	patterns = append(patterns, gitignore.ParsePattern(".git/", nil))

	for absPath := range getLocalIgnoreFiles(rootDir) {
		relPath, err := filepath.Rel(rootDir, absPath)
		if err != nil {
			continue
		}
		relDir := filepath.Dir(relPath)
		var domain []string
		if relDir != "." {
			domain = strings.Split(relDir, string(os.PathSeparator))
		}
		ignoreFile, err := os.Open(absPath)
		if err != nil {
			continue
		}
		defer ignoreFile.Close()
		scanner := bufio.NewScanner(ignoreFile)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			patterns = append(patterns, gitignore.ParsePattern(line, domain))
		}
	}
	return gitignore.NewMatcher(patterns)
}

func getLocalFiles(rootDir string) func(yield func(absPath, name string) bool) {
	return func(yield func(absPath, name string) bool) {
		matcher := getLocalIgnoreMatcher(rootDir)
		_ = filepath.WalkDir(rootDir, func(absPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			isDir := d.IsDir()
			relPath, err := filepath.Rel(rootDir, absPath)
			if err != nil {
				return err
			}
			gitPath := strings.Split(relPath, string(os.PathSeparator))
			if len(gitPath) == 0 {
				return nil
			}
			if gitPath[0] == "." {
				gitPath = gitPath[1:]
			}
			if matcher.Match(gitPath, isDir) {
				if isDir {
					return filepath.SkipDir
				}
				return nil
			}
			if isDir {
				return nil
			}
			if !yield(absPath, filepath.ToSlash(relPath)) {
				return filepath.SkipAll
			}
			return nil
		})
	}
}
