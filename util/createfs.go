package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	vfs "github.com/mkishere/sshsyrup/virtualfs"
)

var (
	level      int
	startPath  string
	outputFile string
	tree       *vfs.VirtualFS
)

func init() {
	flag.IntVar(&level, "level", 6, "Number of directory level for the program to recurse into")
	flag.StringVar(&startPath, "p", "", "The directory to be imported as root")
	flag.StringVar(&outputFile, "o", "vfs.img", "The output file")
	tree = vfs.Init()
}

func record(p string, f os.FileInfo, err error) error {
	if err != nil && !os.IsExist(err) {
		fmt.Printf("%v not exists\n", p)
		return err
	}
	relDir := strings.TrimPrefix(p, startPath)
	if runtime.GOOS == "windows" {
		relDir = strings.Replace(relDir, "\\", "/", -1)
	}
	fmt.Printf("Reading %v\n", relDir)
	if len(relDir) == 0 {
		return nil
	} else if f.IsDir() {
		err := tree.Mkdir(relDir, f.Mode())
		if err != nil {
			return err
		}
	} else {
		content, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		tree.Mkfile(relDir, f.Mode(), content)
	}
	return nil
}

func main() {
	flag.Parse()

	if len(startPath) == 0 {
		fmt.Println("Path value empty. Run with -help to check")
		os.Exit(0)
	}
	err := filepath.Walk(startPath, record)
	if err != nil {
		fmt.Println("Path not found")
		os.Exit(0)
	}
	fmt.Println("Import finished")
	ma, err := tree.ReadDir("/")
	fmt.Printf("%v", ma)
	file, err := os.Create(outputFile)
	defer file.Close()
	if err != nil {
		fmt.Errorf("Cannot create file: %v", err)
	}
	enc := gob.NewEncoder(file)
	enc.Encode(&tree)
}
