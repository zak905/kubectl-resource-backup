package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/alecthomas/kingpin"
)

var namespaceFlag string
var dirFlag string

var (
	namespace = kingpin.Flag("namespace", "if the resource is namespaced, this flag sets the namespace scope").Short('n').Default("default").String()
	dir       = kingpin.Flag("dir", "the directory where the resources will be saved").Default(".").String()
	resource  = kingpin.Arg("resource", "the Kubernetes resource to backup. e.g deployment, service,...").Required().String()
)

/* func init() {
	flag.StringVar(&namespaceFlag, "namespace", v1.NamespaceDefault, "if the resource is namespaced, this flag sets the namespace scope")
	flag.StringVar(&dirFlag, "dir", ".", "the director where the resources will be saved")
	fmt.Println("================ called")
} */

func main() {
	//flag.Parse()
	kingpin.Version(runtime.Version())
	kingpin.Parse()
	fmt.Println("================ ", namespaceFlag)

	/* resourceName := os.Args[1]
	fmt.Println(resourceName)
	if len(os.Args) < 1 {
		log.Fatal("usage: kubectl backup <resource name> [flags]")
	} */
	fInfo, err := os.Stat(dirFlag)
	if err != nil {
		log.Fatal(err.Error())
	}

	if !fInfo.IsDir() {
		log.Fatalf("%s is not a directory", dirFlag)
	}

	/* log.Printf("backing up resource %s", resourceName)

	err = backup.BackupResource(resourceName, namespaceFlag, dirFlag)
	if err != nil {
		log.Fatalf("backup failed: %s", err.Error())
	} */
}
