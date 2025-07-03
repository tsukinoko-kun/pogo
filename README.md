# Pogo

Centralized version control system with a workflow inspired by Jujutsu.

All repository data is stored in a PostgreSQL database.

It is not distributed like Git.

You don't push your changes to the central server, you just make your changes and Pogo will take care of the rest.

You can describe your changes before, during and after you made them.
By starting a new change, you commit your changes and make them immutable.
All changes are directly stored in the repository, so your team can use your changes right away.

## Install client

### From binary

Binary releases are available for Linux, macOS and Windows.

### Homebrew

```sh
brew install tsukinoko-kun/tap/pogo
```

### Scoop

```sh
scoop bucket add tsukinoko-kun https://github.com/tsukinoko-kun/scoop-bucket
scoop install pogo
```

### From source

CGO is optional for the client but recommended. When CGO is enabled, the original C-implementation of Zstd is used. Otherwise, a pure Go re-implementation (which is slower) is used.

You need Sqlc and Protoc (with the Go plugin) installed.

```sh
protoc --go_out=paths=source_relative:. protos/messages.proto
sqlc generate
go install ./bin/pogo/
```

## Server

ghcr.io/tsukinoko-kun/pogo

## Need to know

### SSH

SSH keys are used for authentication.
You need to set up your SSH agent properly and have a key pair to use Pogo.
You can set the public key to us with the `pogo config` command.
Verify that the key is detected by running `pogo whoami`.

On UNIX systems, make sure to set the `SSH_AUTH_SOCK` environment variable to the path of your SSH agent socket.
On Windows, named pipes are used for OpenSSH agents.

## Contributing

Please report bugs and feature requests to the [issue tracker](https://github.com/tsukinoko-kun/pogo/issues).

Pull requests are welcome but please ask first (issue) before starting work on a new feature.

This project is intended to be a safe, welcoming space for collaboration. It is and will always be available to the public under the [MIT License](https://github.com/tsukinoko-kun/pogo/blob/main/LICENSE).

## Building complementary tools

Just import `github.com/tsukinoko-kun/pogo/client` in your Go program.
Take a look at how this package is used in the [cmd](https://github.com/tsukinoko-kun/pogo/tree/main/cmd) package.
Tell me about your tool in the [issue tracker](https://github.com/tsukinoko-kun/pogo/issues) and I might mention it here.

## Props to …

- [github.com/DataDog/zstd](https://pkg.go.dev/github.com/DataDog/zstd) for Go bindings to [Zstd](https://github.com/facebook/zstd) by Meta.
- [github.com/devsisters/go-diff3](https://pkg.go.dev/github.com/devsisters/go-diff3) for implementing a three-way merge with Myers algorithm.
- [github.com/spf13/cobra](https://pkg.go.dev/github.com/spf13/cobra) for a feature-rich CLI framework.
- [github.com/nulab/autog](https://pkg.go.dev/github.com/nulab/autog) for creating visually readable and aesthetically pleasing graphical layouts of directed graphs.
  I have forked it to [tsukinoko-kun/autog](https://github.com/tsukinoko-kun/autog) to add some features that are necessary for my special use case.
- [github.com/Microsoft/go-winio](https://pkg.go.dev/github.com/Microsoft/go-winio) for Windows-specific file operations.
- [github.com/charmbracelet/huh](https://pkg.go.dev/github.com/charmbracelet/huh) for providing a simple text editor that is used as a fallback, when no other editor is found.
- [Protobuf](https://protobuf.dev/) for the server-client communication encoding.
- [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) for making YAML available as the configuration format.
- [github.com/jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5) for the PostgreSQL driver.
