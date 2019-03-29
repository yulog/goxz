package goxz

import (
	"compress/flate"
	"compress/gzip"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver"
	"github.com/pkg/errors"
)

type builder struct {
	name, version                   string
	platform                        *platform
	output, buildLdFlags, buildTags string
	pkgs                            []string
	workDirBase                     string
	zipAlways                       bool
	resources                       []string
}

func (bdr *builder) build() (string, error) {
	dirStuff := []string{bdr.name}
	if bdr.version != "" {
		dirStuff = append(dirStuff, bdr.version)
	}
	dirStuff = append(dirStuff, bdr.platform.os, bdr.platform.arch)
	dirname := strings.Join(dirStuff, "_")
	workDir := filepath.Join(bdr.workDirBase, dirname)
	if err := os.Mkdir(workDir, 0755); err != nil {
		return "", err
	}

	for _, pkg := range bdr.pkgs {
		log.Printf("Building %s for %s/%s\n", pkg, bdr.platform.os, bdr.platform.arch)
		output := bdr.output
		if output == "" {
			output = filepath.Base(pkg)
		}
		cmdArgs := []string{"build", "-o", filepath.Join(workDir, output)}
		if bdr.buildLdFlags != "" {
			cmdArgs = append(cmdArgs, "-ldflags", bdr.buildLdFlags)
		}
		if bdr.buildTags != "" {
			cmdArgs = append(cmdArgs, "-tags", bdr.buildTags)
		}
		cmdArgs = append(cmdArgs, pkg)

		cmd := exec.Command("go", cmdArgs...)
		cmd.Env = append(os.Environ(), "GOOS="+bdr.platform.os, "GOARCH="+bdr.platform.arch)
		bs, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.Wrapf(err,
				"go build failed while building %s for %s/%s with following output:\n%s",
				pkg, bdr.platform.os, bdr.platform.arch, string(bs))
		}
	}
	files, err := ioutil.ReadDir(workDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.Errorf("No binaries are built from [%s] for %s/%s",
			strings.Join(bdr.pkgs, " "), bdr.platform.os, bdr.platform.arch)
	}

	for _, rc := range bdr.resources {
		dest := filepath.Join(workDir, filepath.Base(rc))
		if err := os.Link(rc, dest); err != nil {
			return "", err
		}
	}

	var arch archiver.Archiver = &archiver.Zip{
		CompressionLevel:     flate.DefaultCompression,
		MkdirAll:             true,
		SelectiveCompression: true,
	}
	archiveFilePath := workDir + ".zip"
	if !bdr.zipAlways && bdr.platform.os != "windows" && bdr.platform.os != "darwin" {
		arch = &archiver.TarGz{
			CompressionLevel: gzip.DefaultCompression,
			Tar: &archiver.Tar{
				MkdirAll: true,
			},
		}
		archiveFilePath = workDir + ".tar.gz"
	}
	log.Printf("Archiving %s\n", filepath.Base(archiveFilePath))
	err = arch.Archive([]string{workDir}, archiveFilePath)
	if err != nil {
		return "", err
	}
	return archiveFilePath, nil
}
