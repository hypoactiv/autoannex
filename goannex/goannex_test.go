package goannex_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	sh "github.com/codeskyblue/go-sh"
	"github.com/hypoactiv/autoannex/goannex"
)

var (
	r       *goannex.Repo
	td      string
	cleanup bool
)

func TestRepo(t *testing.T) {
	chkerr := func(e error) {
		if e != nil {
			t.Error(e)
			t.FailNow()
		}
	}
	tf := td + "/test"
	chkerr(ioutil.WriteFile(tf, []byte("version 1"), 0644))
	chkerr(r.Add(tf))
	chkerr(r.Commit("goannex test"))
	chkerr(r.Sync())
	// TODO: test that write fails before unlock
	chkerr(r.Unlock(tf))
	chkerr(ioutil.WriteFile(tf, []byte("version 2"), 0644))
	chkerr(r.Add(td))
	chkerr(r.Commit("goannex test"))
	chkerr(r.Sync())
}

func TestRemote(t *testing.T) {
	chkerr := func(e error) {
		if e != nil {
			t.Error(e)
			t.FailNow()
		}
	}
	td2, err := ioutil.TempDir("", "goannex")
	chkerr(err)
	r2, err := goannex.CreateRepo(td2)
	chkerr(err)
	chkerr(r.AddRemote("goannex-test2", td2))
	chkerr(r2.AddRemote("goannex-test", td))
	chkerr(ioutil.WriteFile(td+"/from-1", []byte("this is from repo 1"), 0644))
	chkerr(r.Add(td + "/from-1"))
	chkerr(ioutil.WriteFile(td2+"/from-2", []byte("this is from repo 2"), 0644))
	chkerr(r2.Add(td2 + "/from-2"))
	chkerr(r.Commit("goannex test repo1"))
	chkerr(r2.Commit("goannex test repo2"))
	chkerr(r.Sync())
	chkerr(r2.Sync())
	chkerr(r.Get("from-2"))
	chkerr(r2.Get("from-1"))
	for i := range r2.Remotes() {
		chkerr(r2.RemoveRemote(i))
	}
	for i := range r.Remotes() {
		chkerr(r.RemoveRemote(i))
	}
	for _ = range r2.Remotes() {
		t.FailNow()
	}
	for _ = range r.Remotes() {
		t.FailNow()
	}
	if cleanup {
		chkerr(sh.Command("chmod", "-R", "0700", td2).Run())
		chkerr(os.RemoveAll(td2))
	}
}

func TestMain(m *testing.M) {
	var err error
	chkerr := func(e error) {
		if e != nil {
			panic(e)
		}
	}
	cleanup = func(a string) bool {
		switch strings.ToLower(a) {
		case "no", "false", "0":
			return false
		default:
			return true
		}
	}(os.Getenv("CLEANUP"))
	td, err = ioutil.TempDir("", "goannex")
	chkerr(err)
	r, err = goannex.CreateRepo(td)
	chkerr(err)
	a := m.Run()
	if cleanup {
		chkerr(sh.Command("chmod", "-R", "0700", td).Run())
		chkerr(os.RemoveAll(td))
	}
	os.Exit(a)
}
