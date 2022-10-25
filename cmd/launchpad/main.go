// Copyright 2022 Jetpack Technologies Inc and contributors. All rights reserved.
// Use of this source code is governed by the license in the LICENSE file.

package main

import (
	"context"

	"go.jetpack.io/launchpad/padcli"
)

func main() {
	padcli.New().Run(context.Background())
}
