# github.com/spf13/cobra (v1.10.2)

## README.md

<div align="center">
<a href="https://cobra.dev">
<img width="512" height="535" alt="cobra-logo" src="https://github.com/user-attachments/assets/c8bf9aad-b5ae-41d3-8899-d83baec10af8" />
</a>
</div>

Cobra is a library for creating powerful modern CLI applications.

<a href="https://cobra.dev">Visit Cobra.dev for extensive documentation</a> 


Cobra is used in many Go projects such as [Kubernetes](https://kubernetes.io/),
[Hugo](https://gohugo.io), and [GitHub CLI](https://github.com/cli/cli) to
name a few. [This list](site/content/projects_using_cobra.md) contains a more extensive list of projects using Cobra.

[![](https://img.shields.io/github/actions/workflow/status/spf13/cobra/test.yml?branch=main&longCache=true&label=Test&logo=github%20actions&logoColor=fff)](https://github.com/spf13/cobra/actions?query=workflow%3ATest)
[![Go Reference](https://pkg.go.dev/badge/github.com/spf13/cobra.svg)](https://pkg.go.dev/github.com/spf13/cobra)
[![Go Report Card](https://goreportcard.com/badge/github.com/spf13/cobra)](https://goreportcard.com/report/github.com/spf13/cobra)
[![Slack](https://img.shields.io/badge/Slack-cobra-brightgreen)](https://gophers.slack.com/archives/CD3LP1199)
<hr>
<div align="center" markdown="1">
   <sup>Supported by:</sup>
   <br>
   <br>
   <a href="https://www.warp.dev/cobra">
      <img alt="Warp sponsorship" width="400" src="https://github.com/user-attachments/assets/ab8dd143-b0fd-4904-bdc5-dd7ecac94eae">
   </a>

### [Warp, the AI terminal for devs](https://www.warp.dev/cobra)
[Try Cobra in Warp today](https://www.warp.dev/cobra)<br>

</div>
<hr>

# Overview

Cobra is a library providing a simple interface to create powerful modern CLI

Cobra provides:
* Easy subcommand-based CLIs: `app server`, `app fetch`, etc.
* Fully POSIX-compliant flags (including short & long versions)
* Nested subcommands
* Global, local and cascading flags
* Intelligent suggestions (`app srver`... did you mean `app server`?)
* Automatic help generation for commands and flags
* Grouping help for subcommands
* Automatic help flag recognition of `-h`, `--help`, etc.
* Automatically generated shell autocomplete for your application (bash, zsh, fish, powershell)
* Automatically generated man pages for your application
* Command aliases so you can change things without breaking them
* The flexibility to define your own help, usage, etc.
* Optional seamless integration with [viper](https://github.com/spf13/viper) for 12-factor apps

# Concepts

Cobra is built on a structure of commands, arguments & flags.

**Commands** represent actions, **Args** are things and **Flags** are modifiers for those actions.

The best applications read like sentences when used, and as a result, users
intuitively know how to interact with them.

The pattern to follow is
`APPNAME VERB NOUN --ADJECTIVE`
    or
`APPNAME COMMAND ARG --FLAG`.

A few good real world examples may better illustrate this point.

In the following example, 'server' is a command, and 'port' is a flag:

    hugo server --port=1313

In this command we are telling Git to clone the url bare.

    git clone URL --bare

## Commands

Command is the central point of the application. Each interaction that
the application supports will be contained in a Command. A command can
have children commands and optionally run an action.

In the example above, 'server' is the command.

[More about cobra.Command](https://pkg.go.dev/github.com/spf13/cobra#Command)

## Flags

A flag is a way to modify the behavior of a command. Cobra supports
fully POSIX-compliant flags as well as the Go [flag package](https://golang.org/pkg/flag/).
A Cobra command can define flags that persist through to children commands
and flags that are only available to that command.

In the example above, 'port' is the flag.

Flag functionality is provided by the [pflag
library](https://github.com/spf13/pflag), a fork of the flag standard library
which maintains the same interface while adding POSIX compliance.

# Installing
Using Cobra is easy. First, use `go get` to install the latest version
of the library.

```
go get -u github.com/spf13/cobra@latest
```

Next, include Cobra in your application:

```go
import "github.com/spf13/cobra"
```

# Usage
`cobra-cli` is a command line program to generate cobra applications and command files.
It will bootstrap your application scaffolding to rapidly
develop a Cobra-based application. It is the easiest way to incorporate Cobra into your application.

It can be installed by running:

```
go install github.com/spf13/cobra-cli@latest
```

For complete details on using the Cobra-CLI generator, please read [The Cobra Generator README](https://github.com/spf13/cobra-cli/blob/main/README.md)

For complete details on using the Cobra library, please read [The Cobra User Guide](site/content/user_guide.md).

# License

Cobra is released under the Apache 2.0 license. See [LICENSE.txt](LICENSE.txt)

## active_help.md

# Active Help

Active Help is a framework provided by Cobra which allows a program to define messages (hints, warnings, etc) that will be printed during program usage.  It aims to make it easier for your users to learn how to use your program.  If configured by the program, Active Help is printed when the user triggers shell completion.

For example,

```console
$ helm repo add [tab]
You must choose a name for the repo you are adding.

$ bin/helm package [tab]
Please specify the path to the chart to package

$ bin/helm package [tab][tab]
bin/    internal/    scripts/    pkg/     testdata/
```

**Hint**: A good place to use Active Help messages is when the normal completion system does not provide any suggestions. In such cases, Active Help nicely supplements the normal shell completions to guide the user in knowing what is expected by the program.

## Supported shells

Active Help is currently only supported for the following shells:
- Bash (using [bash completion V2](completions/_index.md#bash-completion-v2) only). Note that bash 4.4 or higher is required for the prompt to appear when an Active Help message is printed.
- Zsh

## Adding Active Help messages

As Active Help uses the shell completion system, the implementation of Active Help messages is done by enhancing custom dynamic completions.  If you are not familiar with dynamic completions, please refer to [Shell Completions](completions/_index.md).

Adding Active Help is done through the use of the `cobra.AppendActiveHelp(...)` function, where the program repeatedly adds Active Help messages to the list of completions.  Keep reading for details.

### Active Help for nouns

Adding Active Help when completing a noun is done within the `ValidArgsFunction(...)` of a command.  Please notice the use of `cobra.AppendActiveHelp(...)` in the following example:

```go
cmd := &cobra.Command{
	Use:   "add [NAME] [URL]",
	Short: "add a chart repository",
	Args:  require.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addRepo(args)
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		var comps []cobra.Completion
		if len(args) == 0 {
			comps = cobra.AppendActiveHelp(comps, "You must choose a name for the repo you are adding")
		} else if len(args) == 1 {
			comps = cobra.AppendActiveHelp(comps, "You must specify the URL for the repo you are adding")
		} else {
			comps = cobra.AppendActiveHelp(comps, "This command does not take any more arguments")
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	},
}
```

The example above defines the completions (none, in this specific example) as well as the Active Help messages for the `helm repo add` command.  It yields the following behavior:

```console
$ helm repo add [tab]
You must choose a name for the repo you are adding

$ helm repo add grafana [tab]
You must specify the URL for the repo you are adding

$ helm repo add grafana https://grafana.github.io/helm-charts [tab]
This command does not take any more arguments
```

**Hint**: As can be seen in the above example, a good place to use Active Help messages is when the normal completion system does not provide any suggestions. In such cases, Active Help nicely supplements the normal shell completions.

### Active Help for flags

Providing Active Help for flags is done in the same fashion as for nouns, but using the completion function registered for the flag.  For example:

```go
_ = cmd.RegisterFlagCompletionFunc("version", func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) != 2 {
			return cobra.AppendActiveHelp(nil, "You must first specify the chart to install before the --version flag can be completed"), cobra.ShellCompDirectiveNoFileComp
		}
		return compVersionFlag(args[1], toComplete)
	})
```
The example above prints an Active Help message when not enough information was given by the user to complete the `--version` flag.

```console
$ bin/helm install myrelease --version 2.0.[tab]
You must first specify the chart to install before the --version flag can be completed

$ bin/helm install myrelease bitnami/solr --version 2.0.[tab][tab]
2.0.1  2.0.2  2.0.3
```

## User control of Active Help

You may want to allow your users to disable Active Help or choose between different levels of Active Help.  It is entirely up to the program to define the type of configurability of Active Help that it wants to offer, if any.
Allowing to configure Active Help is entirely optional; you can use Active Help in your program without doing anything about Active Help configuration.

The way to configure Active Help is to use the program's Active Help environment
variable.  That variable is named `<PROGRAM>_ACTIVE_HELP` where `<PROGRAM>` is the name of your 
program in uppercase with any non-ASCII-alphanumeric characters replaced by an `_`.  The variable should be set by the user to whatever
Active Help configuration values are supported by the program.

For example, say `helm` has chosen to support three levels for Active Help: `on`, `off`, `local`.  Then a user
would set the desired behavior to `local` by doing `export HELM_ACTIVE_HELP=local` in their shell.

For simplicity, when in `cmd.ValidArgsFunction(...)` or a flag's completion function, the program should read the
Active Help configuration using the `cobra.GetActiveHelpConfig(cmd)` function and select what Active Help messages
should or should not be added (instead of reading the environment variable directly).

For example:

```go
ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	activeHelpLevel := cobra.GetActiveHelpConfig(cmd)

	var comps []cobra.Completion
	if len(args) == 0 {
		if activeHelpLevel != "off"  {
			comps = cobra.AppendActiveHelp(comps, "You must choose a name for the repo you are adding")
		}
	} else if len(args) == 1 {
		if activeHelpLevel != "off" {
			comps = cobra.AppendActiveHelp(comps, "You must specify the URL for the repo you are adding")
		}
	} else {
		if activeHelpLevel == "local" {
			comps = cobra.AppendActiveHelp(comps, "This command does not take any more arguments")
		}
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
},
```

**Note 1**: If the `<PROGRAM>_ACTIVE_HELP` environment variable is set to the string "0", Cobra will automatically disable all Active Help output (even if some output was specified by the program using the `cobra.AppendActiveHelp(...)` function).  Using "0" can simplify your code in situations where you want to blindly disable Active Help without having to call `cobra.GetActiveHelpConfig(cmd)` explicitly.

**Note 2**: If a user wants to disable Active Help for every single program based on Cobra, she can set the environment variable `COBRA_ACTIVE_HELP` to "0".  In this case `cobra.GetActiveHelpConfig(cmd)` will return "0" no matter what the variable `<PROGRAM>_ACTIVE_HELP` is set to.

**Note 3**: If the user does not set `<PROGRAM>_ACTIVE_HELP` or `COBRA_ACTIVE_HELP` (which will be a common case), the default value for the Active Help configuration returned by `cobra.GetActiveHelpConfig(cmd)` will be the empty string. 

## Active Help with Cobra's default completion command

Cobra provides a default `completion` command for programs that wish to use it.
When using the default `completion` command, Active Help is configurable in the same
fashion as described above using environment variables.  You may wish to document this in more
details for your users.

## Debugging Active Help

Debugging your Active Help code is done in the same way as debugging your dynamic completion code, which is with Cobra's hidden `__complete` command.  Please refer to [debugging shell completion](completions/_index.md#debugging) for details.

When debugging with the `__complete` command, if you want to specify different Active Help configurations, you should use the active help environment variable.  That variable is named `<PROGRAM>_ACTIVE_HELP` where any non-ASCII-alphanumeric characters are replaced by an `_`.  For example, we can test deactivating some Active Help as shown below:

```console
$ HELM_ACTIVE_HELP=1 bin/helm __complete install wordpress bitnami/h<ENTER>
bitnami/haproxy
bitnami/harbor
_activeHelp_ WARNING: cannot re-use a name that is still in use
:0
Completion ended with directive: ShellCompDirectiveDefault

$ HELM_ACTIVE_HELP=0 bin/helm __complete install wordpress bitnami/h<ENTER>
bitnami/haproxy
bitnami/harbor
:0
Completion ended with directive: ShellCompDirectiveDefault
```

## _index.md

# Generating shell completions

Cobra can generate shell completions for multiple shells.
The currently supported shells are:
- Bash
- Zsh
- fish
- PowerShell

Cobra will automatically provide your program with a fully functional `completion` command,
similarly to how it provides the `help` command. If there are no other subcommands, the
default `completion` command will be hidden, but still functional.

## Creating your own completion command

If you do not wish to use the default `completion` command, you can choose to
provide your own, which will take precedence over the default one. (This also provides
backwards-compatibility with programs that already have their own `completion` command.)

If you are using the `cobra-cli` generator,
which can be found at [spf13/cobra-cli](https://github.com/spf13/cobra-cli),
you can create a completion command by running

```bash
cobra-cli add completion
```
and then modifying the generated `cmd/completion.go` file to look something like this
(writing the shell script to stdout allows the most flexible use):

```go
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: fmt.Sprintf(`To load completions:

Bash:

  $ source <(%[1]s completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ %[1]s completion bash > /etc/bash_completion.d/%[1]s
  # macOS:
  $ %[1]s completion bash > $(brew --prefix)/etc/bash_completion.d/%[1]s

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ %[1]s completion zsh > "${fpath[1]}/_%[1]s"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ %[1]s completion fish | source

  # To load completions for each session, execute once:
  $ %[1]s completion fish > ~/.config/fish/completions/%[1]s.fish

PowerShell:

  PS> %[1]s completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> %[1]s completion powershell > %[1]s.ps1
  # and source this file from your PowerShell profile.
`,cmd.Root().Name()),
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}
```

**Note:** The cobra generator may include messages printed to stdout, for example, if the config file is loaded; this will break the auto-completion script so must be removed.

## Adapting the default completion command

Cobra provides a few options for the default `completion` command.  To configure such options you must set
the `CompletionOptions` field on the *root* command.

To tell Cobra *not* to provide the default `completion` command:
```
rootCmd.CompletionOptions.DisableDefaultCmd = true
```

To tell Cobra to mark the default `completion` command as *hidden*:
```
rootCmd.CompletionOptions.HiddenDefaultCmd = true
```

To tell Cobra *not* to provide the user with the `--no-descriptions` flag to the completion sub-commands:
```
rootCmd.CompletionOptions.DisableNoDescFlag = true
```

To tell Cobra to completely disable descriptions for completions:
```
rootCmd.CompletionOptions.DisableDescriptions = true
```

# Customizing completions

The generated completion scripts will automatically handle completing commands and flags.  However, you can make your completions much more powerful by providing information to complete your program's nouns and flag values.

## Completion of nouns

### Static completion of nouns

Cobra allows you to provide a pre-defined list of completion choices for your nouns using the `ValidArgs` field.
For example, if you want `kubectl get [tab][tab]` to show a list of valid "nouns" you have to set them.
Some simplified code from `kubectl get` looks like:

```go
validArgs = []string{ "pod", "node", "service", "replicationcontroller" }

cmd := &cobra.Command{
	Use:     "get [(-o|--output=)json|yaml|template|...] (RESOURCE [NAME] | RESOURCE/NAME ...)",
	Short:   "Display one or many resources",
	Long:    get_long,
	Example: get_example,
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(RunGet(f, out, cmd, args))
	},
	ValidArgs: validArgs,
}
```

Notice we put the `ValidArgs` field on the `get` sub-command. Doing so will give results like:

```bash
$ kubectl get [tab][tab]
node   pod   replicationcontroller   service
```

#### Aliases for nouns

If your nouns have aliases, you can define them alongside `ValidArgs` using `ArgAliases`:

```go
argAliases = []string { "pods", "nodes", "services", "svc", "replicationcontrollers", "rc" }

cmd := &cobra.Command{
    ...
	ValidArgs:  validArgs,
	ArgAliases: argAliases
}
```

The aliases are shown to the user on tab completion only if no completions were found within sub-commands or `ValidArgs`.

### Dynamic completion of nouns

In some cases it is not possible to provide a list of completions in advance.  Instead, the list of completions must be determined at execution-time. In a similar fashion as for static completions, you can use the `ValidArgsFunction` field to provide a Go function that Cobra will execute when it needs the list of completion choices for the nouns of a command.  Note that either `ValidArgs` or `ValidArgsFunction` can be used for a single cobra command, but not both.
Simplified code from `helm status` looks like:

```go
cmd := &cobra.Command{
	Use:   "status RELEASE_NAME",
	Short: "Display the status of the named release",
	Long:  status_long,
	RunE: func(cmd *cobra.Command, args []string) {
		RunGet(args[0])
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return getReleasesFromCluster(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
}
```
Where `getReleasesFromCluster()` is a Go function that obtains the list of current Helm releases running on the Kubernetes cluster.
Notice we put the `ValidArgsFunction` on the `status` sub-command. Let's assume the Helm releases on the cluster are: `harbor`, `notary`, `rook` and `thanos` then this dynamic completion will give results like:

```bash
$ helm status [tab][tab]
harbor notary rook thanos
```
You may have noticed the use of `cobra.ShellCompDirective`.  These directives are bit fields allowing to control some shell completion behaviors for your particular completion.  You can combine them with the bit-or operator such as `cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp`
```go
// Indicates that the shell will perform its default behavior after completions
// have been provided (this implies none of the other directives).
ShellCompDirectiveDefault

// Indicates an error occurred and completions should be ignored.
ShellCompDirectiveError

// Indicates that the shell should not add a space after the completion,
// even if there is a single completion provided.
ShellCompDirectiveNoSpace

// Indicates that the shell should not provide file completion even when
// no completion is provided.
ShellCompDirectiveNoFileComp

// Indicates that the returned completions should be used as file extension filters.
// For example, to complete only files of the form *.json or *.yaml:
//    return []cobra.Completion{"yaml", "json"}, cobra.ShellCompDirectiveFilterFileExt
// For flags, using MarkFlagFilename() and MarkPersistentFlagFilename()
// is a shortcut to using this directive explicitly.
//
ShellCompDirectiveFilterFileExt

// Indicates that only directory names should be provided in file completion.
// For example:
//    return nil, cobra.ShellCompDirectiveFilterDirs
// For flags, using MarkFlagDirname() is a shortcut to using this directive explicitly.
//
// To request directory names within another directory, the returned completions
// should specify a single directory name within which to search. For example,
// to complete directories within "themes/":
//    return []cobra.Completion{"themes"}, cobra.ShellCompDirectiveFilterDirs
//
ShellCompDirectiveFilterDirs

// ShellCompDirectiveKeepOrder indicates that the shell should preserve the order
// in which the completions are provided
ShellCompDirectiveKeepOrder
```

***Note***: When using the `ValidArgsFunction`, Cobra will call your registered function after having parsed all flags and arguments provided in the command-line.  You therefore don't need to do this parsing yourself.  For example, when a user calls `helm status --namespace my-rook-ns [tab][tab]`, Cobra will call your registered `ValidArgsFunction` after having parsed the `--namespace` flag, as it would have done when calling the `RunE` function.

#### Debugging

Cobra achieves dynamic completion through the use of a hidden command called by the completion script.  To debug your Go completion code, you can call this hidden command directly:
```bash
$ helm __complete status har<ENTER>
harbor
:4
Completion ended with directive: ShellCompDirectiveNoFileComp # This is on stderr
```
***Important:*** If the noun to complete is empty (when the user has not yet typed any letters of that noun), you must pass an empty parameter to the `__complete` command:
```bash
$ helm __complete status ""<ENTER>
harbor
notary
rook
thanos
:4
Completion ended with directive: ShellCompDirectiveNoFileComp # This is on stderr
```
Calling the `__complete` command directly allows you to run the Go debugger to troubleshoot your code.  You can also add printouts to your code; Cobra provides the following functions to use for printouts in Go completion code:
```go
// Prints to the completion script debug file (if BASH_COMP_DEBUG_FILE
// is set to a file path) and optionally prints to stderr.
cobra.CompDebug(msg string, printToStdErr bool)
cobra.CompDebugln(msg string, printToStdErr bool)

// Prints to the completion script debug file (if BASH_COMP_DEBUG_FILE
// is set to a file path) and to stderr.
cobra.CompError(msg string)
cobra.CompErrorln(msg string)
```
***Important:*** You should **not** leave traces that print directly to stdout in your completion code as they will be interpreted as completion choices by the completion script.  Instead, use the cobra-provided debugging traces functions mentioned above.

## Completions for flags

### Mark flags as required

Most of the time completions will only show sub-commands. But if a flag is required to make a sub-command work, you probably want it to show up when the user types [tab][tab].  You can mark a flag as 'Required' like so:

```go
cmd.MarkFlagRequired("pod")
cmd.MarkFlagRequired("container")
```

and you'll get something like

```bash
$ kubectl exec [tab][tab]
-c            --container=  -p            --pod=
```

### Specify dynamic flag completion

As for nouns, Cobra provides a way of defining dynamic completion of flags.  To provide a Go function that Cobra will execute when it needs the list of completion choices for a flag, you must register the function using the `command.RegisterFlagCompletionFunc()` function.

```go
flagName := "output"
cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{"json", "table", "yaml"}, cobra.ShellCompDirectiveDefault
})
```
Notice that calling `RegisterFlagCompletionFunc()` is done through the `command` with which the flag is associated.  In our example this dynamic completion will give results like so:

```bash
$ helm status --output [tab][tab]
json table yaml
```

#### Change the default ShellCompDirective

When no completion function is registered for a leaf command or for a flag, Cobra will
automatically use `ShellCompDirectiveDefault`, which will invoke the shell's filename completion.
This implies that when file completion does not apply to a leaf command or to a flag (the command
or flag does not operate on a filename), turning off file completion requires you to register a
completion function for that command/flag.
For example:

```go
cmd.RegisterFlagCompletionFunc("flag-name", cobra.NoFileCompletions)
```

If you find that there are more situations where file completion should be turned off than
when it is applicable, you can recursively change the default `ShellCompDirective` for a command
and its subcommands to `ShellCompDirectiveNoFileComp`:

```go
cmd.CompletionOptions.SetDefaultShellCompDirective(ShellCompDirectiveNoFileComp)
```

If doing so, keep in mind that you should instead register a completion function for leaf commands or
flags where file completion is applicable. For example:

```go
cmd.RegisterFlagCompletionFunc("flag-name", cobra.FixedCompletions(nil, ShellCompDirectiveDefault))
```

To change the default directive for the entire program, set the DefaultShellCompDirective on the root command.

#### Debugging

You can also easily debug your Go completion code for flags:
```bash
$ helm __complete status --output ""
json
table
yaml
:4
Completion ended with directive: ShellCompDirectiveNoFileComp # This is on stderr
```
***Important:*** You should **not** leave traces that print to stdout in your completion code as they will be interpreted as completion choices by the completion script.  Instead, use the cobra-provided debugging traces functions mentioned further above.

### Specify valid filename extensions for flags that take a filename

To limit completions of flag values to file names with certain extensions you can either use the different `MarkFlagFilename()` functions or a combination of `RegisterFlagCompletionFunc()` and `ShellCompDirectiveFilterFileExt`, like so:
```go
flagName := "output"
cmd.MarkFlagFilename(flagName, "yaml", "json")
```
or
```go
flagName := "output"
cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{"yaml", "json"}, cobra.ShellCompDirectiveFilterFileExt})
```

### Limit flag completions to directory names

To limit completions of flag values to directory names you can either use the `MarkFlagDirname()` functions or a combination of `RegisterFlagCompletionFunc()` and `ShellCompDirectiveFilterDirs`, like so:
```go
flagName := "output"
cmd.MarkFlagDirname(flagName)
```
or
```go
flagName := "output"
cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
})
```
To limit completions of flag values to directory names *within another directory* you can use a combination of `RegisterFlagCompletionFunc()` and `ShellCompDirectiveFilterDirs` like so:
```go
flagName := "output"
cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{"themes"}, cobra.ShellCompDirectiveFilterDirs
})
```
### Descriptions for completions

Cobra provides support for completion descriptions.  Such descriptions are supported for each shell
(however, for bash, it is only available in the [completion V2 version](#bash-completion-v2)).
For commands and flags, Cobra will provide the descriptions automatically, based on usage information.
For example, using zsh:
```
$ helm s[tab]
search  -- search for a keyword in charts
show    -- show information of a chart
status  -- displays the status of the named release
```
while using fish:
```
$ helm s[tab]
search  (search for a keyword in charts)  show  (show information of a chart)  status  (displays the status of the named release)
```

Cobra allows you to add descriptions to your own completions.  Simply add the description text after each completion, following a `\t` separator. Cobra provides the helper function `CompletionWithDesc(string, string)` to create a completion with a description. This technique applies to completions returned by `ValidArgs`, `ValidArgsFunction` and `RegisterFlagCompletionFunc()`.  For example:
```go
ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{
		cobra.CompletionWithDesc("harbor", "An image registry"),
		cobra.CompletionWithDesc("thanos", "Long-term metrics")
		}, cobra.ShellCompDirectiveNoFileComp
}}
```
or
```go
ValidArgs: []cobra.Completion{
	cobra.CompletionWithDesc("bash", "Completions for bash"),
	cobra.CompletionWithDesc("zsh", "Completions for zsh")
	}
```

If you don't want to show descriptions in the completions, you can add `--no-descriptions` to the default `completion` command to disable them, like:

```bash
$ source <(helm completion bash)
$ helm completion [tab][tab]
bash        (generate autocompletion script for bash)        powershell  (generate autocompletion script for powershell)
fish        (generate autocompletion script for fish)        zsh         (generate autocompletion script for zsh)

$ source <(helm completion bash --no-descriptions)
$ helm completion [tab][tab]
bash        fish        powershell  zsh
```

Setting the `<PROGRAM>_COMPLETION_DESCRIPTIONS` environment variable (falling back to `COBRA_COMPLETION_DESCRIPTIONS` if empty or not set) to a [falsey value](https://pkg.go.dev/strconv#ParseBool) achieves the same. `<PROGRAM>` is the name of your program with all non-ASCII-alphanumeric characters replaced by `_`.

## Bash completions

### Dependencies

The bash completion script generated by Cobra requires the `bash_completion` package. You should update the help text of your completion command to show how to install the `bash_completion` package ([Kubectl docs](https://kubernetes.io/docs/tasks/tools/install-kubectl/#enabling-shell-autocompletion))

### Aliases

You can also configure `bash` aliases for your program and they will also support completions.

```bash
alias aliasname=origcommand
complete -o default -F __start_origcommand aliasname

# and now when you run `aliasname` completion will make
# suggestions as it did for `origcommand`.

$ aliasname <tab><tab>
completion     firstcommand   secondcommand
```
### Bash legacy dynamic completions

For backward compatibility, Cobra still supports its bash legacy dynamic completion solution.
Please refer to [Bash Completions](bash.md) for details.

### Bash completion V2

Cobra provides two versions for bash completion.  The original bash completion (which started it all!) can be used by calling
`GenBashCompletion()` or `GenBashCompletionFile()`.

A new V2 bash completion version is also available.  This version can be used by calling `GenBashCompletionV2()` or
`GenBashCompletionFileV2()`.  The V2 version does **not** support the legacy dynamic completion
(see [Bash Completions](bash.md)) but instead works only with the Go dynamic completion
solution described in this document.
Unless your program already uses the legacy dynamic completion solution, it is recommended that you use the bash
completion V2 solution which provides the following extra features:
- Supports completion descriptions (like the other shells)
- Small completion script of less than 300 lines (v1 generates scripts of thousands of lines; `kubectl` for example has a bash v1 completion script of over 13K lines)
- Streamlined user experience thanks to a completion behavior aligned with the other shells

`Bash` completion V2 supports descriptions for completions. When calling `GenBashCompletionV2()` or `GenBashCompletionFileV2()`
you must provide these functions with a parameter indicating if the completions should be annotated with a description; Cobra
will provide the description automatically based on usage information.  You can choose to make this option configurable by
your users.

```
# With descriptions
$ helm s[tab][tab]
search  (search for a keyword in charts)           status  (display the status of the named release)
show    (show information of a chart)

# Without descriptions
$ helm s[tab][tab]
search  show  status
```
**Note**: Cobra's default `completion` command uses bash completion V2.  If for some reason you need to use bash completion V1, you will need to implement your own `completion` command.
## Zsh completions

Cobra supports native zsh completion generated from the root `cobra.Command`.
The generated completion script should be put somewhere in your `$fpath` and be named
`_<yourProgram>`.  You will need to start a new shell for the completions to become available.

Zsh supports descriptions for completions. Cobra will provide the description automatically,
based on usage information. Cobra provides a way to completely disable such descriptions by
using `GenZshCompletionNoDesc()` or `GenZshCompletionFileNoDesc()`. You can choose to make
this a configurable option to your users.
```
# With descriptions
$ helm s[tab]
search  -- search for a keyword in charts
show    -- show information of a chart
status  -- displays the status of the named release

# Without descriptions
$ helm s[tab]
search  show  status
```
*Note*: Because of backward-compatibility requirements, we were forced to have a different API to disable completion descriptions between `zsh` and `fish`.

### Limitations

* Custom completions implemented in Bash scripting (legacy) are not supported and will be ignored for `zsh` (including the use of the `BashCompCustom` flag annotation).
  * You should instead use `ValidArgsFunction` and `RegisterFlagCompletionFunc()` which are portable to the different shells (`bash`, `zsh`, `fish`, `powershell`).
* The function `MarkFlagCustom()` is not supported and will be ignored for `zsh`.
  * You should instead use `RegisterFlagCompletionFunc()`.

### Zsh completions standardization

Cobra 1.1 standardized its zsh completion support to align it with its other shell completions.  Although the API was kept backward-compatible, some small changes in behavior were introduced.
Please refer to [Zsh Completions](zsh.md) for details.

## fish completions

Cobra supports native fish completions generated from the root `cobra.Command`.  You can use the `command.GenFishCompletion()` or `command.GenFishCompletionFile()` functions. You must provide these functions with a parameter indicating if the completions should be annotated with a description; Cobra will provide the description automatically based on usage information.  You can choose to make this option configurable by your users.
```
# With descriptions
$ helm s[tab]
search  (search for a keyword in charts)  show  (show information of a chart)  status  (displays the status of the named release)

# Without descriptions
$ helm s[tab]
search  show  status
```
*Note*: Because of backward-compatibility requirements, we were forced to have a different API to disable completion descriptions between `zsh` and `fish`.

### Limitations

* Custom completions implemented in bash scripting (legacy) are not supported and will be ignored for `fish` (including the use of the `BashCompCustom` flag annotation).
  * You should instead use `ValidArgsFunction` and `RegisterFlagCompletionFunc()` which are portable to the different shells (`bash`, `zsh`, `fish`, `powershell`).
* The function `MarkFlagCustom()` is not supported and will be ignored for `fish`.
  * You should instead use `RegisterFlagCompletionFunc()`.
* The following flag completion annotations are not supported and will be ignored for `fish`:
  * `BashCompFilenameExt` (filtering by file extension)
  * `BashCompSubdirsInDir` (filtering by directory)
* The functions corresponding to the above annotations are consequently not supported and will be ignored for `fish`:
  * `MarkFlagFilename()` and `MarkPersistentFlagFilename()` (filtering by file extension)
  * `MarkFlagDirname()` and `MarkPersistentFlagDirname()` (filtering by directory)
* Similarly, the following completion directives are not supported and will be ignored for `fish`:
  * `ShellCompDirectiveFilterFileExt` (filtering by file extension)
  * `ShellCompDirectiveFilterDirs` (filtering by directory)

## PowerShell completions

Cobra supports native PowerShell completions generated from the root `cobra.Command`. You can use the `command.GenPowerShellCompletion()` or `command.GenPowerShellCompletionFile()` functions. To include descriptions use `command.GenPowerShellCompletionWithDesc()` and `command.GenPowerShellCompletionFileWithDesc()`. Cobra will provide the description automatically based on usage information. You can choose to make this option configurable by your users.

The script is designed to support all three PowerShell completion modes:

* TabCompleteNext (default windows style - on each key press the next option is displayed)
* Complete (works like bash)
* MenuComplete (works like zsh)

You set the mode with `Set-PSReadLineKeyHandler -Key Tab -Function <mode>`. Descriptions are only displayed when using the `Complete` or `MenuComplete` mode.

Users need PowerShell version 5.0 or above, which comes with Windows 10 and can be downloaded separately for Windows 7 or 8.1. They can then write the completions to a file and source this file from their PowerShell profile, which is referenced by the `$Profile` environment variable. See `Get-Help about_Profiles` for more info about PowerShell profiles.

```
# With descriptions and Mode 'Complete'
$ helm s[tab]
search  (search for a keyword in charts)  show  (show information of a chart)  status  (displays the status of the named release)

# With descriptions and Mode 'MenuComplete' The description of the current selected value will be displayed below the suggestions.
$ helm s[tab]
search    show     status

search for a keyword in charts

# Without descriptions
$ helm s[tab]
search  show  status
```
### Aliases

You can also configure `powershell` aliases for your program and they will also support completions.

```
$ sal aliasname origcommand
$ Register-ArgumentCompleter -CommandName 'aliasname' -ScriptBlock $__origcommandCompleterBlock

# and now when you run `aliasname` completion will make
# suggestions as it did for `origcommand`.

$ aliasname <tab>
completion     firstcommand   secondcommand
```
The name of the completer block variable is of the form `$__<programName>CompleterBlock` where every `-` and `:` in the program name have been replaced with `_`, to respect powershell naming syntax.

### Limitations

* Custom completions implemented in bash scripting (legacy) are not supported and will be ignored for `powershell` (including the use of the `BashCompCustom` flag annotation).
  * You should instead use `ValidArgsFunction` and `RegisterFlagCompletionFunc()` which are portable to the different shells (`bash`, `zsh`, `fish`, `powershell`).
* The function `MarkFlagCustom()` is not supported and will be ignored for `powershell`.
  * You should instead use `RegisterFlagCompletionFunc()`.
* The following flag completion annotations are not supported and will be ignored for `powershell`:
  * `BashCompFilenameExt` (filtering by file extension)
  * `BashCompSubdirsInDir` (filtering by directory)
* The functions corresponding to the above annotations are consequently not supported and will be ignored for `powershell`:
  * `MarkFlagFilename()` and `MarkPersistentFlagFilename()` (filtering by file extension)
  * `MarkFlagDirname()` and `MarkPersistentFlagDirname()` (filtering by directory)
* Similarly, the following completion directives are not supported and will be ignored for `powershell`:
  * `ShellCompDirectiveFilterFileExt` (filtering by file extension)
  * `ShellCompDirectiveFilterDirs` (filtering by directory)

## bash.md

# Generating Bash Completions For Your cobra.Command

Please refer to [Shell Completions](_index.md) for details.

## Bash legacy dynamic completions

For backward compatibility, Cobra still supports its legacy dynamic completion solution (described below).  Unlike the `ValidArgsFunction` solution, the legacy solution will only work for Bash shell-completion and not for other shells. This legacy solution can be used along-side `ValidArgsFunction` and `RegisterFlagCompletionFunc()`, as long as both solutions are not used for the same command.  This provides a path to gradually migrate from the legacy solution to the new solution.

**Note**: Cobra's default `completion` command uses bash completion V2.  If you are currently using Cobra's legacy dynamic completion solution, you should not use the default `completion` command but continue using your own.

The legacy solution allows you to inject bash functions into the bash completion script.  Those bash functions are responsible for providing the completion choices for your own completions.

Some code that works in kubernetes:

```bash
const (
        bash_completion_func = `__kubectl_parse_get()
{
    local kubectl_output out
    if kubectl_output=$(kubectl get --no-headers "$1" 2>/dev/null); then
        out=($(echo "${kubectl_output}" | awk '{print $1}'))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__kubectl_get_resource()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        return 1
    fi
    __kubectl_parse_get ${nouns[${#nouns[@]} -1]}
    if [[ $? -eq 0 ]]; then
        return 0
    fi
}

__kubectl_custom_func() {
    case ${last_command} in
        kubectl_get | kubectl_describe | kubectl_delete | kubectl_stop)
            __kubectl_get_resource
            return
            ;;
        *)
            ;;
    esac
}
`)
```

And then I set that in my command definition:

```go
cmds := &cobra.Command{
	Use:   "kubectl",
	Short: "kubectl controls the Kubernetes cluster manager",
	Long: `kubectl controls the Kubernetes cluster manager.

Find more information at https://github.com/GoogleCloudPlatform/kubernetes.`,
	Run: runHelp,
	BashCompletionFunction: bash_completion_func,
}
```

The `BashCompletionFunction` option is really only valid/useful on the root command. Doing the above will cause `__kubectl_custom_func()` (`__<command-use>_custom_func()`) to be called when the built in processor was unable to find a solution. In the case of kubernetes a valid command might look something like `kubectl get pod [mypod]`. If you type `kubectl get pod [tab][tab]` the `__kubectl_customc_func()` will run because the cobra.Command only understood "kubectl" and "get." `__kubectl_custom_func()` will see that the cobra.Command is "kubectl_get" and will thus call another helper `__kubectl_get_resource()`.  `__kubectl_get_resource` will look at the 'nouns' collected. In our example the only noun will be `pod`.  So it will call `__kubectl_parse_get pod`.  `__kubectl_parse_get` will actually call out to kubernetes and get any pods.  It will then set `COMPREPLY` to valid pods!

Similarly, for flags:

```go
	annotation := make(map[string][]string)
	annotation[cobra.BashCompCustom] = []string{"__kubectl_get_namespaces"}

	flag := &pflag.Flag{
		Name:        "namespace",
		Usage:       usage,
		Annotations: annotation,
	}
	cmd.Flags().AddFlag(flag)
```

In addition add the `__kubectl_get_namespaces` implementation in the `BashCompletionFunction`
value, e.g.:

```bash
__kubectl_get_namespaces()
{
    local template
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    local kubectl_out
    if kubectl_out=$(kubectl get -o template --template="${template}" namespace 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out}[*]" -- "$cur" ) )
    fi
}
```

## fish.md

## Generating Fish Completions For Your cobra.Command

Please refer to [Shell Completions](_index.md) for details.

## powershell.md

# Generating PowerShell Completions For Your Own cobra.Command

Please refer to [Shell Completions](_index.md#powershell-completions) for details.

## zsh.md

## Generating Zsh Completion For Your cobra.Command

Please refer to [Shell Completions](_index.md) for details.

## Zsh completions standardization

Cobra 1.1 standardized its zsh completion support to align it with its other shell completions.  Although the API was kept backwards-compatible, some small changes in behavior were introduced.

### Deprecation summary

See further below for more details on these deprecations.

* `cmd.MarkZshCompPositionalArgumentFile(pos, []string{})` is no longer needed.  It is therefore **deprecated** and silently ignored.
* `cmd.MarkZshCompPositionalArgumentFile(pos, glob[])` is **deprecated** and silently ignored.
  * Instead use `ValidArgsFunction` with `ShellCompDirectiveFilterFileExt`.
* `cmd.MarkZshCompPositionalArgumentWords()` is **deprecated** and silently ignored.
  * Instead use `ValidArgsFunction`.

### Behavioral changes

**Noun completion**
|Old behavior|New behavior|
|---|---|
|No file completion by default (opposite of bash)|File completion by default; use `ValidArgsFunction` with `ShellCompDirectiveNoFileComp` to turn off file completion on a per-argument basis|
|Completion of flag names without the `-` prefix having been typed|Flag names are only completed if the user has typed the first `-`|
`cmd.MarkZshCompPositionalArgumentFile(pos, []string{})` used to turn on file completion on a per-argument position basis|File completion for all arguments by default; `cmd.MarkZshCompPositionalArgumentFile()` is **deprecated** and silently ignored|
|`cmd.MarkZshCompPositionalArgumentFile(pos, glob[])` used to turn on file completion **with glob filtering** on a per-argument position basis (zsh-specific)|`cmd.MarkZshCompPositionalArgumentFile()` is **deprecated** and silently ignored; use `ValidArgsFunction` with `ShellCompDirectiveFilterFileExt` for file **extension** filtering (not full glob filtering)|
|`cmd.MarkZshCompPositionalArgumentWords(pos, words[])` used to provide completion choices on a per-argument position basis (zsh-specific)|`cmd.MarkZshCompPositionalArgumentWords()` is **deprecated** and silently ignored; use `ValidArgsFunction` to achieve the same behavior|

**Flag-value completion**

|Old behavior|New behavior|
|---|---|
|No file completion by default (opposite of bash)|File completion by default; use `RegisterFlagCompletionFunc()` with `ShellCompDirectiveNoFileComp` to turn off file completion|
|`cmd.MarkFlagFilename(flag, []string{})` and similar used to turn on file completion|File completion by default; `cmd.MarkFlagFilename(flag, []string{})` no longer needed in this context and silently ignored|
|`cmd.MarkFlagFilename(flag, glob[])`  used to turn on file completion **with glob filtering** (syntax of `[]string{"*.yaml", "*.yml"}` incompatible with bash)|Will continue to work, however, support for bash syntax is added and should be used instead so as to work for all shells (`[]string{"yaml", "yml"}`)|
|`cmd.MarkFlagDirname(flag)` only completes directories (zsh-specific)|Has been added for all shells|
|Completion of a flag name does not repeat, unless flag is of type `*Array` or `*Slice` (not supported by bash)|Retained for `zsh` and added to `fish`|
|Completion of a flag name does not provide the `=` form (unlike bash)|Retained for `zsh` and added to `fish`|

**Improvements**

* Custom completion support (`ValidArgsFunction` and `RegisterFlagCompletionFunc()`)
* File completion by default if no other completions found
* Handling of required flags
* File extension filtering no longer mutually exclusive with bash usage
* Completion of directory names *within* another directory
* Support for `=` form of flags

## _index.md

# Documentation generation

- [Man page docs](man.md)
- [Markdown docs](md.md)
- [Rest docs](rest.md)
- [Yaml docs](yaml.md)

## Options
### `DisableAutoGenTag`

You may set `cmd.DisableAutoGenTag = true`
to _entirely_ remove the auto generated string "Auto generated by spf13/cobra..."
from any documentation source.

### `InitDefaultCompletionCmd`

You may call `cmd.InitDefaultCompletionCmd()` to document the default autocompletion command.

## man.md

# Generating Man Pages For Your Own cobra.Command

Generating man pages from a cobra command is incredibly easy. An example is as follows:

```go
package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "my test program",
	}
	header := &doc.GenManHeader{
		Title: "MINE",
		Section: "3",
	}
	err := doc.GenManTree(cmd, header, "/tmp")
	if err != nil {
		log.Fatal(err)
	}
}
```

That will get you a man page `/tmp/test.3`

## md.md

# Generating Markdown Docs For Your Own cobra.Command

Generating Markdown pages from a cobra command is incredibly easy. An example is as follows:

```go
package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "my test program",
	}
	err := doc.GenMarkdownTree(cmd, "/tmp")
	if err != nil {
		log.Fatal(err)
	}
}
```

That will get you a Markdown document `/tmp/test.md`

## Generate markdown docs for the entire command tree

This program can actually generate docs for the kubectl command in the kubernetes project

```go
package main

import (
	"log"
	"io"
	"os"

	"k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra/doc"
)

func main() {
	kubectl := cmd.NewKubectlCommand(cmdutil.NewFactory(nil), os.Stdin, io.Discard, io.Discard)
	err := doc.GenMarkdownTree(kubectl, "./")
	if err != nil {
		log.Fatal(err)
	}
}
```

This will generate a whole series of files, one for each command in the tree, in the directory specified (in this case "./")

## Generate markdown docs for a single command

You may wish to have more control over the output, or only generate for a single command, instead of the entire command tree. If this is the case you may prefer to `GenMarkdown` instead of `GenMarkdownTree`

```go
	out := new(bytes.Buffer)
	err := doc.GenMarkdown(cmd, out)
	if err != nil {
		log.Fatal(err)
	}
```

This will write the markdown doc for ONLY "cmd" into the out, buffer.

## Customize the output

Both `GenMarkdown` and `GenMarkdownTree` have alternate versions with callbacks to get some control of the output:

```go
func GenMarkdownTreeCustom(cmd *Command, dir string, filePrepender, linkHandler func(string) string) error {
	//...
}
```

```go
func GenMarkdownCustom(cmd *Command, out *bytes.Buffer, linkHandler func(string) string) error {
	//...
}
```

The `filePrepender` will prepend the return value given the full filepath to the rendered Markdown file. A common use case is to add front matter to use the generated documentation with [Hugo](https://gohugo.io/):

```go
const fmTemplate = `---
date: %s
title: "%s"
slug: %s
url: %s
---
`

filePrepender := func(filename string) string {
	now := time.Now().Format(time.RFC3339)
	name := filepath.Base(filename)
	base := strings.TrimSuffix(name, path.Ext(name))
	url := "/commands/" + strings.ToLower(base) + "/"
	return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1), base, url)
}
```

The `linkHandler` can be used to customize the rendered internal links to the commands, given a filename:

```go
linkHandler := func(name string) string {
	base := strings.TrimSuffix(name, path.Ext(name))
	return "/commands/" + strings.ToLower(base) + "/"
}
```

## rest.md

# Generating ReStructured Text Docs For Your Own cobra.Command

Generating ReST pages from a cobra command is incredibly easy. An example is as follows:

```go
package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "my test program",
	}
	err := doc.GenReSTTree(cmd, "/tmp")
	if err != nil {
		log.Fatal(err)
	}
}
```

That will get you a ReST document `/tmp/test.rst`

## Generate ReST docs for the entire command tree

This program can actually generate docs for the kubectl command in the kubernetes project

```go
package main

import (
	"log"
	"io"
	"os"

	"k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra/doc"
)

func main() {
	kubectl := cmd.NewKubectlCommand(cmdutil.NewFactory(nil), os.Stdin, io.Discard, io.Discard)
	err := doc.GenReSTTree(kubectl, "./")
	if err != nil {
		log.Fatal(err)
	}
}
```

This will generate a whole series of files, one for each command in the tree, in the directory specified (in this case "./")

## Generate ReST docs for a single command

You may wish to have more control over the output, or only generate for a single command, instead of the entire command tree. If this is the case you may prefer to `GenReST` instead of `GenReSTTree`

```go
	out := new(bytes.Buffer)
	err := doc.GenReST(cmd, out)
	if err != nil {
		log.Fatal(err)
	}
```

This will write the ReST doc for ONLY "cmd" into the out, buffer.

## Customize the output

Both `GenReST` and `GenReSTTree` have alternate versions with callbacks to get some control of the output:

```go
func GenReSTTreeCustom(cmd *Command, dir string, filePrepender func(string) string, linkHandler func(string, string) string) error {
	//...
}
```

```go
func GenReSTCustom(cmd *Command, out *bytes.Buffer, linkHandler func(string, string) string) error {
	//...
}
```

The `filePrepender` will prepend the return value given the full filepath to the rendered ReST file. A common use case is to add front matter to use the generated documentation with [Hugo](https://gohugo.io/):

```go
const fmTemplate = `---
date: %s
title: "%s"
slug: %s
url: %s
---
`
filePrepender := func(filename string) string {
	now := time.Now().Format(time.RFC3339)
	name := filepath.Base(filename)
	base := strings.TrimSuffix(name, path.Ext(name))
	url := "/commands/" + strings.ToLower(base) + "/"
	return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1), base, url)
}
```

The `linkHandler` can be used to customize the rendered links to the commands, given a command name and reference. This is useful while converting rst to html or while generating documentation with tools like Sphinx where `:ref:` is used:

```go
// Sphinx cross-referencing format
linkHandler := func(name, ref string) string {
    return fmt.Sprintf(":ref:`%s <%s>`", name, ref)
}
```

## yaml.md

# Generating Yaml Docs For Your Own cobra.Command

Generating yaml files from a cobra command is incredibly easy. An example is as follows:

```go
package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "my test program",
	}
	err := doc.GenYamlTree(cmd, "/tmp")
	if err != nil {
		log.Fatal(err)
	}
}
```

That will get you a Yaml document `/tmp/test.yaml`

## Generate yaml docs for the entire command tree

This program can actually generate docs for the kubectl command in the kubernetes project

```go
package main

import (
	"io"
	"log"
	"os"

	"k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra/doc"
)

func main() {
	kubectl := cmd.NewKubectlCommand(cmdutil.NewFactory(nil), os.Stdin, io.Discard, io.Discard)
	err := doc.GenYamlTree(kubectl, "./")
	if err != nil {
		log.Fatal(err)
	}
}
```

This will generate a whole series of files, one for each command in the tree, in the directory specified (in this case "./")

## Generate yaml docs for a single command

You may wish to have more control over the output, or only generate for a single command, instead of the entire command tree. If this is the case you may prefer to `GenYaml` instead of `GenYamlTree`

```go
	out := new(bytes.Buffer)
	doc.GenYaml(cmd, out)
```

This will write the yaml doc for ONLY "cmd" into the out, buffer.

## Customize the output

Both `GenYaml` and `GenYamlTree` have alternate versions with callbacks to get some control of the output:

```go
func GenYamlTreeCustom(cmd *Command, dir string, filePrepender, linkHandler func(string) string) error {
	//...
}
```

```go
func GenYamlCustom(cmd *Command, out *bytes.Buffer, linkHandler func(string) string) error {
	//...
}
```

The `filePrepender` will prepend the return value given the full filepath to the rendered Yaml file. A common use case is to add front matter to use the generated documentation with [Hugo](https://gohugo.io/):

```go
const fmTemplate = `---
date: %s
title: "%s"
slug: %s
url: %s
---
`

filePrepender := func(filename string) string {
	now := time.Now().Format(time.RFC3339)
	name := filepath.Base(filename)
	base := strings.TrimSuffix(name, path.Ext(name))
	url := "/commands/" + strings.ToLower(base) + "/"
	return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1), base, url)
}
```

The `linkHandler` can be used to customize the rendered internal links to the commands, given a filename:

```go
linkHandler := func(name string) string {
	base := strings.TrimSuffix(name, path.Ext(name))
	return "/commands/" + strings.ToLower(base) + "/"
}
```

## projects_using_cobra.md

## Projects using Cobra

- [Allero](https://github.com/allero-io/allero)
- [Arewefastyet](https://benchmark.vitess.io)
- [Arduino CLI](https://github.com/arduino/arduino-cli)
- [Azion](https://github.com/aziontech/azion)
- [Bleve](https://blevesearch.com/)
- [Cilium](https://cilium.io/)
- [CloudQuery](https://github.com/cloudquery/cloudquery)
- [CockroachDB](https://www.cockroachlabs.com/)
- [Conduit](https://github.com/conduitio/conduit)
- [Constellation](https://github.com/edgelesssys/constellation)
- [Cosmos SDK](https://github.com/cosmos/cosmos-sdk)
- [Datree](https://github.com/datreeio/datree)
- [Delve](https://github.com/derekparker/delve)
- [Docker (distribution)](https://github.com/docker/distribution)
- [Encore](https://encore.dev)
- [Etcd](https://etcd.io/)
- [Gardener](https://github.com/gardener/gardenctl)
- [Giant Swarm's gsctl](https://github.com/giantswarm/gsctl)
- [Git Bump](https://github.com/erdaltsksn/git-bump)
- [GitHub CLI](https://github.com/cli/cli)
- [GitHub Labeler](https://github.com/erdaltsksn/gh-label)
- [Golangci-lint](https://golangci-lint.run)
- [GopherJS](https://github.com/gopherjs/gopherjs)
- [GoReleaser](https://goreleaser.com)
- [Helm](https://helm.sh)
- [Hugo](https://gohugo.io)
- [Incus](https://linuxcontainers.org/incus/)
- [Infracost](https://github.com/infracost/infracost)
- [Istio](https://istio.io)
- [Kool](https://github.com/kool-dev/kool)
- [Kubernetes](https://kubernetes.io/)
- [Kubescape](https://github.com/kubescape/kubescape)
- [KubeVirt](https://github.com/kubevirt/kubevirt)
- [Linkerd](https://linkerd.io/)
- [LXC](https://github.com/canonical/lxd)
- [Mattermost-server](https://github.com/mattermost/mattermost-server)
- [Mercure](https://mercure.rocks/)
- [Meroxa CLI](https://github.com/meroxa/cli)
- [Metal Stack CLI](https://github.com/metal-stack/metalctl)
- [Moby (former Docker)](https://github.com/moby/moby)
- [Moldy](https://github.com/Moldy-Community/moldy)
- [Multi-gitter](https://github.com/lindell/multi-gitter)
- [Nanobox](https://github.com/nanobox-io/nanobox)/[Nanopack](https://github.com/nanopack)
- [nFPM](https://nfpm.goreleaser.com)
- [Okteto](https://github.com/okteto/okteto)
- [OpenShift](https://www.openshift.com/)
- [Ory Hydra](https://github.com/ory/hydra)
- [Ory Kratos](https://github.com/ory/kratos)
- [Periscope](https://github.com/anishathalye/periscope)
- [Pixie](https://github.com/pixie-io/pixie)
- [Polygon Edge](https://github.com/0xPolygon/polygon-edge)
- [Pouch](https://github.com/alibaba/pouch)
- [ProjectAtomic (enterprise)](https://www.projectatomic.io/)
- [Prototool](https://github.com/uber/prototool)
- [Pulumi](https://www.pulumi.com)
- [QRcp](https://github.com/claudiodangelis/qrcp)
- [Random](https://github.com/erdaltsksn/random)
- [Rclone](https://rclone.org/)
- [Scaleway CLI](https://github.com/scaleway/scaleway-cli)
- [Sia](https://github.com/SiaFoundation/siad)
- [Skaffold](https://skaffold.dev/)
- [Taikun](https://taikun.cloud/)
- [Tendermint](https://github.com/tendermint/tendermint)
- [Twitch CLI](https://github.com/twitchdev/twitch-cli)
- [UpCloud CLI (`upctl`)](https://github.com/UpCloudLtd/upcloud-cli)
- [Vitess](https://vitess.io)
- VMware's [Tanzu Community Edition](https://github.com/vmware-tanzu/community-edition) & [Tanzu Framework](https://github.com/vmware-tanzu/tanzu-framework)
- [Werf](https://werf.io/)
- [Zarf](https://github.com/defenseunicorns/zarf)
- [ZITADEL](https://github.com/zitadel/zitadel)

## user_guide.md

# User Guide

While you are welcome to provide your own organization, typically a Cobra-based
application will follow the following organizational structure:

```console
▾ appName/
  ▾ cmd/
      add.go
      your.go
      commands.go
      here.go
  main.go
```

In a Cobra app, typically the main.go file is very bare. It serves one purpose: initializing Cobra.

```go
package main

import "{pathToYourApp}/cmd"

func main() {
  cmd.Execute()
}
```

## Using the Cobra Generator

Cobra-CLI is its own program that will create your application and add any commands you want.
It's the easiest way to incorporate Cobra into your application.

For complete details on using the Cobra generator, please refer to [The Cobra-CLI Generator README](https://github.com/spf13/cobra-cli/blob/main/README.md)

## Using the Cobra Library

To manually implement Cobra you need to create a bare main.go file and a rootCmd file.
You will optionally provide additional commands as you see fit.

### Create rootCmd

Cobra doesn't require any special constructors. Simply create your commands.

Ideally you place this in app/cmd/root.go:

```go
var rootCmd = &cobra.Command{
  Use:   "hugo",
  Short: "Hugo is a very fast static site generator",
  Long: `A Fast and Flexible Static Site Generator built with
                love by spf13 and friends in Go.
                Complete documentation is available at https://gohugo.io/documentation/`,
  Run: func(cmd *cobra.Command, args []string) {
    // Do Stuff Here
  },
}

func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
```

You will additionally define flags and handle configuration in your init() function.

For example cmd/root.go:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile     string
	userLicense string

	rootCmd = &cobra.Command{
		Use:   "cobra-cli",
		Short: "A generator for Cobra based Applications",
		Long: `Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	viper.SetDefault("license", "apache")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(initCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cobra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
```

### Create your main.go

With the root command you need to have your main function execute it.
Execute should be run on the root for clarity, though it can be called on any command.

In a Cobra app, typically the main.go file is very bare. It serves one purpose: to initialize Cobra.

```go
package main

import "{pathToYourApp}/cmd"

func main() {
  cmd.Execute()
}
```

### Create additional commands

Additional commands can be defined and typically are each given their own file
inside of the cmd/ directory.

If you wanted to create a version command you would create cmd/version.go and
populate it with the following:

```go
package cmd

import (
  "fmt"

  "github.com/spf13/cobra"
)

func init() {
  rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
  Use:   "version",
  Short: "Print the version number of Hugo",
  Long:  `All software has versions. This is Hugo's`,
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
  },
}
```

### Organizing subcommands

A command may have subcommands which in turn may have other subcommands. This is achieved by using
`AddCommand`. In some cases, especially in larger applications, each subcommand may be defined in
its own go package.

The suggested approach is for the parent command to use `AddCommand` to add its most immediate
subcommands. For example, consider the following directory structure:

```console
├── cmd
│   ├── root.go
│   └── sub1
│       ├── sub1.go
│       └── sub2
│           ├── leafA.go
│           ├── leafB.go
│           └── sub2.go
└── main.go
```

In this case:

* The `init` function of `root.go` adds the command defined in `sub1.go` to the root command.
* The `init` function of `sub1.go` adds the command defined in `sub2.go` to the sub1 command.
* The `init` function of `sub2.go` adds the commands defined in `leafA.go` and `leafB.go` to the
  sub2 command.

This approach ensures the subcommands are always included at compile time while avoiding cyclic
references.

### Returning and handling errors

If you wish to return an error to the caller of a command, `RunE` can be used.

```go
package cmd

import (
  "fmt"

  "github.com/spf13/cobra"
)

func init() {
  rootCmd.AddCommand(tryCmd)
}

var tryCmd = &cobra.Command{
  Use:   "try",
  Short: "Try and possibly fail at something",
  RunE: func(cmd *cobra.Command, args []string) error {
    if err := someFunc(); err != nil {
	return err
    }
    return nil
  },
}
```

The error can then be caught at the execute function call.

## Working with Flags

Flags provide modifiers to control how the action command operates.

### Assign flags to a command

Since the flags are defined and used in different locations, we need to
define a variable outside with the correct scope to assign the flag to
work with.

```go
var Verbose bool
var Source string
```

There are two different approaches to assign a flag.

### Persistent Flags

A flag can be 'persistent', meaning that this flag will be available to the
command it's assigned to as well as every command under that command. For
global flags, assign a flag as a persistent flag on the root.

```go
rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
```

### Local Flags

A flag can also be assigned locally, which will only apply to that specific command.

```go
localCmd.Flags().StringVarP(&Source, "source", "s", "", "Source directory to read from")
```

### Local Flag on Parent Commands

By default, Cobra only parses local flags on the target command, and any local flags on
parent commands are ignored. By enabling `Command.TraverseChildren`, Cobra will
parse local flags on each command before executing the target command.

```go
command := cobra.Command{
  Use: "print [OPTIONS] [COMMANDS]",
  TraverseChildren: true,
}
```

### Bind Flags with Config

You can also bind your flags with [viper](https://github.com/spf13/viper):

```go
var author string

func init() {
  rootCmd.PersistentFlags().StringVar(&author, "author", "YOUR NAME", "Author name for copyright attribution")
  viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
}
```

In this example, the persistent flag `author` is bound with `viper`.
**Note**: the variable `author` will not be set to the value from config,
when the `--author` flag is provided by user.

More in [viper documentation](https://github.com/spf13/viper#working-with-flags).

### Required flags

Flags are optional by default. If instead you wish your command to report an error
when a flag has not been set, mark it as required:

```go
rootCmd.Flags().StringVarP(&Region, "region", "r", "", "AWS region (required)")
rootCmd.MarkFlagRequired("region")
```

Or, for persistent flags:

```go
rootCmd.PersistentFlags().StringVarP(&Region, "region", "r", "", "AWS region (required)")
rootCmd.MarkPersistentFlagRequired("region")
```

### Flag Groups

If you have different flags that must be provided together (e.g. if they provide the `--username` flag they MUST provide the `--password` flag as well) then
Cobra can enforce that requirement:

```go
rootCmd.Flags().StringVarP(&u, "username", "u", "", "Username (required if password is set)")
rootCmd.Flags().StringVarP(&pw, "password", "p", "", "Password (required if username is set)")
rootCmd.MarkFlagsRequiredTogether("username", "password")
```

You can also prevent different flags from being provided together if they represent mutually
exclusive options such as specifying an output format as either `--json` or `--yaml` but never both:

```go
rootCmd.Flags().BoolVar(&ofJson, "json", false, "Output in JSON")
rootCmd.Flags().BoolVar(&ofYaml, "yaml", false, "Output in YAML")
rootCmd.MarkFlagsMutuallyExclusive("json", "yaml")
```

If you want to require at least one flag from a group to be present, you can use `MarkFlagsOneRequired`.
This can be combined with `MarkFlagsMutuallyExclusive` to enforce exactly one flag from a given group:

```go
rootCmd.Flags().BoolVar(&ofJson, "json", false, "Output in JSON")
rootCmd.Flags().BoolVar(&ofYaml, "yaml", false, "Output in YAML")
rootCmd.MarkFlagsOneRequired("json", "yaml")
rootCmd.MarkFlagsMutuallyExclusive("json", "yaml")
```

In these cases:
  - both local and persistent flags can be used
    - **NOTE:** the group is only enforced on commands where every flag is defined
  - a flag may appear in multiple groups
  - a group may contain any number of flags

### Repeated Flags

Cobra supports two types of repeated flags, useful for implementing SSH-like verbose flags (`-v`, `-vv`, `-vvv`) or collecting multiple values.

#### Count Flags

For implementing verbose-style flags where repeated usage increases a counter (like SSH's `-v`, `-vv`, `-vvv`):

```go
var verbose int

func init() {
  // CountVarP allows the flag to be repeated to increment the counter
  rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose output (can be repeated: -v, -vv, -vvv)")
}
```

Usage examples:
- `myapp -v` → verbose = 1 (info level)
- `myapp -vv` or `myapp -v -v` → verbose = 2 (debug level)
- `myapp -vvv` or `myapp -v -v -v` → verbose = 3 (trace level)

Then in your command logic:
```go
Run: func(cmd *cobra.Command, args []string) {
  switch verbose {
  case 0:
    // Default: no verbose output
  case 1:
    // Info level logging
    log.SetLevel(log.InfoLevel)
  case 2:
    // Debug level logging
    log.SetLevel(log.DebugLevel)
  case 3:
    // Trace level logging
    log.SetLevel(log.TraceLevel)
  default:
    // Maximum verbosity
    log.SetLevel(log.TraceLevel)
  }
},
```

#### Array/Slice Flags

For collecting multiple values of the same flag:

```go
var inputFiles []string

func init() {
  // StringArrayVarP allows multiple values: --input file1.txt --input file2.txt
  rootCmd.Flags().StringArrayVarP(&inputFiles, "input", "i", []string{}, "input files (can be repeated)")

  // Alternative: StringSliceVarP for comma-separated values
  // rootCmd.Flags().StringSliceVarP(&inputFiles, "input", "i", []string{}, "input files (comma-separated or repeated)")
}
```

Usage examples:
- `myapp --input file1.txt --input file2.txt`
- `myapp -i file1.txt -i file2.txt`
- With StringSlice: `myapp --input file1.txt,file2.txt,file3.txt`

**Note**: Both `CountVar` and array flags leverage the underlying [pflag](https://github.com/spf13/pflag) library's support for repeated flags.

## Positional and Custom Arguments

Validation of positional arguments can be specified using the `Args` field of `Command`.
The following validators are built in:

- Number of arguments:
  - `NoArgs` - report an error if there are any positional args.
  - `ArbitraryArgs` - accept any number of args.
  - `MinimumNArgs(int)` - report an error if less than N positional args are provided.
  - `MaximumNArgs(int)` - report an error if more than N positional args are provided.
  - `ExactArgs(int)` - report an error if there are not exactly N positional args.
  - `RangeArgs(min, max)` - report an error if the number of args is not between `min` and `max`.
- Content of the arguments:
  - `OnlyValidArgs` - report an error if there are any positional args not specified in the `ValidArgs` field of `Command`, which can optionally be set to a list of valid values for positional args.

If `Args` is undefined or `nil`, it defaults to `ArbitraryArgs`.

Moreover, `MatchAll(pargs ...PositionalArgs)` enables combining existing checks with arbitrary other checks.
For instance, if you want to report an error if there are not exactly N positional args OR if there are any positional
args that are not in the `ValidArgs` field of `Command`, you can call `MatchAll` on `ExactArgs` and `OnlyValidArgs`, as
shown below:

```go
var cmd = &cobra.Command{
  Short: "hello",
  Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hello, World!")
  },
}
```

It is possible to set any custom validator that satisfies `func(cmd *cobra.Command, args []string) error`.
For example:

```go
var cmd = &cobra.Command{
  Short: "hello",
  Args: func(cmd *cobra.Command, args []string) error {
    // Optionally run one of the validators provided by cobra
    if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
        return err
    }
    // Run the custom validation logic
    if myapp.IsValidColor(args[0]) {
      return nil
    }
    return fmt.Errorf("invalid color specified: %s", args[0])
  },
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hello, World!")
  },
}
```

## Example

In the example below, we have defined three commands. Two are at the top level
and one (cmdTimes) is a child of one of the top commands. In this case the root
is not executable, meaning that a subcommand is required. This is accomplished
by not providing a 'Run' for the 'rootCmd'.

We have only defined one flag for a single command.

More documentation about flags is available at https://github.com/spf13/pflag.

```go
package main

import (
  "fmt"
  "strings"

  "github.com/spf13/cobra"
)

func main() {
  var echoTimes int

  var cmdPrint = &cobra.Command{
    Use:   "print [string to print]",
    Short: "Print anything to the screen",
    Long: `print is for printing anything back to the screen.
For many years people have printed back to the screen.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Println("Print: " + strings.Join(args, " "))
    },
  }

  var cmdEcho = &cobra.Command{
    Use:   "echo [string to echo]",
    Short: "Echo anything to the screen",
    Long: `echo is for echoing anything back.
Echo works a lot like print, except it has a child command.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Println("Echo: " + strings.Join(args, " "))
    },
  }

  var cmdTimes = &cobra.Command{
    Use:   "times [string to echo]",
    Short: "Echo anything to the screen more times",
    Long: `echo things multiple times back to the user by providing
a count and a string.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      for i := 0; i < echoTimes; i++ {
        fmt.Println("Echo: " + strings.Join(args, " "))
      }
    },
  }

  cmdTimes.Flags().IntVarP(&echoTimes, "times", "t", 1, "times to echo the input")

  var rootCmd = &cobra.Command{Use: "app"}
  rootCmd.AddCommand(cmdPrint, cmdEcho)
  cmdEcho.AddCommand(cmdTimes)
  rootCmd.Execute()
}
```

For a more complete example of a larger application, please checkout [Hugo](https://gohugo.io/).

## Help Command

Cobra automatically adds a help command to your application when you have subcommands.
This will be called when a user runs 'app help'. Additionally, help will also
support all other commands as input. Say, for instance, you have a command called
'create' without any additional configuration; Cobra will work when 'app help
create' is called.  Every command will automatically have the '--help' flag added.

### Example

The following output is automatically generated by Cobra. Nothing beyond the
command and flag definitions are needed.

```console
$ cobra-cli help

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.

Usage:
  cobra-cli [command]

Available Commands:
  add         Add a command to a Cobra Application
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Initialize a Cobra Application

Flags:
  -a, --author string    author name for copyright attribution (default "YOUR NAME")
      --config string    config file (default is $HOME/.cobra.yaml)
  -h, --help             help for cobra-cli
  -l, --license string   name of license for the project
      --viper            use Viper for configuration

Use "cobra-cli [command] --help" for more information about a command.
```

Help is just a command like any other. There is no special logic or behavior
around it. In fact, you can provide your own if you want.

### Grouping commands in help

Cobra supports grouping of available commands in the help output.  To group commands, each group must be explicitly
defined using `AddGroup()` on the parent command.  Then a subcommand can be added to a group using the `GroupID` element
of that subcommand. The groups will appear in the help output in the same order as they are defined using different
calls to `AddGroup()`.  If you use the generated `help` or `completion` commands, you can set their group ids using
`SetHelpCommandGroupId()` and `SetCompletionCommandGroupId()` on the root command, respectively.

### Defining your own help

You can provide your own Help command or your own template for the default command to use
with the following functions:

```go
cmd.SetHelpCommand(cmd *Command)
cmd.SetHelpFunc(f func(*Command, []string))
cmd.SetHelpTemplate(s string)
```

The latter two will also apply to any children commands.

Note that templates specified with `SetHelpTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Usage Message

When the user provides an invalid flag or invalid command, Cobra responds by
showing the user the 'usage'.

### Example
You may recognize this from the help above. That's because the default help
embeds the usage as part of its output.

```console
$ cobra-cli --invalid
Error: unknown flag: --invalid
Usage:
  cobra-cli [command]

Available Commands:
  add         Add a command to a Cobra Application
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Initialize a Cobra Application

Flags:
  -a, --author string    author name for copyright attribution (default "YOUR NAME")
      --config string    config file (default is $HOME/.cobra.yaml)
  -h, --help             help for cobra-cli
  -l, --license string   name of license for the project
      --viper            use Viper for configuration

Use "cobra [command] --help" for more information about a command.
```

### Defining your own usage

You can provide your own usage function or template for Cobra to use.
Like help, the function and template are overridable through public methods:

```go
cmd.SetUsageFunc(f func(*Command) error)
cmd.SetUsageTemplate(s string)
```

Note that templates specified with `SetUsageTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Version Flag

Cobra adds a top-level '--version' flag if the Version field is set on the root command.
Running an application with the '--version' flag will print the version to stdout using
the version template. The template can be customized using the
`cmd.SetVersionTemplate(s string)` function.

Note that templates specified with `SetVersionTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Error Message Prefix

Cobra prints an error message when receiving a non-nil error value.
The default error message is `Error: <error contents>`.
The Prefix, `Error:` can be customized using the `cmd.SetErrPrefix(s string)` function.

## PreRun and PostRun Hooks

It is possible to run functions before or after the main `Run` function of your command. The `PersistentPreRun` and `PreRun` functions will be executed before `Run`. `PersistentPostRun` and `PostRun` will be executed after `Run`.  The `Persistent*Run` functions will be inherited by children if they do not declare their own.  The `*PreRun` and `*PostRun` functions will only be executed if the `Run` function of the current command has been declared.  These functions are run in the following order:

- `PersistentPreRun`
- `PreRun`
- `Run`
- `PostRun`
- `PersistentPostRun`

An example of two commands which use all of these features is below.  When the subcommand is executed, it will run the root command's `PersistentPreRun` but not the root command's `PersistentPostRun`:

```go
package main

import (
  "fmt"

  "github.com/spf13/cobra"
)

func main() {

  var rootCmd = &cobra.Command{
    Use:   "root [sub]",
    Short: "My root command",
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PersistentPreRun with args: %v\n", args)
    },
    PreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PreRun with args: %v\n", args)
    },
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd Run with args: %v\n", args)
    },
    PostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PostRun with args: %v\n", args)
    },
    PersistentPostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PersistentPostRun with args: %v\n", args)
    },
  }

  var subCmd = &cobra.Command{
    Use:   "sub [no options!]",
    Short: "My subcommand",
    PreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PreRun with args: %v\n", args)
    },
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd Run with args: %v\n", args)
    },
    PostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PostRun with args: %v\n", args)
    },
    PersistentPostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PersistentPostRun with args: %v\n", args)
    },
  }

  rootCmd.AddCommand(subCmd)

  rootCmd.SetArgs([]string{""})
  rootCmd.Execute()
  fmt.Println()
  rootCmd.SetArgs([]string{"sub", "arg1", "arg2"})
  rootCmd.Execute()
}
```

Output:
```
Inside rootCmd PersistentPreRun with args: []
Inside rootCmd PreRun with args: []
Inside rootCmd Run with args: []
Inside rootCmd PostRun with args: []
Inside rootCmd PersistentPostRun with args: []

Inside rootCmd PersistentPreRun with args: [arg1 arg2]
Inside subCmd PreRun with args: [arg1 arg2]
Inside subCmd Run with args: [arg1 arg2]
Inside subCmd PostRun with args: [arg1 arg2]
Inside subCmd PersistentPostRun with args: [arg1 arg2]
```

By default, only the first persistent hook found in the command chain is executed.
That is why in the above output, the `rootCmd PersistentPostRun` was not called for a child command.
Set `EnableTraverseRunHooks` global variable to `true` if you want to execute all parents' persistent hooks.

## Suggestions when "unknown command" happens

Cobra will print automatic suggestions when "unknown command" errors happen. This allows Cobra to behave similarly to the `git` command when a typo happens. For example:

```console
$ hugo srever
Error: unknown command "srever" for "hugo"

Did you mean this?
        server

Run 'hugo --help' for usage.
```

Suggestions are automatically generated based on existing subcommands and use an implementation of [Levenshtein distance](https://en.wikipedia.org/wiki/Levenshtein_distance). Every registered command that matches a minimum distance of 2 (ignoring case) will be displayed as a suggestion.

If you need to disable suggestions or tweak the string distance in your command, use:

```go
command.DisableSuggestions = true
```

or

```go
command.SuggestionsMinimumDistance = 1
```

You can also explicitly set names for which a given command will be suggested using the `SuggestFor` attribute. This allows suggestions for strings that are not close in terms of string distance, but make sense in your set of commands but for which
you don't want aliases. Example:

```console
$ kubectl remove
Error: unknown command "remove" for "kubectl"

Did you mean this?
        delete

Run 'kubectl help' for usage.
```

## Generating documentation for your command

Cobra can generate documentation based on subcommands, flags, etc.
Read more about it in the [docs generation documentation](docgen/_index.md).

## Generating shell completions

Cobra can generate a shell-completion file for the following shells: bash, zsh, fish, PowerShell.
If you add more information to your commands, these completions can be amazingly powerful and flexible.
Read more about it in [Shell Completions](completions/_index.md).

## Providing Active Help

Cobra makes use of the shell-completion system to define a framework allowing you to provide Active Help to your users.
Active Help are messages (hints, warnings, etc) printed as the program is being used.
Read more about it in [Active Help](active_help.md).

## Creating a plugin

When creating a plugin for tools like *kubectl*, the executable is named
`kubectl-myplugin`, but it is used as `kubectl myplugin`. To fix help
messages and completions, annotate the root command with the
`cobra.CommandDisplayNameAnnotation` annotation.

### Example kubectl plugin

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "kubectl-myplugin",
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: "kubectl myplugin",
		},
	}
	subCmd := &cobra.Command{
		Use: "subcmd",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("kubectl myplugin subcmd")
		},
	}
	rootCmd.AddCommand(subCmd)
	rootCmd.Execute()
}
```

Example run as a kubectl plugin:

```console
$ kubectl myplugin
Usage:
  kubectl myplugin [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  subcmd

Flags:
  -h, --help   help for kubectl myplugin

Use "kubectl myplugin [command] --help" for more information about a command.
```

## projects_using_cobra.md

## Projects using Cobra

- [Allero](https://github.com/allero-io/allero)
- [Arewefastyet](https://benchmark.vitess.io)
- [Arduino CLI](https://github.com/arduino/arduino-cli)
- [Azion](https://github.com/aziontech/azion)
- [Bleve](https://blevesearch.com/)
- [Cilium](https://cilium.io/)
- [CloudQuery](https://github.com/cloudquery/cloudquery)
- [CockroachDB](https://www.cockroachlabs.com/)
- [Conduit](https://github.com/conduitio/conduit)
- [Constellation](https://github.com/edgelesssys/constellation)
- [Cosmos SDK](https://github.com/cosmos/cosmos-sdk)
- [Datree](https://github.com/datreeio/datree)
- [Delve](https://github.com/derekparker/delve)
- [Docker (distribution)](https://github.com/docker/distribution)
- [Encore](https://encore.dev)
- [Etcd](https://etcd.io/)
- [Gardener](https://github.com/gardener/gardenctl)
- [Giant Swarm's gsctl](https://github.com/giantswarm/gsctl)
- [Git Bump](https://github.com/erdaltsksn/git-bump)
- [GitHub CLI](https://github.com/cli/cli)
- [GitHub Labeler](https://github.com/erdaltsksn/gh-label)
- [Golangci-lint](https://golangci-lint.run)
- [GopherJS](https://github.com/gopherjs/gopherjs)
- [GoReleaser](https://goreleaser.com)
- [Helm](https://helm.sh)
- [Hugo](https://gohugo.io)
- [Incus](https://linuxcontainers.org/incus/)
- [Infracost](https://github.com/infracost/infracost)
- [Istio](https://istio.io)
- [Kool](https://github.com/kool-dev/kool)
- [Kubernetes](https://kubernetes.io/)
- [Kubescape](https://github.com/kubescape/kubescape)
- [KubeVirt](https://github.com/kubevirt/kubevirt)
- [Linkerd](https://linkerd.io/)
- [LXC](https://github.com/canonical/lxd)
- [Mattermost-server](https://github.com/mattermost/mattermost-server)
- [Mercure](https://mercure.rocks/)
- [Meroxa CLI](https://github.com/meroxa/cli)
- [Metal Stack CLI](https://github.com/metal-stack/metalctl)
- [Moby (former Docker)](https://github.com/moby/moby)
- [Moldy](https://github.com/Moldy-Community/moldy)
- [Multi-gitter](https://github.com/lindell/multi-gitter)
- [Nanobox](https://github.com/nanobox-io/nanobox)/[Nanopack](https://github.com/nanopack)
- [nFPM](https://nfpm.goreleaser.com)
- [Okteto](https://github.com/okteto/okteto)
- [OpenShift](https://www.openshift.com/)
- [Ory Hydra](https://github.com/ory/hydra)
- [Ory Kratos](https://github.com/ory/kratos)
- [Periscope](https://github.com/anishathalye/periscope)
- [Pixie](https://github.com/pixie-io/pixie)
- [Polygon Edge](https://github.com/0xPolygon/polygon-edge)
- [Pouch](https://github.com/alibaba/pouch)
- [ProjectAtomic (enterprise)](https://www.projectatomic.io/)
- [Prototool](https://github.com/uber/prototool)
- [Pulumi](https://www.pulumi.com)
- [QRcp](https://github.com/claudiodangelis/qrcp)
- [Random](https://github.com/erdaltsksn/random)
- [Rclone](https://rclone.org/)
- [Scaleway CLI](https://github.com/scaleway/scaleway-cli)
- [Sia](https://github.com/SiaFoundation/siad)
- [Skaffold](https://skaffold.dev/)
- [Taikun](https://taikun.cloud/)
- [Tendermint](https://github.com/tendermint/tendermint)
- [Twitch CLI](https://github.com/twitchdev/twitch-cli)
- [UpCloud CLI (`upctl`)](https://github.com/UpCloudLtd/upcloud-cli)
- [Vitess](https://vitess.io)
- VMware's [Tanzu Community Edition](https://github.com/vmware-tanzu/community-edition) & [Tanzu Framework](https://github.com/vmware-tanzu/tanzu-framework)
- [Werf](https://werf.io/)
- [Zarf](https://github.com/defenseunicorns/zarf)
- [ZITADEL](https://github.com/zitadel/zitadel)

## user_guide.md

# User Guide

While you are welcome to provide your own organization, typically a Cobra-based
application will follow the following organizational structure:

```console
▾ appName/
  ▾ cmd/
      add.go
      your.go
      commands.go
      here.go
  main.go
```

In a Cobra app, typically the main.go file is very bare. It serves one purpose: initializing Cobra.

```go
package main

import "{pathToYourApp}/cmd"

func main() {
  cmd.Execute()
}
```

## Using the Cobra Generator

Cobra-CLI is its own program that will create your application and add any commands you want.
It's the easiest way to incorporate Cobra into your application.

For complete details on using the Cobra generator, please refer to [The Cobra-CLI Generator README](https://github.com/spf13/cobra-cli/blob/main/README.md)

## Using the Cobra Library

To manually implement Cobra you need to create a bare main.go file and a rootCmd file.
You will optionally provide additional commands as you see fit.

### Create rootCmd

Cobra doesn't require any special constructors. Simply create your commands.

Ideally you place this in app/cmd/root.go:

```go
var rootCmd = &cobra.Command{
  Use:   "hugo",
  Short: "Hugo is a very fast static site generator",
  Long: `A Fast and Flexible Static Site Generator built with
                love by spf13 and friends in Go.
                Complete documentation is available at https://gohugo.io/documentation/`,
  Run: func(cmd *cobra.Command, args []string) {
    // Do Stuff Here
  },
}

func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
```

You will additionally define flags and handle configuration in your init() function.

For example cmd/root.go:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile     string
	userLicense string

	rootCmd = &cobra.Command{
		Use:   "cobra-cli",
		Short: "A generator for Cobra based Applications",
		Long: `Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	viper.SetDefault("license", "apache")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(initCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cobra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
```

### Create your main.go

With the root command you need to have your main function execute it.
Execute should be run on the root for clarity, though it can be called on any command.

In a Cobra app, typically the main.go file is very bare. It serves one purpose: to initialize Cobra.

```go
package main

import "{pathToYourApp}/cmd"

func main() {
  cmd.Execute()
}
```

### Create additional commands

Additional commands can be defined and typically are each given their own file
inside of the cmd/ directory.

If you wanted to create a version command you would create cmd/version.go and
populate it with the following:

```go
package cmd

import (
  "fmt"

  "github.com/spf13/cobra"
)

func init() {
  rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
  Use:   "version",
  Short: "Print the version number of Hugo",
  Long:  `All software has versions. This is Hugo's`,
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
  },
}
```

### Organizing subcommands

A command may have subcommands which in turn may have other subcommands. This is achieved by using
`AddCommand`. In some cases, especially in larger applications, each subcommand may be defined in
its own go package.

The suggested approach is for the parent command to use `AddCommand` to add its most immediate
subcommands. For example, consider the following directory structure:

```console
├── cmd
│   ├── root.go
│   └── sub1
│       ├── sub1.go
│       └── sub2
│           ├── leafA.go
│           ├── leafB.go
│           └── sub2.go
└── main.go
```

In this case:

* The `init` function of `root.go` adds the command defined in `sub1.go` to the root command.
* The `init` function of `sub1.go` adds the command defined in `sub2.go` to the sub1 command.
* The `init` function of `sub2.go` adds the commands defined in `leafA.go` and `leafB.go` to the
  sub2 command.

This approach ensures the subcommands are always included at compile time while avoiding cyclic
references.

### Returning and handling errors

If you wish to return an error to the caller of a command, `RunE` can be used.

```go
package cmd

import (
  "fmt"

  "github.com/spf13/cobra"
)

func init() {
  rootCmd.AddCommand(tryCmd)
}

var tryCmd = &cobra.Command{
  Use:   "try",
  Short: "Try and possibly fail at something",
  RunE: func(cmd *cobra.Command, args []string) error {
    if err := someFunc(); err != nil {
	return err
    }
    return nil
  },
}
```

The error can then be caught at the execute function call.

## Working with Flags

Flags provide modifiers to control how the action command operates.

### Assign flags to a command

Since the flags are defined and used in different locations, we need to
define a variable outside with the correct scope to assign the flag to
work with.

```go
var Verbose bool
var Source string
```

There are two different approaches to assign a flag.

### Persistent Flags

A flag can be 'persistent', meaning that this flag will be available to the
command it's assigned to as well as every command under that command. For
global flags, assign a flag as a persistent flag on the root.

```go
rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
```

### Local Flags

A flag can also be assigned locally, which will only apply to that specific command.

```go
localCmd.Flags().StringVarP(&Source, "source", "s", "", "Source directory to read from")
```

### Local Flag on Parent Commands

By default, Cobra only parses local flags on the target command, and any local flags on
parent commands are ignored. By enabling `Command.TraverseChildren`, Cobra will
parse local flags on each command before executing the target command.

```go
command := cobra.Command{
  Use: "print [OPTIONS] [COMMANDS]",
  TraverseChildren: true,
}
```

### Bind Flags with Config

You can also bind your flags with [viper](https://github.com/spf13/viper):

```go
var author string

func init() {
  rootCmd.PersistentFlags().StringVar(&author, "author", "YOUR NAME", "Author name for copyright attribution")
  viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
}
```

In this example, the persistent flag `author` is bound with `viper`.
**Note**: the variable `author` will not be set to the value from config,
when the `--author` flag is provided by user.

More in [viper documentation](https://github.com/spf13/viper#working-with-flags).

### Required flags

Flags are optional by default. If instead you wish your command to report an error
when a flag has not been set, mark it as required:

```go
rootCmd.Flags().StringVarP(&Region, "region", "r", "", "AWS region (required)")
rootCmd.MarkFlagRequired("region")
```

Or, for persistent flags:

```go
rootCmd.PersistentFlags().StringVarP(&Region, "region", "r", "", "AWS region (required)")
rootCmd.MarkPersistentFlagRequired("region")
```

### Flag Groups

If you have different flags that must be provided together (e.g. if they provide the `--username` flag they MUST provide the `--password` flag as well) then
Cobra can enforce that requirement:

```go
rootCmd.Flags().StringVarP(&u, "username", "u", "", "Username (required if password is set)")
rootCmd.Flags().StringVarP(&pw, "password", "p", "", "Password (required if username is set)")
rootCmd.MarkFlagsRequiredTogether("username", "password")
```

You can also prevent different flags from being provided together if they represent mutually
exclusive options such as specifying an output format as either `--json` or `--yaml` but never both:

```go
rootCmd.Flags().BoolVar(&ofJson, "json", false, "Output in JSON")
rootCmd.Flags().BoolVar(&ofYaml, "yaml", false, "Output in YAML")
rootCmd.MarkFlagsMutuallyExclusive("json", "yaml")
```

If you want to require at least one flag from a group to be present, you can use `MarkFlagsOneRequired`.
This can be combined with `MarkFlagsMutuallyExclusive` to enforce exactly one flag from a given group:

```go
rootCmd.Flags().BoolVar(&ofJson, "json", false, "Output in JSON")
rootCmd.Flags().BoolVar(&ofYaml, "yaml", false, "Output in YAML")
rootCmd.MarkFlagsOneRequired("json", "yaml")
rootCmd.MarkFlagsMutuallyExclusive("json", "yaml")
```

In these cases:
  - both local and persistent flags can be used
    - **NOTE:** the group is only enforced on commands where every flag is defined
  - a flag may appear in multiple groups
  - a group may contain any number of flags

### Repeated Flags

Cobra supports two types of repeated flags, useful for implementing SSH-like verbose flags (`-v`, `-vv`, `-vvv`) or collecting multiple values.

#### Count Flags

For implementing verbose-style flags where repeated usage increases a counter (like SSH's `-v`, `-vv`, `-vvv`):

```go
var verbose int

func init() {
  // CountVarP allows the flag to be repeated to increment the counter
  rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose output (can be repeated: -v, -vv, -vvv)")
}
```

Usage examples:
- `myapp -v` → verbose = 1 (info level)
- `myapp -vv` or `myapp -v -v` → verbose = 2 (debug level)
- `myapp -vvv` or `myapp -v -v -v` → verbose = 3 (trace level)

Then in your command logic:
```go
Run: func(cmd *cobra.Command, args []string) {
  switch verbose {
  case 0:
    // Default: no verbose output
  case 1:
    // Info level logging
    log.SetLevel(log.InfoLevel)
  case 2:
    // Debug level logging
    log.SetLevel(log.DebugLevel)
  case 3:
    // Trace level logging
    log.SetLevel(log.TraceLevel)
  default:
    // Maximum verbosity
    log.SetLevel(log.TraceLevel)
  }
},
```

#### Array/Slice Flags

For collecting multiple values of the same flag:

```go
var inputFiles []string

func init() {
  // StringArrayVarP allows multiple values: --input file1.txt --input file2.txt
  rootCmd.Flags().StringArrayVarP(&inputFiles, "input", "i", []string{}, "input files (can be repeated)")

  // Alternative: StringSliceVarP for comma-separated values
  // rootCmd.Flags().StringSliceVarP(&inputFiles, "input", "i", []string{}, "input files (comma-separated or repeated)")
}
```

Usage examples:
- `myapp --input file1.txt --input file2.txt`
- `myapp -i file1.txt -i file2.txt`
- With StringSlice: `myapp --input file1.txt,file2.txt,file3.txt`

**Note**: Both `CountVar` and array flags leverage the underlying [pflag](https://github.com/spf13/pflag) library's support for repeated flags.

## Positional and Custom Arguments

Validation of positional arguments can be specified using the `Args` field of `Command`.
The following validators are built in:

- Number of arguments:
  - `NoArgs` - report an error if there are any positional args.
  - `ArbitraryArgs` - accept any number of args.
  - `MinimumNArgs(int)` - report an error if less than N positional args are provided.
  - `MaximumNArgs(int)` - report an error if more than N positional args are provided.
  - `ExactArgs(int)` - report an error if there are not exactly N positional args.
  - `RangeArgs(min, max)` - report an error if the number of args is not between `min` and `max`.
- Content of the arguments:
  - `OnlyValidArgs` - report an error if there are any positional args not specified in the `ValidArgs` field of `Command`, which can optionally be set to a list of valid values for positional args.

If `Args` is undefined or `nil`, it defaults to `ArbitraryArgs`.

Moreover, `MatchAll(pargs ...PositionalArgs)` enables combining existing checks with arbitrary other checks.
For instance, if you want to report an error if there are not exactly N positional args OR if there are any positional
args that are not in the `ValidArgs` field of `Command`, you can call `MatchAll` on `ExactArgs` and `OnlyValidArgs`, as
shown below:

```go
var cmd = &cobra.Command{
  Short: "hello",
  Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hello, World!")
  },
}
```

It is possible to set any custom validator that satisfies `func(cmd *cobra.Command, args []string) error`.
For example:

```go
var cmd = &cobra.Command{
  Short: "hello",
  Args: func(cmd *cobra.Command, args []string) error {
    // Optionally run one of the validators provided by cobra
    if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
        return err
    }
    // Run the custom validation logic
    if myapp.IsValidColor(args[0]) {
      return nil
    }
    return fmt.Errorf("invalid color specified: %s", args[0])
  },
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Hello, World!")
  },
}
```

## Example

In the example below, we have defined three commands. Two are at the top level
and one (cmdTimes) is a child of one of the top commands. In this case the root
is not executable, meaning that a subcommand is required. This is accomplished
by not providing a 'Run' for the 'rootCmd'.

We have only defined one flag for a single command.

More documentation about flags is available at https://github.com/spf13/pflag.

```go
package main

import (
  "fmt"
  "strings"

  "github.com/spf13/cobra"
)

func main() {
  var echoTimes int

  var cmdPrint = &cobra.Command{
    Use:   "print [string to print]",
    Short: "Print anything to the screen",
    Long: `print is for printing anything back to the screen.
For many years people have printed back to the screen.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Println("Print: " + strings.Join(args, " "))
    },
  }

  var cmdEcho = &cobra.Command{
    Use:   "echo [string to echo]",
    Short: "Echo anything to the screen",
    Long: `echo is for echoing anything back.
Echo works a lot like print, except it has a child command.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Println("Echo: " + strings.Join(args, " "))
    },
  }

  var cmdTimes = &cobra.Command{
    Use:   "times [string to echo]",
    Short: "Echo anything to the screen more times",
    Long: `echo things multiple times back to the user by providing
a count and a string.`,
    Args: cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
      for i := 0; i < echoTimes; i++ {
        fmt.Println("Echo: " + strings.Join(args, " "))
      }
    },
  }

  cmdTimes.Flags().IntVarP(&echoTimes, "times", "t", 1, "times to echo the input")

  var rootCmd = &cobra.Command{Use: "app"}
  rootCmd.AddCommand(cmdPrint, cmdEcho)
  cmdEcho.AddCommand(cmdTimes)
  rootCmd.Execute()
}
```

For a more complete example of a larger application, please checkout [Hugo](https://gohugo.io/).

## Help Command

Cobra automatically adds a help command to your application when you have subcommands.
This will be called when a user runs 'app help'. Additionally, help will also
support all other commands as input. Say, for instance, you have a command called
'create' without any additional configuration; Cobra will work when 'app help
create' is called.  Every command will automatically have the '--help' flag added.

### Example

The following output is automatically generated by Cobra. Nothing beyond the
command and flag definitions are needed.

```console
$ cobra-cli help

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.

Usage:
  cobra-cli [command]

Available Commands:
  add         Add a command to a Cobra Application
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Initialize a Cobra Application

Flags:
  -a, --author string    author name for copyright attribution (default "YOUR NAME")
      --config string    config file (default is $HOME/.cobra.yaml)
  -h, --help             help for cobra-cli
  -l, --license string   name of license for the project
      --viper            use Viper for configuration

Use "cobra-cli [command] --help" for more information about a command.
```

Help is just a command like any other. There is no special logic or behavior
around it. In fact, you can provide your own if you want.

### Grouping commands in help

Cobra supports grouping of available commands in the help output.  To group commands, each group must be explicitly
defined using `AddGroup()` on the parent command.  Then a subcommand can be added to a group using the `GroupID` element
of that subcommand. The groups will appear in the help output in the same order as they are defined using different
calls to `AddGroup()`.  If you use the generated `help` or `completion` commands, you can set their group ids using
`SetHelpCommandGroupId()` and `SetCompletionCommandGroupId()` on the root command, respectively.

### Defining your own help

You can provide your own Help command or your own template for the default command to use
with the following functions:

```go
cmd.SetHelpCommand(cmd *Command)
cmd.SetHelpFunc(f func(*Command, []string))
cmd.SetHelpTemplate(s string)
```

The latter two will also apply to any children commands.

Note that templates specified with `SetHelpTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Usage Message

When the user provides an invalid flag or invalid command, Cobra responds by
showing the user the 'usage'.

### Example
You may recognize this from the help above. That's because the default help
embeds the usage as part of its output.

```console
$ cobra-cli --invalid
Error: unknown flag: --invalid
Usage:
  cobra-cli [command]

Available Commands:
  add         Add a command to a Cobra Application
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Initialize a Cobra Application

Flags:
  -a, --author string    author name for copyright attribution (default "YOUR NAME")
      --config string    config file (default is $HOME/.cobra.yaml)
  -h, --help             help for cobra-cli
  -l, --license string   name of license for the project
      --viper            use Viper for configuration

Use "cobra [command] --help" for more information about a command.
```

### Defining your own usage

You can provide your own usage function or template for Cobra to use.
Like help, the function and template are overridable through public methods:

```go
cmd.SetUsageFunc(f func(*Command) error)
cmd.SetUsageTemplate(s string)
```

Note that templates specified with `SetUsageTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Version Flag

Cobra adds a top-level '--version' flag if the Version field is set on the root command.
Running an application with the '--version' flag will print the version to stdout using
the version template. The template can be customized using the
`cmd.SetVersionTemplate(s string)` function.

Note that templates specified with `SetVersionTemplate` are evaluated using
`text/template` which can increase the size of the compiled executable.

## Error Message Prefix

Cobra prints an error message when receiving a non-nil error value.
The default error message is `Error: <error contents>`.
The Prefix, `Error:` can be customized using the `cmd.SetErrPrefix(s string)` function.

## PreRun and PostRun Hooks

It is possible to run functions before or after the main `Run` function of your command. The `PersistentPreRun` and `PreRun` functions will be executed before `Run`. `PersistentPostRun` and `PostRun` will be executed after `Run`.  The `Persistent*Run` functions will be inherited by children if they do not declare their own.  The `*PreRun` and `*PostRun` functions will only be executed if the `Run` function of the current command has been declared.  These functions are run in the following order:

- `PersistentPreRun`
- `PreRun`
- `Run`
- `PostRun`
- `PersistentPostRun`

An example of two commands which use all of these features is below.  When the subcommand is executed, it will run the root command's `PersistentPreRun` but not the root command's `PersistentPostRun`:

```go
package main

import (
  "fmt"

  "github.com/spf13/cobra"
)

func main() {

  var rootCmd = &cobra.Command{
    Use:   "root [sub]",
    Short: "My root command",
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PersistentPreRun with args: %v\n", args)
    },
    PreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PreRun with args: %v\n", args)
    },
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd Run with args: %v\n", args)
    },
    PostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PostRun with args: %v\n", args)
    },
    PersistentPostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside rootCmd PersistentPostRun with args: %v\n", args)
    },
  }

  var subCmd = &cobra.Command{
    Use:   "sub [no options!]",
    Short: "My subcommand",
    PreRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PreRun with args: %v\n", args)
    },
    Run: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd Run with args: %v\n", args)
    },
    PostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PostRun with args: %v\n", args)
    },
    PersistentPostRun: func(cmd *cobra.Command, args []string) {
      fmt.Printf("Inside subCmd PersistentPostRun with args: %v\n", args)
    },
  }

  rootCmd.AddCommand(subCmd)

  rootCmd.SetArgs([]string{""})
  rootCmd.Execute()
  fmt.Println()
  rootCmd.SetArgs([]string{"sub", "arg1", "arg2"})
  rootCmd.Execute()
}
```

Output:
```
Inside rootCmd PersistentPreRun with args: []
Inside rootCmd PreRun with args: []
Inside rootCmd Run with args: []
Inside rootCmd PostRun with args: []
Inside rootCmd PersistentPostRun with args: []

Inside rootCmd PersistentPreRun with args: [arg1 arg2]
Inside subCmd PreRun with args: [arg1 arg2]
Inside subCmd Run with args: [arg1 arg2]
Inside subCmd PostRun with args: [arg1 arg2]
Inside subCmd PersistentPostRun with args: [arg1 arg2]
```

By default, only the first persistent hook found in the command chain is executed.
That is why in the above output, the `rootCmd PersistentPostRun` was not called for a child command.
Set `EnableTraverseRunHooks` global variable to `true` if you want to execute all parents' persistent hooks.

## Suggestions when "unknown command" happens

Cobra will print automatic suggestions when "unknown command" errors happen. This allows Cobra to behave similarly to the `git` command when a typo happens. For example:

```console
$ hugo srever
Error: unknown command "srever" for "hugo"

Did you mean this?
        server

Run 'hugo --help' for usage.
```

Suggestions are automatically generated based on existing subcommands and use an implementation of [Levenshtein distance](https://en.wikipedia.org/wiki/Levenshtein_distance). Every registered command that matches a minimum distance of 2 (ignoring case) will be displayed as a suggestion.

If you need to disable suggestions or tweak the string distance in your command, use:

```go
command.DisableSuggestions = true
```

or

```go
command.SuggestionsMinimumDistance = 1
```

You can also explicitly set names for which a given command will be suggested using the `SuggestFor` attribute. This allows suggestions for strings that are not close in terms of string distance, but make sense in your set of commands but for which
you don't want aliases. Example:

```console
$ kubectl remove
Error: unknown command "remove" for "kubectl"

Did you mean this?
        delete

Run 'kubectl help' for usage.
```

## Generating documentation for your command

Cobra can generate documentation based on subcommands, flags, etc.
Read more about it in the [docs generation documentation](docgen/_index.md).

## Generating shell completions

Cobra can generate a shell-completion file for the following shells: bash, zsh, fish, PowerShell.
If you add more information to your commands, these completions can be amazingly powerful and flexible.
Read more about it in [Shell Completions](completions/_index.md).

## Providing Active Help

Cobra makes use of the shell-completion system to define a framework allowing you to provide Active Help to your users.
Active Help are messages (hints, warnings, etc) printed as the program is being used.
Read more about it in [Active Help](active_help.md).

## Creating a plugin

When creating a plugin for tools like *kubectl*, the executable is named
`kubectl-myplugin`, but it is used as `kubectl myplugin`. To fix help
messages and completions, annotate the root command with the
`cobra.CommandDisplayNameAnnotation` annotation.

### Example kubectl plugin

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "kubectl-myplugin",
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: "kubectl myplugin",
		},
	}
	subCmd := &cobra.Command{
		Use: "subcmd",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("kubectl myplugin subcmd")
		},
	}
	rootCmd.AddCommand(subCmd)
	rootCmd.Execute()
}
```

Example run as a kubectl plugin:

```console
$ kubectl myplugin
Usage:
  kubectl myplugin [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  subcmd

Flags:
  -h, --help   help for kubectl myplugin

Use "kubectl myplugin [command] --help" for more information about a command.
```

