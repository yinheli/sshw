// +build mage

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

// tidy code
func Fmt() error {
	packages := strings.Split("cmd", " ")
	files, _ := filepath.Glob("*.go")
	packages = append(packages, files...)
	return sh.Run("gofmt", append([]string{"-s", "-l", "-w"}, packages...)...)
}

// for local machine build
func Build() error {
	return buildTarget(runtime.GOOS, runtime.GOARCH, nil)
}

// build all platform
func Pack() error {
	buildTarget("darwin", "amd64", nil)
	buildTarget("darwin", "386", nil)
	buildTarget("freebsd", "amd64", nil)
	buildTarget("freebsd", "386", nil)
	buildTarget("linux", "amd64", nil)
	buildTarget("linux", "386", nil)
	buildTarget("linux", "arm", nil)
	buildTarget("linux", "arm64", nil)
	buildTarget("windows", "amd64", nil)
	buildTarget("windows", "386", nil)
	return genCheckSum()
}

// Test all packages
func Test() error {
	err := sh.RunV("go", "test", "-v", "-coverprofile", ".cover.out", "./...")
	if err != nil {
		return err
	}

	err = sh.RunV("go", "tool", "cover", "-func=.cover.out")
	if err != nil {
		return err
	}

	err = sh.Run("go", "tool", "cover", "-html=.cover.out", "-o", ".cover.html")
	if err != nil {
		return err
	}

	return sh.Rm(".cover.out")
}

// build to target (cross build)
func buildTarget(OS, arch string, envs map[string]string) error {
	tag := tag()
	name := fmt.Sprintf("sshw-%s-%s-%s", OS, arch, tag)
	dir := fmt.Sprintf("dist/%s", name)
	target := fmt.Sprintf("%s/sshw", dir)

	args := make([]string, 0, 10)
	args = append(args, "build", "-o", target)
	args = append(args, "-ldflags", flags(), "cmd/sshw/main.go")

	fmt.Println("build", target)
	env := make(map[string]string)
	env["GOOS"] = OS
	env["GOARCH"] = arch
	env["CGO_ENABLED"] = "0"

	if envs != nil {
		for k, v := range envs {
			env[k] = v
		}
	}

	if err := sh.RunWith(env, mg.GoCmd(), args...); err != nil {
		return err
	}

	sh.Run("tar", "-czf", fmt.Sprintf("%s.tar.gz", dir), "-C", "dist", name)

	return nil
}

func flags() string {
	hash := hash()
	tag := tag()
	return fmt.Sprintf(`-s -w -X "main.Build=%s-%s" -extldflags "-static"`, tag, hash)
}

// tag returns the git tag for the current branch or "" if none.
func tag() string {
	s, _ := sh.Output("bash", "-c", "git describe --abbrev=0 --tags 2> /dev/null")
	if s == "" {
		return "dev"
	}
	return s
}

// hash returns the git hash for the current repo or "" if none.
func hash() string {
	hash, _ := sh.Output("git", "rev-parse", "--short", "HEAD")
	return hash
}

func mod() string {
	f, err := os.Open("go.mod")
	if err == nil {
		reader := bufio.NewReader(f)
		line, _, _ := reader.ReadLine()
		return strings.Replace(string(line), "module ", "", 1)
	}
	return ""
}

// cleanup all build files
func Clean() {
	sh.Rm("dist")
	sh.Rm(".cover.html")
}

func genCheckSum() error {
	fmt.Println("generate checksum.txt file")
	fs, err := ioutil.ReadDir("dist")
	if err != nil {
		return err
	}

	file, err := os.OpenFile("dist/checksum.txt", os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, f := range fs {
		if f.IsDir() {
			continue
		}

		if strings.HasSuffix(f.Name(), ".tar.gz") {
			sum, _ := fileHash(fmt.Sprintf("dist/%s", f.Name()))
			fmt.Println(sum, f.Name())
			file.WriteString(fmt.Sprintf("%s  %s\n", sum, f.Name()))
		}
	}
	return nil
}

func fileHash(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	hash := sha1.New()
	io.Copy(hash, file)
	ret := hash.Sum(nil)
	return hex.EncodeToString(ret[:]), nil
}
