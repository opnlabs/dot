// Dot is a local first CI system.
//
// Dot uses Docker to run jobs concurrently in stages. More info can be found at https://github.com/opnlabs/dot/wiki
package main

import (
	"github.com/opnlabs/dot/cmd/dot"
)

func main() {
	dot.Execute()
}
