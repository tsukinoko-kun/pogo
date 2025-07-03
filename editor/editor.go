package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
)

func String(title, value string) (string, error) {
	editor := getEditor()
	if len(editor) == 0 {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewText().ShowLineNumbers(true).Title(title).Value(&value),
		)).Run(); err != nil {
			if err == huh.ErrUserAborted {
				return value, err
			}
			return value, errors.Join(errors.New("opening builtin editor"), err)
		}
		return strings.TrimSpace(value), nil
	} else {
		f, err := os.CreateTemp("", "edit-*")
		if err != nil {
			return value, errors.Join(errors.New("creating temp file"), err)
		}
		fmt.Fprintln(f, value)
		_ = f.Close()
		defer os.Remove(f.Name())
		args := strings.Fields(editor)
		cmd := exec.Command(args[0], append(args[1:], f.Name())...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return value, errors.Join(errors.New("running editor"), err)
		}
		b, err := os.ReadFile(f.Name())
		if err != nil {
			return value, errors.Join(errors.New("reading temp file"), err)
		}
		return strings.TrimSpace(string(b)), nil
	}
}

func File(path string) error {
	editor := getEditor()
	if len(editor) == 0 {
		return errors.New("no editor found")
	}
	args := strings.Fields(editor)
	cmd := exec.Command(args[0], append(args[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Join(errors.New("running editor"), err)
	}
	return nil
}

func getEditor() string {
	if editorEnv, ok := os.LookupEnv("EDITOR"); ok {
		return editorEnv
	}
	if neovim, err := exec.LookPath("nvim"); err == nil {
		return neovim
	}
	if vim, err := exec.LookPath("vim"); err == nil {
		return vim
	}
	if vi, err := exec.LookPath("vi"); err == nil {
		return vi
	}
	if nano, err := exec.LookPath("nano"); err == nil {
		return nano
	}
	if edit, err := exec.LookPath("edit"); err == nil {
		return edit
	}
	return ""
}
