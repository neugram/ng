# The Neugram Shell

A shell is a computer text interface. You type commands and the shell
executes them. The Neugram scripting language has an embedded shell.
It has been designed for interactive use as a shell on a computer in
the Unix style.

The Neugram shell uses Unix syntax for common shell operations such as
executing commands, changing directories, pipes, and I/O redirection.
If you are familiar with another Unix shell, such as the Bourne shell,
then you already know a lot about the Neugram shell.

This document assumes no familiarity with any Unix shell, and is
intended as an introduction and reference.

_As of January 2017 the Neugram scripting language itself is an early
prototype and not ready for anything more than experimentation.
However the embedded shell has enough features to be used as an
interactive shell replacement._

## A shell session

```
ng>
ng> $$
ng$ echo "Directory list:" > dir_list.txt
ng$ $$
ng> import "strings"
ng> files := $$ ls $$
ng> for i, name := range strings.Split(files, "\n") {
..>	$$ echo "$i. $name" >> dir_list.txt $$
..> }
ng> $$
ng$ cat dir_list.txt
Directory list:
0. dir_list.txt
1. hello.go
2. shell.md
ng$
```

This shell session creates a new file, `dir_list.txt`. It uses
several standard Unix shell utilities: ls, echo, and cat.
It also uses common unix I/O redirection syntax: > and >>, to send
output from a command to a file.

These shell commands are bookended by $$. Inside a $$-expression,
Neugram processes Unix-style commands. If the $$-expression is
executed at the top-level of the interpreter (as it is in the
beginning of the example session above), command lines are
interpreted as they are entered. If you open a $$-expression at
the start and spend most of your time in it, the Neugram interpreter
acts like an interactive shell.

Where Neugram's embedded shell diverges from typical Unix shells
is in the control flow constructs: if, for, and similar. It doesn't
have any. Instead, control flow is handled by using the surrounding
scripting language. In the example above, a shell command is executed
in a loop defined in the scripting language:

```
for i, name := range strings.Split(files, "\n") {
	$$ echo "$i. $name" >> dir_list.txt $$
}
```

On each pass through the loop, the shell command is executed,
appending a line to the file `dir_list.txt`.

## $$-expressions

A `$$ ... $$` expression in Neugram is a sequence of shell commands.
If used as a top-level statement in the interpreter, commands are
executed as they are parsed. Otherwise, shell commands are executed
when the $$-expression is evaluated according to standard Neugram
rules.

The $$-expression stops evaluating when a shell command returns a
non-zero return value. The expression:

```
$$ echo one; false; echo two $$
```

Prints:

```
one
exit code: 1
```

This means scripts act as if they are running under set -e. The
exception to this is interactive sessions with top-level $$-expressions
being evaluated as commands are parsed: here execution will continue.

The $$-expression returns two values. The first is the combined
output written to STDOUT and STDERR, the second is of type error.
(To avoid excessive memory consumption, output is not collected if
no name is given to the output variable)

## Error handling

If a shell command exits with a non-zero return value, an error is
returned. The Neugram language will automatically transform an
elided error into a stack-unwinding panic, so an uncaught error will
stop the execution of the program.

Consider the same error under four different conditions:

```
ng> x, err := $$ echo one; false $$ // error reported in err
ng> x
one
ng> err
shell.exitError{code:1}
ng>
ng> x := $$ echo one; false $$      // error becomes a panic
neugram panic: exit code: 1
ng> x, _ := $$ echo one; false $$   // error is ignored
ng> x
one
ng> $$
ng$ echo one
one
ng$ false
exit code: 1                        // error printed, shell continues
ng$ echo two
two
ng$ $$
ng>
```

## Shell Grammar

### Simple Commands

A simple command is an optional sequence of variable assignments,
followed by space-separated words and I/O redirections.

The first word is the command to execute. The command can be specified
either as a name, a relative path, or an absolute path. An unadorned
name is resolved using the PATH variable.

Command     | 
------------|--------------
ls          | Unadorned command, resolved via PATH to /usr/bin/ls
../bin/ls   | Relative path
/usr/bin/ls | Absolute path

Subsequent words are passed to the command as arguments.
For example, `ls /` runs /usr/bin/ls, passing it '/' as a parameter
which the `ls` command interprets as the directory to list.

Variable assignments that come before the command become part of
envrionment used to execute the command.

### Pipelines

A pipeline is a sequence of commands separated by '|'. The STDOUT of
the command on the left is connected to the STDIN of the command on
the right.

The pipeline connection is done before any redirections are processed.

Each command is executed as a separate process concurrently.

### Lists

A list is a sequence of commands or pipelines. The list elements are
separated by `;`, `&`, `&&`, or `||`.

Commands separated by `;` are executed sequentially.

Commands that end in `&` are executed in the background,
the next command begins executing immediately.

When two commands are separated by `&&`, the second one is executed
after the first if and only if the first succeeds (exits with code 0).

When two commands are separated by `||`, the second one is executed
after the first if and only if the first fails (non-zero exit code).

One or more newlines is equivalent to `;`.

## Redirection

The input and output of a command can be redirected.

Input can be redirected using `<`. The redirection `[n]<path`
opens the file named `path` for reading on file descriptor `n`.
If `n` is ommitted, the file is directed to STDIN.

Output can be redirected using `>`. The redirection `[n]>path`
directs file descriptor `n` to the file named `path`.
If `n` is ommitted, STDOUT is directed to the file.

