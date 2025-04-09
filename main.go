package main

import (
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/zak905/kubectl-resource-backup/internal/backup"
)

var (
	resourceArg   = kingpin.Arg("resource", "the Kubernetes resource to backup. e.g deployment, service,...").Required().String()
	namespaceFlag = kingpin.Flag("namespace", "if the resource is namespaced, this flag sets the namespace scope").Short('n').Default("default").String()
	dirFlag       = kingpin.Flag("dir", "the directory where the resources will be saved").Default(".").String()
)

var Version = "unknown"

func main() {
	kingpin.CommandLine.Name = "kubectl resource-backup"
	kingpin.Version(Version)
	kingpin.Parse()

	directory := *dirFlag
	resource := *resourceArg
	namespace := *namespaceFlag

	fInfo, err := os.Stat(directory)
	if err != nil {
		log.Fatal(err.Error())
	}

	if !fInfo.IsDir() {
		log.Fatalf("%s is not a directory", directory)
	}

	err = backup.BackupResource(resource, namespace, directory)
	if err != nil {
		log.Fatalf("backup failed: %s", err.Error())
	}
}
