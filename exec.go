package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type executionContext struct {
	writeBack        bool
	listFilesDiffer  bool
	errorPresenter   *errorPresenter
	shouldIgnoreFile func(filename string) bool
}

func (ec executionContext) processDir(dirName string) error {
	return filepath.Walk(dirName, func(path string, info os.FileInfo, err error) error {
		shouldProcess := ec.shouldProcessFileOrDir(info)

		if !shouldProcess {
			ec.errorPresenter.Verbosef("ignoring file %v", path)
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		if !info.IsDir() {
			if err := ec.processFile(path); err != nil {
				ec.errorPresenter.Errorf("%v: %v", path, err)
			}
		}
		return nil
	})
}

func (ec executionContext) processFile(filename string) error {
	ec.errorPresenter.Verbosef("processing file %v", filename)

	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "cannot read file")
	}

	gf, err := NewGoFile(bytes.NewReader(fileBytes))
	if err != nil {
		return errors.Wrap(err, "cannot read file")
	}

	gf.SortImportsInPlace()

	formattedFile := new(bytes.Buffer)
	if err := gf.Format(formattedFile); err != nil {
		return errors.Wrap(err, "cannot format file")
	}

	if ec.listFilesDiffer {
		areEqual := bytes.Equal(fileBytes, formattedFile.Bytes())
		if !areEqual {
			ec.errorPresenter.Printf("%v", filename)
		}
	}

	if ec.writeBack {
		f, err := os.Create(filename)
		if err != nil {
			return errors.Wrap(err, "cannot open file for writing")
		}
		defer f.Close()

		if _, err := io.Copy(f, formattedFile); err != nil {
			return errors.Wrap(err, "cannot write file")
		}
	} else {
		io.Copy(os.Stdout, formattedFile)
	}
	return nil
}

func (executionContext) processStdin() error {
	gf, err := NewGoFile(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read file")
	}

	if err := gf.Format(os.Stdout); err != nil {
		return errors.Wrap(err, "cannot format file")
	}
	return nil
}

func (ec executionContext) shouldProcessFileOrDir(info os.FileInfo) bool {
	filename := info.Name()

	// Visit the current directory
	if filename == "." && info.IsDir() {
		return true
	}

	// Skip files or directories starting with '.' or '_'
	if strings.HasPrefix(filename, ".") || strings.HasPrefix(filename, "_") {
		return false
	}

	// Skip vendor directories
	if info.IsDir() && filename == "vendor" {
		return false
	}

	// Skip regular files that do not end in '.go'
	if !info.IsDir() && filepath.Ext(filename) != ".go" {
		return false
	}

	// Additional filter
	if (ec.shouldIgnoreFile != nil) && ec.shouldIgnoreFile(filename) {
		return false
	}

	return true
}
