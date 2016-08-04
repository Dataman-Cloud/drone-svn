package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

// Params stores the git clone parameters used to
// configure and customzie the git clone behavior.
type Params struct {
	Depth           int               `json:"depth"`
	Recursive       bool              `json:"recursive"`
	SkipVerify      bool              `json:"skip_verify"`
	Tags            bool              `json:"tags"`
	Submodules      map[string]string `json:"submodule_override"`
	SubmoduleRemote bool              `json:"submodule_update_remote"`
}

var (
	buildCommit string
)

func main() {
	fmt.Printf("Drone Svn Plugin built from %s\n", buildCommit)

	v := new(Params)
	r := new(plugin.Repo)
	b := new(plugin.Build)
	w := new(plugin.Workspace)
	plugin.Param("repo", r)
	plugin.Param("build", b)
	plugin.Param("workspace", w)
	plugin.Param("vargs", &v)
	plugin.MustParse()

	err := clone(r, b, w, v)
	if err != nil {
		os.Exit(1)
	}
}

// Clone clones the repository and build revision
// into the build workspace.
func clone(r *plugin.Repo, b *plugin.Build, w *plugin.Workspace, v *Params) error {
	err := os.MkdirAll(w.Path, 0777)
	if err != nil {
		fmt.Printf("Error creating directory %s. %s\n", w.Path, err)
		return err
	}

	// write the rsa private key if provided
	if err := writeKey(w); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	var cmds []*exec.Cmd

	// check for a .svn directory and whether it's empty
	if isDirEmpty(filepath.Join(w.Path, ".svn")) {

		cmds = append(cmds, checkoutVersion(b, r.Clone))
	} else {

		cmds = append(cmds, updateVersion(b))
	}

	for _, cmd := range cmds {
		cmd.Dir = w.Path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		trace(cmd)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

// Checkout executes a svn checkout command.
func updateVersion(b *plugin.Build) *exec.Cmd {
	return exec.Command(
		"svn",
		"update",
		"--revision",
		b.Commit,
	)
}

// Checkout executes a svn checkout command.
func checkoutVersion(b *plugin.Build, svnUrl string) *exec.Cmd {
	return exec.Command(
		"svn",
		"checkout",
		"--revision",
		b.Commit,
		fmt.Sprintf("%s/%s", svnUrl, b.Branch),
		".",
	)
}

// Trace writes each command to standard error (preceded by a ‘$ ’) before it
// is executed. Used for debugging your build.
func trace(cmd *exec.Cmd) {
	fmt.Println("$", strings.Join(cmd.Args, " "))
}

// Writes the RSA private key
func writeKey(in *plugin.Workspace) error {
	if in.Keys == nil || len(in.Keys.Private) == 0 {
		return nil
	}
	home := "/root"
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	sshpath := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshpath, 0700); err != nil {
		return err
	}
	confpath := filepath.Join(sshpath, "config")
	privpath := filepath.Join(sshpath, "id_rsa")
	ioutil.WriteFile(confpath, []byte("StrictHostKeyChecking no\n"), 0700)
	return ioutil.WriteFile(privpath, []byte(in.Keys.Private), 0600)
}

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return true
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true
	}
	return false
}
