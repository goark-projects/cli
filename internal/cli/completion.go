package cli

import (
	"fmt"
	"io"
)

const completionCommands = "help version new run build test install vet list fix generate clean tasks task graph sync tools tool doctor codegen info go completion"
const codegenCommands = "configuration registry annotations"

func (c Command) runCompletion(args []string) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		c.printCompletionHelp(c.Out)
		return 0
	}
	if len(args) != 1 {
		c.printCompletionUsageError()
		return 2
	}

	var script string
	switch args[0] {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	case "fish":
		script = fishCompletion
	case "powershell":
		script = powershellCompletion
	default:
		c.printCompletionUsageError()
		return 2
	}
	_, _ = io.WriteString(c.Out, script)
	return 0
}

func (c Command) printCompletionUsageError() {
	_, _ = fmt.Fprintln(c.Err, "completion shell 必须是 bash、zsh、fish、powershell 之一")
	c.printCompletionHelp(c.Err)
}

func (c Command) printCompletionHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark completion <bash|zsh|fish|powershell>

Writes a shell completion script to stdout.

`)
}

const bashCompletion = `_goark() {
  local current
  current="${COMP_WORDS[COMP_CWORD]}"
  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=($(compgen -W "` + completionCommands + `" -- "${current}"))
  elif [[ ${COMP_CWORD} -eq 2 ]]; then
    case "${COMP_WORDS[1]}" in
      completion) COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "${current}")) ;;
      codegen) COMPREPLY=($(compgen -W "` + codegenCommands + `" -- "${current}")) ;;
      new) COMPREPLY=($(compgen -W "app" -- "${current}")) ;;
    esac
  fi
}
complete -F _goark goark
`

const zshCompletion = `#compdef goark
_goark() {
  local -a commands
  commands=(` + completionCommands + `)
  if (( CURRENT == 2 )); then
    _values 'command' $commands
	elif (( CURRENT == 3 )) && [[ ${words[2]} == completion ]]; then
		_values 'shell' bash zsh fish powershell
	elif (( CURRENT == 3 )) && [[ ${words[2]} == codegen ]]; then
		_values 'generator' ` + codegenCommands + `
	elif (( CURRENT == 3 )) && [[ ${words[2]} == new ]]; then
		_values 'scaffold' app
  fi
}
compdef _goark goark
`

const fishCompletion = `complete -c goark -f
complete -c goark -n '__fish_use_subcommand' -a '` + completionCommands + `'
complete -c goark -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'
complete -c goark -n '__fish_seen_subcommand_from codegen' -a '` + codegenCommands + `'
complete -c goark -n '__fish_seen_subcommand_from new' -a 'app'
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName goark -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $elements = $commandAst.CommandElements
  $candidates = if ($elements.Count -eq 2) {
    '` + completionCommands + `'.Split(' ')
	} elseif ($elements.Count -eq 3 -and $elements[1].Value -eq 'completion') {
		'bash zsh fish powershell'.Split(' ')
	} elseif ($elements.Count -eq 3 -and $elements[1].Value -eq 'codegen') {
		'` + codegenCommands + `'.Split(' ')
	} elseif ($elements.Count -eq 3 -and $elements[1].Value -eq 'new') {
		'app'
  } else {
    @()
  }
  $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
