---
title: "Installation"
weight: 1
---
# Installation

To get started with `dbackup`, you need to install the CLI binary.

## Prerequisites

- [Go 1.22+](https://go.dev/doc/install) (if building from source or installing via `go install`)

## Using Go Install

The easiest way to install `dbackup` globally is via `go install`.

{{< tabs "go-install" >}}
{{% tab "Linux / macOS" %}}
```bash
go install github.com/lupppig/dbackup@latest
```
{{% /tab %}}

{{% tab "Windows (PowerShell)" %}}
```powershell
go install github.com/lupppig/dbackup@latest
```
{{% /tab %}}
{{< /tabs >}}

Make sure that your `$GOPATH/bin` directory is in your system `PATH`.

## Building from Source

If you prefer to build the project locally using Make:

{{< tabs "building" >}}
{{% tab "Linux / macOS" %}}
```bash
git clone https://github.com/lupppig/dbackup.git
cd dbackup
make build
# The executable will be in the bin/ directory
./bin/dbackup --help
```
{{% /tab %}}

{{% tab "Windows (Requires Make)" %}}
```powershell
git clone https://github.com/lupppig/dbackup.git
cd dbackup
make build
# The executable will be in the bin/ directory
.\bin\dbackup.exe --help
```
{{% /tab %}}
{{< /tabs >}}
