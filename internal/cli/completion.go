// SPDX-License-Identifier: MIT
package cli

import (
	"flag"
	"fmt"
)

var bashCompletion = `_dxrk_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=($(compgen -W "chat check-permission help install mcp query restore sync uninstall update upgrade version" -- "$cur"))
    if [[ ${#COMP_WORDS[@]} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "chat check-permission help install mcp query restore sync uninstall update upgrade version" -- "$cur"))
    fi
    case "${COMP_WORDS[1]}" in
        mcp)
            if [[ ${#COMP_WORDS[@]} -eq 3 ]]; then
                COMPREPLY=($(compgen -W "discover generate-config serve" -- "$cur"))
            fi
            ;;
    esac
}
complete -F _dxrk_completions dxrk
`

var zshCompletion = `#compdef dxrk
_dxrk() {
    local -a commands
    commands=(
        'chat:Start a chat session'
        'check-permission:Check file permissions'
        'help:Show help'
        'install:Install components'
        'mcp:MCP server commands'
        'mcp-discover:Discover MCP servers'
        'mcp-generate-config:Generate MCP config'
        'mcp-serve:Serve MCP over stdio'
        'query:Run a query'
        'restore:Restore from backup'
        'sync:Sync configuration'
        'uninstall:Uninstall components'
        'update:Check for updates'
        'upgrade:Upgrade components'
        'version:Show version'
    )
    _describe 'command' commands
}
compdef _dxrk dxrk
`

var fishCompletion = `set -g fish_complete_path
complete -c dxrk -f -a "chat" -d "Start a chat session"
complete -c dxrk -f -a "check-permission" -d "Check file permissions"
complete -c dxrk -f -a "help" -d "Show help"
complete -c dxrk -f -a "install" -d "Install components"
complete -c dxrk -f -a "mcp" -d "MCP server commands"
complete -c dxrk -f -a "query" -d "Run a query"
complete -c dxrk -f -a "restore" -d "Restore from backup"
complete -c dxrk -f -a "sync" -d "Sync configuration"
complete -c dxrk -f -a "uninstall" -d "Uninstall components"
complete -c dxrk -f -a "update" -d "Check for updates"
complete -c dxrk -f -a "upgrade" -d "Upgrade components"
complete -c dxrk -f -a "version" -d "Show version"
complete -c dxrk -f -n "__fish_seen_subcommand_from mcp" -a "discover generate-config serve"
`

func RunCompletion(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletion, nil
	case "zsh":
		return zshCompletion, nil
	case "fish":
		return fishCompletion, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", shell)
	}
}

func ParseCompletionFlags(args []string) (string, error) {
	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	shell := fs.String("shell", "", "Shell type (bash, zsh, fish)")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if *shell != "" {
		return *shell, nil
	}
	// Fall back to first positional arg.
	if fs.NArg() > 0 {
		return fs.Arg(0), nil
	}
	return "", fmt.Errorf("--shell is required (bash, zsh, fish)")
}
