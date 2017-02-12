package goannex

import (
	"bufio"
	"errors"
	"io"
	"os"

	"github.com/codeskyblue/go-sh"
)

func init() {
	_, err := sh.Command("git", "version").Output()
	if err != nil {
		panic(err)
	}
	_, err = sh.Command("git-annex", "version", "--raw").Output()
	if err != nil {
		panic(err)
	}
}

type Repo struct {
	Path string
	p    sh.Dir
}

func newRepo(path string) (r *Repo) {
	return &Repo{Path: path, p: sh.Dir(path)}
}

// Creates a new git-annex repository in the specified path
func CreateRepo(path string) (r *Repo, err error) {
	if s, _ := os.Stat(path + "/.git"); s != nil && s.IsDir() {
		return nil, errors.New("Path already contains a git repo")
	}
	r = newRepo(path)
	err = r.cmdNoPanic("git", "init")
	if err != nil {
		return nil, err
	}
	err = r.cmdNoPanic("git-annex", "init")
	if err != nil {
		return nil, err
	}
	return r, nil
}

func OpenRepo(path string) (r *Repo, err error) {
	if s, _ := os.Stat(path + "/.git"); s == nil || !s.IsDir() {
		return nil, errors.New("Path is not a git repo")
	}
	r = newRepo(path)
	return r, nil
}

func (r *Repo) Add(path string) (err error) {
	err = r.cmdNoPanic("git-annex", "add", path)
	return
}

func (r *Repo) Unlock(path string) (err error) {
	err = r.cmdNoPanic("git-annex", "unlock", path)
	return
}

func (r *Repo) Sync() (err error) {
	err = r.cmdNoPanic("git-annex", "sync")
	return
}

func (r *Repo) Commit(msg string) (err error) {
	err = r.cmdNoPanic("git", "commit", "-m", msg)
	return
}

func (r *Repo) Get(path string) (err error) {
	err = r.cmdNoPanic("git-annex", "get", path)
	return
}

func (r *Repo) GetAuto() (err error) {
	err = r.cmdNoPanic("git-annex", "get", "--auto")
	return
}

func (r *Repo) DropAuto() (err error) {
	err = r.cmdNoPanic("git-annex", "drop", "--auto")
	return
}

func (r *Repo) FastFsck() (err error) {
	err = r.cmdNoPanic("git-annex", "fsck", "--fast", "--quiet")
	return
}

func (r *Repo) Remotes() <-chan string {
	c := make(chan string)
	read, write := io.Pipe()
	s := sh.NewSession()
	s.Stdout = write
	go s.Command("git", "remote", r.p).Run()
	go func() {
		defer close(c)
		scanner := bufio.NewScanner(read)
		for scanner.Scan() {
			remote := scanner.Text()
			c <- remote
		}
	}()
	return c
}

func (r *Repo) RemoveRemote(name string) (err error) {
	err = r.cmdNoPanic("git", "remote", "rm", name)
	return
}

func (r *Repo) AddRemote(name string, location string) (err error) {
	err = r.cmdNoPanic("git", "remote", "add", name, location)
	return
}

func (r *Repo) cmd(name string, a ...interface{}) {
	a = append(a, r.p)
	s := sh.NewSession()
	s.ShowCMD = true
	out, err := s.Command(name, a...).CombinedOutput()
	if err != nil {
		panic(errors.New(err.Error() + "\npwd: " + string(r.p) + "\nCommand output:\n" + string(out)))
	}
}

func (r *Repo) cmdNoPanic(name string, a ...interface{}) (err error) {
	defer func() {
		if r, ok := recover().(error); ok {
			err = r
		} else if r != nil {
			panic(r)
		}
	}()
	r.cmd(name, a...)
	return
}
