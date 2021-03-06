// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Taken from https://github.com/linkerd/linkerd2/blob/main/cli/cmd/completion.go

func NewCmdCompletion() *cobra.Command {
	example := `  # bash <= 3.2
  source /dev/stdin <<< "$(kapp completion bash)"

  # bash >= 4.0
  source <(kapp completion bash)

  # bash <= 3.2 on osx
  brew install bash-completion # ensure you have bash-completion 1.3+
  kapp completion bash > $(brew --prefix)/etc/bash_completion.d/kapp

  # bash >= 4.0 on osx
  brew install bash-completion@2
  kapp completion bash > $(brew --prefix)/etc/bash_completion.d/kapp

  # zsh
  source <(kapp completion zsh)

  # zsh on osx / oh-my-zsh
  kapp completion zsh > "${fpath[1]}/_kapp"`

	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish]",
		Short:     "Output shell completion code for the specified shell (bash, zsh or fish)",
		Long:      "Output shell completion code for the specified shell (bash, zsh or fish).",
		Example:   example,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := getCompletion(args[0], cmd.Parent())
			if err != nil {
				return err
			}

			fmt.Print(out)
			return nil
		},
	}

	return cmd
}

// getCompletion will return the auto completion shell script, if supported
func getCompletion(sh string, parent *cobra.Command) (string, error) {
	var err error
	var buf bytes.Buffer

	switch sh {
	case "bash":
		err = parent.GenBashCompletion(&buf)
	case "zsh":
		err = parent.GenZshCompletion(&buf)
	// TODO requires update of cobra
	// case "fish":
	// 	err = parent.GenFishCompletion(&buf, true)

	default:
		err = errors.New("unsupported shell type (must be bash, zsh or fish): " + sh)
	}

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
