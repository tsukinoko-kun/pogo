package main

import (
	"errors"
	"fmt"
	"github.com/tsukinoko-kun/pogo/cmd"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/cobra/doc"
)

const (
	checkoutPath = "../pogo.wiki"
	repoUrl      = "https://github.com/tsukinoko-kun/pogo.wiki.git"
)

func main() {
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	email := os.Getenv("EMAIL")
	auth := &http.BasicAuth{Username: username, Password: password}

	fmt.Println("Cloning wiki repo")
	r, err := git.PlainClone(checkoutPath, false, &git.CloneOptions{
		URL:   repoUrl,
		Auth:  auth,
		Depth: 1,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Deleting old files")
	if oldFiles, err := filepath.Glob(filepath.Join(checkoutPath, cmd.RootCmd.Use+"*.md")); err == nil {
		for _, f := range oldFiles {
			_ = os.Remove(f)
		}
	}

	fmt.Println("Generating new wiki files")
	if err = doc.GenMarkdownTreeCustom(
		cmd.RootCmd,
		checkoutPath,
		func(s string) string { return s },
		func(s string) string {
			return strings.TrimSuffix(s, ".md")
		},
	); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Opening worktree")
	w, err := r.Worktree()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Adding changes")
	if err := w.AddGlob("*"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Committing changes")
	if _, err := w.Commit("auto gen wiki", &git.CommitOptions{
		Author: &object.Signature{
			Name:  username,
			Email: email,
			When:  time.Now(),
		},
	}); err != nil {
		if errors.Is(err, git.ErrEmptyCommit) {
			fmt.Println("No changes to commit")
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Pushing commit")
	if err := r.Push(&git.PushOptions{
		Auth: auth,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Done")
}
