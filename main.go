package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sh "github.com/codeskyblue/go-sh"
	"github.com/go-yaml/yaml"
	"github.com/hypoactiv/autoannex/dirsig"
	"github.com/hypoactiv/autoannex/goannex"
	uuid "github.com/nu7hatch/gouuid"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const DEFAULT_SIGNATURE_FILENAME = ".signature"

var (
	app            = kingpin.New("autoannex", "A simple git-annex automation tool")
	appSigFilename = app.Flag("sig-file", "Signature filename to look for").Default(DEFAULT_SIGNATURE_FILENAME).Short('s').String()
	appSshHosts    = app.Flag("ssh-hosts", "Also look for remote repos on these comma-separated SSH hosts").String()
	appDepth       = app.Flag("depth", "Maximum search depth (default 1)").Default("1").Short('d').Uint()

	syncCmd       = app.Command("sync", "Synchronize a group of repositories")
	syncUuid      = Uuid(syncCmd.Arg("uuid", "Signature UUID of directory group to synchronize").Required())
	syncRmRemotes = syncCmd.Flag("remove-remotes", "Remove all remotes from all repos before synchronizing").Default("false").Bool()
	syncDrop      = syncCmd.Flag("drop", "Run git-annex drop --auto on each repository").Short('D').Default("false").Bool()
	syncGet       = syncCmd.Flag("get", "Run git-annex get --auto on each repository").Short('g').Default("false").Bool()
	syncFastFsck  = syncCmd.Flag("fast-fsck", "Run git-annex fsck --fast --quiet on each repository").Short('F').Default("false").Bool()
	syncAdd       = syncCmd.Flag("add", "Run git-annex add . on each repository before syncing").Short('A').Default("false").Bool()

	exec         = app.Command("exec", "Execute an arbitrary git command on all discovered repositories")
	execUuid     = Uuid(exec.Arg("uuid", "Signature UUID of directory group to execute on").Required())
	execCmd      = StringList(exec.Arg("command", "Git command to execute").Required())
	execParallel = exec.Flag("parallel", "Execute command on all repositories in parallel").Short('p').Bool()

	sig         = app.Command("sig", "Manage signature files")
	sigFind     = sig.Command("find", "Search for signature files")
	sigFindUuid = sigFind.Flag("uuid", "Only look for this signature UUID").String()

	sigNew      = sig.Command("new", "Create a new signature file")
	sigNewPath  = sigNew.Arg("path", "Path in which to create signature file").Required().ExistingDir()
	sigNewForce = sigNew.Flag("force", "Overwrite existing signature file").Default("false").Bool()
)

// kingpin parsers
type UuidValue string

func (u *UuidValue) Set(s string) (err error) {
	_, err = uuid.ParseHex(s)
	*u = UuidValue(s)
	return
}

func (u *UuidValue) String() string {
	return string(*u)
}

func Uuid(s kingpin.Settings) (target *UuidValue) {
	target = new(UuidValue)
	s.SetValue((*UuidValue)(target))
	return
}

type stringList []string

func (sl *stringList) Set(s string) (err error) {
	*sl = append(*sl, s)
	return nil
}

func (sl *stringList) String() string {
	return "not implemented"
}

func (sl *stringList) IsCumulative() bool {
	return true
}

func StringList(s kingpin.Settings) (target *[]string) {
	target = new([]string)
	s.SetValue((*stringList)(target))
	return
}

