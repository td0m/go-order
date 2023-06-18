# go-order
> a Go formatter for those very particular about ordering.

Most of us who use Go like to structure their files in the following format:
 - `package ...`
 - `import ...`
 - `const ...`
 - `var ...`
 - `type ...`
 - `func ...`

This is exactly what `go-order` does! This idea was originally implemented by me
in [go-order.nvim](https://github.com/td0m/go-order.nvim), a Lua NeoVim
extension that uses Treesitter. I wanted this to be more widely to allow others
who do not use Neovim to benefit from this, hence the rewrite.

## Installation

To install, simply:

```bash
go install github.com/td0m/go-order@latest
```

## Usage

To sort the file and print the output to stdout:

```bash
go-order main.go
```

To sort and write the results back to the file:

```bash
go-order -f main.go
```

For help:

```bash
go-order -h
```