Output can be redirected and appended to a file using `[n]>>path`.

Both STDOUT and STDERR can be redirected together using `&>`.

## Quoting

Quoting turns special control characters into literal text.
There are three ways to quote: \-prefixing, ''-quoting, and "-quoting.

A backslash (`\`) quotes the character that follows it, unless the
character is a newline, in which case the pair is erased and the next
line continues.

Single-quotes (`''`) quotes every character between the quotes.
A single-quote cannot appear between single-quotes.

Double-quotes (`""`) quotes every character between the quotes, with
the exception of `$` and `\`. A `$` has the same special control
meaning inside double quotes as it does out. A `\` only escapes the
characters `$`, `\`, `"`, and newline. This means a double-quote can
appear inside a double-quote when escaped by a preceding `\`.

## Variables

Variables are named values.

When a $$-expression is evaluated it collects the named values in the
current Neugram scope.

Inside a $$-expression, a variable can be assigned with `name=value`.
The variable is set for the remainder of the list of commands. A
$$-expression defines a scope and for the purpose of Neugram setting
a variable in the shell acts as a declaration.

The value of a variable can be used in a shell command by prefixing
its name with the special control character `$`.

For example:

```
x := "value-1"
$$
echo "x: $x"       # prints "x: value-1"
x=value-2
echo "x: $x"       # prints "x: value-2"
$$
print("x: " + x)  // prints "x: value-1"
```

A single shell command can be prefixed by a variable assignment that
will be made part of the environment of the executed command.

TODO

## Expansion

When the shell processes a command it expands special control
characters in the words of the command. There are several kinds
of expansion, organized into phases. The phases are: 1. braces,
2. tildes, 3. parameters, and 4. paths.

### Brace Expansion

Brace expansion generates strings, creating new shell words.
There are two kinds of brace expansion, `,` and `..` expansion.

In both cases, the brace expands to a sequence of strings. The
prefix and suffix of the word containing the brace expression is
appended to the sequence.

A brace containing `,` creates a sequence out of the values to the
left and right of the comma.

For example, `a{b,c,}d` expands to `abd acd ad`.

A brace containing `..` takes the numbers each side of the `..`
and generates a string for each number between the first and last.

For example, `a{3..6}d` expands to `a3d a4d a5d a6d`.

## Tilde Expansion

If the first character of a word is `~`, then the `~` and the suffix
of characters that follow it up until the first path separator are
replaced.

If there is no suffix, then the `~` is replaced with $HOME.

If the suffix is equal to a login name, then the `~` and the suffix
are replaced with the home directory of that user.

Otherwise the ~ and its suffix are left unchanged.

## Parameter Expansion

The control character `$` in a shell word is the prefix for a
parameter name. It invokes parameter expansion.

The non-separator, non-control characters that directly follow the
`$` make up a parameter name. The `$` and the name are placed with
the value of the parameter.

If the `$` is directly followed by an unescaped `{`, then the
open-brace, until the next unescaped close-brace, make up a parameter
pattern which is expanded based on the following patterns:

- `${param}`:
  Use the value of the variable named *param*.
- `${param[i]}`:
  Use the value of the array, slice or map value at index *i*.
- `${param/regexp/replacement}`:
  Apply the regular expression *regexp* to the value of of the
  variable named *param*. All matches of the regular expression are
  replaced with *replacement*. This follows the semantics of Go's
  `regexp.Regexp.ReplaceAllString` method.
- `${param[offset:length]}`:
  Substring selection. The value of the variable *param* is sliced
  like a string.
- `${param:-word}`:
  Use the value of the variable named *param*, unless there is no
  such named variable, in which case the value *word* is substituted.

TODO: these parameter expansions are not yet supported:

## Path Expansion

If a shell word contains the control character `*`, `?`, or `[`, then
the word is a pattern and is replaced with a list of matching file
paths.

The pattern is expanded into file paths according to the semantics of
Go's `filepath.Match` function. Roughly: `*` matches zero or more
non-separator characters, `?` matches exactly one non-separator
character, and [ch0-ch1] matches a character range.

## Jobs

Shell commands and pipelines start jobs. A job can execute in the
foreground, attached to the shell's STDIN and STDOUT, a job can be
suspended and resumed at a later time, and a job can be executed in
the background while the shell continues processing input.

A job has at least one current system process associated with it,
more if the job is a pipeline. Each job is numbered, the shell
numbering jobs sequentially when they are started.

By default jobs are started in the foreground, attached to STDIN and
STDOUT. A foreground job can be suspended by sending a signal, SIGINT.
Typically Unix terminal emulators map this to Ctrl+Z.

A command can also be started in a running background job by
terminating the command with `&`.

The built-in shell command `jobs` prints the currently suspended and
background running jobs.

The built-in shell command `fg [number]` moves resumes (or removes
from the background) a job, attaching it to STDIN and STDOUT. If no
job number is specified, the shell resumes the job with the lowest
number.

The built-in shell command `bg [number]` takes a suspended job and
resumes it in the background. If no job number is specified the job
with the lowest number is used.

For example:

```
ng$ vim
# VIM takes over the screen.
# User presses Ctrl+Z
[1]+ Stopped vim
ng$ jobs
[1]+ Stopped vim
ng$ fg
# VIM resumes on the screen.
# User exits
ng$
```
