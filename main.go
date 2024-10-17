// The idea here is rather than pulling resources at runtime, we
// serve the resources statically.
// I'm thinking about just full on packing the resources into the binary,
// no container bullshit needed
package main

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:modules
var modulesRepo embed.FS

func main() {
	files, err := fs.ReadDir(modulesRepo, "modules")
	if err != nil {
		fmt.Println("Error reading directory:", err)
	}

	for _, file := range files {
		fmt.Println(file.Name())
	}
}
