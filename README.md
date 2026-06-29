# sweb-go-sdk

[![CI](https://github.com/sanchpet/sweb-go-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/sanchpet/sweb-go-sdk/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/sanchpet/sweb-go-sdk.svg)](https://pkg.go.dev/github.com/sanchpet/sweb-go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/sanchpet/sweb-go-sdk)](https://goreportcard.com/report/github.com/sanchpet/sweb-go-sdk)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A Go client for the [SpaceWeb](https://sweb.ru) (sweb.ru) hosting API.

The API speaks JSON-RPC 2.0 over HTTPS. This SDK wraps the transport (envelope,
Bearer auth, error handling) and exposes typed operations grouped into services.
It is the shared foundation for the `sweb` CLI and a future Terraform provider.

## Install

```sh
go get github.com/sanchpet/sweb-go-sdk
```

## Usage

```go
package main

import (
	"context"
	"fmt"

	sweb "github.com/sanchpet/sweb-go-sdk"
)

func main() {
	ctx := context.Background()

	// 1. Exchange credentials for a token (unauthenticated endpoint).
	tmp := sweb.New()
	token, err := tmp.CreateToken(ctx, "login", "password")
	if err != nil {
		panic(err)
	}

	// 2. Use the token for authenticated calls.
	c := sweb.New(sweb.WithToken(token))

	vpsList, err := c.VPS.List(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println(vpsList)
}
```

## Status

Early. The transport (JSON-RPC envelope, auth, error handling) is covered by
tests. Resource result types (VPS list, available config, create) are
provisional and will be firmed up against recorded API responses.

## License

MIT — see [LICENSE](LICENSE).
