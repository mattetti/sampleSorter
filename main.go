package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

/*
Given a keyword, a source folder and a destination folder we want the script to
look for all matching filenames in the source folder and move them to the
destination but also group them by a maximum of 128 files per subfolder folder.
*/

var (
	flagSource      = flag.String("src", "", "Path to look for samples")
	flagKeyword     = flag.String("keyword", "", "Keyword to look for in samples")
	flagDestination = flag.String("dest", "", "Destination of where to put the filtered samples (defaults to your user folder)")
	flagGroupSize   = flag.Int("perFolder", 128, "Maximum of samples per destination sub folder")
	flagDryRun      = flag.Bool("dry", false, "Enable a dry run where files aren't really copied")
	flagDebug       = flag.Bool("debug", false, "Enable debugging logs")
	flagMax         = flag.Int("max", 1000, "Max samples to be moved")

	matchingPaths = []string{}
)

func main() {
	flag.Parse()
	if *flagSource == "" {
		log.Println("You need to pass a source path to search: -src=<path where to search>")
		flag.Usage()
		os.Exit(1)
	}
	if *flagKeyword == "" {
		log.Println("You need to pass a keyword to search for: -keyword=<path where to search>")
		flag.Usage()
		os.Exit(1)
	}
	*flagKeyword = strings.ToLower(*flagKeyword)

	usr, err := user.Current()
	if err != nil {
		log.Println("Failed to get the user home directory")
		os.Exit(1)
	}

	if *flagDestination == "" {
		*flagDestination = usr.HomeDir
	}

	// expand the paths
	sourcePath := *flagSource
	if sourcePath[:2] == "~/" {
		sourcePath = strings.Replace(sourcePath, "~", usr.HomeDir, 1)
	}
	destPath := *flagDestination
	if destPath[:2] == "~/" {
		destPath = strings.Replace(destPath, "~", usr.HomeDir, 1)
	}

	// recursively search for matching file names in the src folder
	matchingPaths, err = findMatchingFiles(sourcePath, *flagKeyword)
	if err != nil {
		log.Println("Something went wrong looking for matching files", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d matching files to copy\n", len(matchingPaths))

	// TODO: ask Dot if he wants to sort the matches
	// TODO: dedupe the files

	groupIdx := 1
	fileIdx := 0
	files := []string{}
	// loop through all the matches and group them by 128 and copy them in their own folders.
	for i, filePath := range matchingPaths {
		if i >= *flagMax {
			fmt.Println("We reached the max amount of samples to copy:", *flagMax)
			break
		}
		// check if we filled up our group yet
		if fileIdx >= 128 {
			// reset our counter
			fileIdx = 0
			// copy the files to the group folder
			if err := copyFilesToGroup(files, destPath, groupIdx); err != nil {
				log.Printf("Something went wrong when copying the matching files into the group %d folder - %s\n", groupIdx, err)
			}
			// increase the group id
			groupIdx++
			// reset the files slice so we can fill it up again
			files = []string{}
		}
		// add the file to the slice
		files = append(files, filePath)
		// increment the file index
		fileIdx++
	}
	// copy the left overs
	if len(files) > 0 {
		if err := copyFilesToGroup(files, destPath, groupIdx); err != nil {
			log.Printf("Something went wrong when copying the matching files into the group %d folder - %s\n", groupIdx, err)
		}
	}
	fmt.Println("Your files are in", destPath)
}

func findMatchingFiles(src, keyword string) (matchPaths []string, err error) {
	if src == "" {
		return nil, fmt.Errorf("missing source folder location")
	}

	fullPath, err := filepath.Abs(src)
	if err != nil {
		return nil, fmt.Errorf("couldn't get the absolute path of the source - %s", err)
	}

	err = filepath.Walk(fullPath, visit)
	return matchingPaths, err
}

// find matching files
func visit(path string, fi os.FileInfo, err error) (e error) {
	if fi.IsDir() {
		return nil
	}

	// test match, if we match, let's add to the matchingPaths
	filename := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(filename)
	if ext != ".wav" && ext != ".aiff" && ext != ".aif" {
		return nil
	}
	if strings.Contains(filename, *flagKeyword) {
		if *flagDebug {
			fmt.Println("match found:", path)
		}
		matchingPaths = append(matchingPaths, path)
	}

	return nil
}

// copyFilesToGroup copies the srcPaths to destPath inside a subfolder named after the idx
func copyFilesToGroup(srcPaths []string, destPath string, idx int) error {
	subFolderPath := filepath.Join(destPath, fmt.Sprintf("group_%d", idx))
	os.MkdirAll(subFolderPath, 0777)
	fmt.Printf("Copying %d files to %s\n", len(srcPaths), subFolderPath)
	// TODO: make sure we don't have 2 files with the same filename
	for _, src := range srcPaths {
		filename := filepath.Base(src)
		dest := filepath.Join(subFolderPath, filename)
		if *flagDebug {
			fmt.Printf("Copying %s to %s\n", src, dest)
		}
		if err := copyFileContents(src, dest); err != nil {
			log.Printf("Failed to copy %s to %s, continuing anyway - %s", src, dest, err)
			continue
		}
	}
	return nil
}

func copyFileContents(src, dst string) (err error) {
	if *flagDryRun {
		log.Printf("Copying %s to %s\n", src, dst)
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