// Search for signature files on remote hosts via SSH
func findSshRepos(uuid UuidValue) (sshRepos []string) {
	if uuid == "" {
		panic("uuid required")
	}
	if *appSshHosts == "" {
		return nil
	}
	hosts := strings.Split(*appSshHosts, ",")
	collect := make(chan string)
	wg := sync.WaitGroup{}
	for _, i := range hosts {
		// Spawn workers
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			fmt.Println("Looking for repos on", host)
			sshDirsigOut, err := sh.Command("ssh", host, "autoannex", "sig", "find", "--uuid", string(uuid), "--sig-file", *appSigFilename, "-d", *appDepth).Output()
			if err != nil {
				fmt.Println("Error looking for repos on", host)
				fmt.Println(string(sshDirsigOut))
				return
			}
			g := make(map[string][]string)
			err = yaml.Unmarshal(sshDirsigOut, g)
			if err != nil {
				fmt.Println("Error parsing SSH host output from", host, "\n", err)
				return
			}
			repos := g[string(uuid)]
			for _, j := range repos {
				collect <- host + ":" + j
			}
			fmt.Println("Found", len(repos), "repo(s) on", host)
		}(i)
	}
	// Collect results
	go func() {
		wg.Wait()
		close(collect)
	}()
	for i := range collect {
		sshRepos = append(sshRepos, i)
	}
	return
}

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case syncCmd.FullCommand():
		// sync
		sshRepos := findSshRepos(*syncUuid)
		groups := dirsig.Find(*appSigFilename, "", *appDepth)
		if repos, ok := groups[string(*syncUuid)]; ok {
			fmt.Println("Found repository group", *syncUuid, "with", len(repos), "members")
		nextRepo:
			for _, repopath := range repos {
				r, err := goannex.OpenRepo(repopath)
				if err != nil {
					logFile := repopath + "/.git/logs/autoannex.open.log"
					ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
					fmt.Println("There were internal errors. They have been saved to", logFile)
					continue nextRepo
				}
				// Clean up old remotes
				for remote := range r.Remotes() {
					if *syncRmRemotes || strings.HasPrefix(remote, "autoannex-") {
						err = r.RemoveRemote(remote)
						if err != nil {
							logFile := repopath + "/.git/logs/autoannex.rrem.log"
							ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
							fmt.Println("There were internal errors. They have been saved to", logFile)
							continue nextRepo
						}
					}
				}
				// Fully connect found repositories
				for j, remotepath := range repos {
					if remotepath == repopath {
						// Don't add a remote to ourselves
						continue
					}
					err = r.AddRemote("autoannex-"+strconv.Itoa(j), remotepath)
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.rem.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were internal errors. They have been saved to", logFile)
						continue nextRepo
					}
				}
				for j, remotepath := range sshRepos {
					err = r.AddRemote("autoannex-extra"+strconv.Itoa(j), remotepath)
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.extrarem.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were internal errors. They have been saved to", logFile)
						continue nextRepo
					}
				}
				// Add .
				if *syncAdd {
					fmt.Println("Adding new files in", repopath, "...")
					start := time.Now()
					err := r.Add(".")
					fmt.Println("Done. Took", sanePrecision(time.Since(start)))
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.add.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were add errors. They have been saved to", logFile)
					}
				}
				// Sync
				fmt.Println("Now syncing", repopath, "...")
				start := time.Now()
				err = r.Sync()
				fmt.Println("Done. Took", sanePrecision(time.Since(start)))
				if err != nil {
					logFile := repopath + "/.git/logs/autoannex.sync.log"
					ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
					fmt.Println("There were sync errors. They have been saved to", logFile)
				}
				// Drop --auto
				if *syncDrop {
					fmt.Println("Dropping unneeded data from", repopath, "...")
					start := time.Now()
					err = r.DropAuto()
					fmt.Println("Done. Took", sanePrecision(time.Since(start)))
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.drop.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were drop errors. They have been saved to", logFile)
					}
				}
				// Get --auto
				if *syncGet {
					fmt.Println("Copying data to", repopath, "...")
					start := time.Now()
					err = r.GetAuto()
					fmt.Println("Done. Took", sanePrecision(time.Since(start)))
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.get.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were get errors. They have been saved to", logFile)
					}
				}
				// Fast fsck
				if *syncFastFsck {
					fmt.Println("Running fast fsck on", repopath, "...")
					start := time.Now()
					err = r.FastFsck()
					fmt.Println("Done. Took", sanePrecision(time.Since(start)))
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.fsck.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were fsck errors. They have been saved to", logFile)
					}
				}
			}
			if *syncGet || *syncFastFsck || *syncDrop {
				// Resync if things may have changed
				for _, repopath := range repos {
					fmt.Println("Now resyncing", repopath, "...")
					r, err := goannex.OpenRepo(repopath)
					if err != nil {
						fmt.Println("Resync errors:\n", err)
					}
					start := time.Now()
					err = r.Sync()
					fmt.Println("Done. Took", sanePrecision(time.Since(start)))
					if err != nil {
						logFile := repopath + "/.git/logs/autoannex.resync.log"
						ioutil.WriteFile(logFile, []byte(err.Error()), 0644)
						fmt.Println("There were resync errors. They have been saved to", logFile)
					}
				}
			}
		} else {
			fmt.Println("error: could not find any members of\nrepository group", *syncUuid)
			fmt.Println("try increasing maximum search depth")
			return
		}

	case exec.FullCommand():
		// exec
		groups := dirsig.Find(*appSigFilename, "", *appDepth)
		wg := sync.WaitGroup{}
		if repos, ok := groups[string(*execUuid)]; ok {
			fmt.Println("Found repository group", *execUuid, "with", len(repos), "members")
			f := func(repopath string) {
				fmt.Println(repopath)
				out, _ := sh.Command("sh", "-c", "(cd "+repopath+";git "+strings.Join(*execCmd, " ")+")").CombinedOutput()
				fmt.Println(string(out))
				wg.Done()
			}
			for _, repopath := range repos {
				wg.Add(1)
				if *execParallel {
					go f(repopath)
				} else {
					f(repopath)
				}
			}
		}
		sshRepos := findSshRepos(*execUuid)
		if len(sshRepos) > 0 {
			f := func(repopath string) {
				fmt.Println(repopath)
				host, path := split_ab(repopath, ":")
				out, _ := sh.Command("ssh", host, "sh", "-c", "\"cd "+path+";git "+strings.Join(*execCmd, " ")+"\"").CombinedOutput()
				fmt.Println(string(out))
				wg.Done()
			}
			for _, repopath := range sshRepos {
				wg.Add(1)
				if *execParallel {
					go f(repopath)
				} else {
					f(repopath)
				}
			}
		}
		wg.Wait()

	case sigNew.FullCommand():
		dirsigCmdNew()

	case sigFind.FullCommand():
		dirsigCmdFind()

	default:
		panic("not implemented")
	}
}

func split_ab(x, sep string) (a, b string) {
	y := strings.SplitN(x, sep, 2)
	a = y[0]
	b = y[1]
	return
}

func sanePrecision(d time.Duration) time.Duration {
	// If duration is less than ... round to nearest ...
	order := map[time.Duration]time.Duration{
		time.Millisecond: time.Microsecond,
		time.Second:      time.Millisecond,
		time.Minute:      time.Second,
		time.Hour:        time.Second,
	}
	for i, j := range order {
		if d < i {
			if i == 0 {
				return d
			}
			return ((d + j/2) / j) * j
		}
	}
	// More than an hour, round to nearest minute
	return ((d + time.Minute/2) / time.Minute) * time.Minute
}
