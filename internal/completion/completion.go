package completion

import (
	"fmt"
	"io"
)

const completionCommands = "help version new run build test install vet list fix generate clean tasks task graph sync tools tool doctor codegen info go completion"
const codegenCommands = "configuration registry annotations"

// Run 校验目标 shell 并将补全脚本写入输出流。
func Run(args []string, out io.Writer, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		Help(out)
		return 0
	}
	if len(args) != 1 {
		printUsageError(errOut)
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
		printUsageError(errOut)
		return 2
	}
	_, _ = io.WriteString(out, script)
	return 0
}

func printUsageError(errOut io.Writer) {
	_, _ = fmt.Fprintln(errOut, "completion shell 必须是 bash、zsh、fish、powershell 之一")
	Help(errOut)
}

// Help 输出 completion 命令帮助。
func Help(w io.Writer) {
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
      new) COMPREPLY=($(compgen -W "-type -module -dir -force" -- "${current}")) ;;
    esac
	elif [[ ${COMP_WORDS[1]} == new ]]; then
		case "${COMP_WORDS[COMP_CWORD-1]}" in
			-type|--type) COMPREPLY=($(compgen -W "app web" -- "${current}")) ;;
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
	elif [[ ${words[2]} == new ]]; then
		if [[ ${words[CURRENT-1]} == -type || ${words[CURRENT-1]} == --type ]]; then
			_values 'type' app web
		else
			_values 'option' -type -module -dir -force
		fi
	fi
}
compdef _goark goark
`

const fishCompletion = `complete -c goark -f
complete -c goark -n '__fish_use_subcommand' -a '` + completionCommands + `'
complete -c goark -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'
complete -c goark -n '__fish_seen_subcommand_from codegen' -a '` + codegenCommands + `'
complete -c goark -n '__fish_seen_subcommand_from new; and not __fish_prev_arg_in -type --type' -a '-type -module -dir -force'
complete -c goark -n '__fish_seen_subcommand_from new; and __fish_prev_arg_in -type --type' -a 'app web'
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
	} elseif ($elements.Count -ge 3 -and $elements[1].Value -eq 'new') {
		if ($elements[$elements.Count - 2].Value -in @('-type', '--type')) {
			'app web'.Split(' ')
		} else {
			'-type -module -dir -force'.Split(' ')
		}
  } else {
    @()
  }
  $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
