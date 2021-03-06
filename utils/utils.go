package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/mitchellh/go-homedir"
	"github.com/rahveiz/topomate/config"
	"github.com/spf13/viper"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func PrintError(args ...interface{}) (n int, err error) {
	return fmt.Fprintln(os.Stderr, args...)
}

func Fatalln(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func Fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// ExecSudo is equivalent to exec.Command with sudo prefixed
func ExecSudo(arg ...string) *exec.Cmd {
	return exec.Command("sudo", arg...)
}

func ExecDocker(cName string, arg ...string) {
	args := []string{"exec", cName}
	args = append(args, arg...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		fmt.Println(args)
		Fatalln(string(out), err)
	}
}

// GetHome returns the home directory of the user. If sudo is used, it returns
// the original user home directory.
func GetHome() string {
	home := os.Getenv("HOME")
	// Find home directory.
	if usr, sudo := os.LookupEnv("SUDO_USER"); sudo {
		u, err := user.Lookup(usr)
		if err != nil {
			Fatalln("Error looking for user", u)
		}
		home = u.HomeDir
	}
	return home
}

// GetDirectoryFromKey returns the directory name specified by the given key
// in the configuration file, and creates it if it does not exists
func GetDirectoryFromKey(key, defaultPath string) string {
	// Check if a directory is configured
	if viper.IsSet(key) {
		d := viper.GetString(key)
		configDir, err := homedir.Expand(d)
		if err != nil {
			Fatalln(err)
		}
		stat, err := os.Stat(configDir)
		if err == nil {
			if !stat.IsDir() {
				Fatalf("GetDirectoryFromKey: specified path (%s) is not a directory\n", configDir)
			}
			return configDir
		}

		if os.IsNotExist(err) { // create directory if it is not present yet
			if e := os.MkdirAll(configDir, os.ModeDir|os.ModePerm); e != nil {
				Fatalln("GetDirectoryFromKey: error creating directory")
			}
			return configDir
		}
		Fatalf("GetDirectoryFromKey: configured directory error: %v\n", err)
	}

	defaultDir, err := homedir.Expand(defaultPath)
	if err != nil {
		Fatalln(err)
	}

	if _, err := os.Stat(defaultDir); os.IsNotExist(err) {
		if e := os.Mkdir(defaultDir, os.ModeDir|os.ModePerm); e != nil {
			Fatalf("GetDirectoryFromKey: error creating directory at %s", defaultDir)
		}
	} else if err != nil {
		Fatalf("GetDirectoryFromKey: configured directory error: %v\n", err)
	}
	return defaultDir
}

// PullImages pulls the latest version of docker images used by topomate
func PullImages() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	if config.VFlag {
		fmt.Print("Pulling latest router image... ")
	}
	out, err := cli.ImagePull(ctx, config.DockerRouterImage, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	if config.VFlag {
		fmt.Println("Done.")
	}

	if config.VFlag {
		fmt.Print("Pulling latest route-server image... ")
	}
	out, err = cli.ImagePull(ctx, config.DockerRSImage, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	if config.VFlag {
		fmt.Println("Done.")
	}

	defer out.Close()
	if _, err := ioutil.ReadAll(out); err != nil {
		panic(err)
	}
}

func StartFrr(cName string) {
	cmd := exec.Command(
		"docker",
		"exec",
		cName,
		"/usr/lib/frr/frrinit.sh",
		"start",
	)
	out, err := cmd.CombinedOutput()
	if config.VFlag {
		fmt.Println(cmd)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s %v\n", cName, string(out), err)
	}
}

func ResolveFilePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return config.ConfigDir + "/" + path
}
