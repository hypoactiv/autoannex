package main

import (
	"fmt"
	"os"
	"path"

	"github.com/go-yaml/yaml"
	"github.com/hypoactiv/autoannex/dirsig"
)

// Starts a new group with a random UUID, and makes dir a member of the group
func dirsigCmdNew() {
	dir := *sigNewPath
	file := path.Join(dir, *appSigFilename)
	fmt.Println("placing new signature in", file)
	if !*sigNewForce {
		if _, err := os.Stat(file); err == nil {
			fmt.Println(file, "already exists, won't overwrite without --force")
			return
		}
	}
	err := dirsig.NewSignature(*appSigFilename).Write(dir)
	if err != nil {
		fmt.Println("unable to create signature:", err)
		return
	}
}

func dirsigCmdFind() {
	groups := dirsig.Find(*appSigFilename, "", *appDepth)
	g := make(map[string][]string)
	for s := range groups {
		if *sigFindUuid != "" && s != *sigFindUuid {
			continue
		}
		g[s] = groups[s]
	}
	y, err := yaml.Marshal(g)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(y))
}
