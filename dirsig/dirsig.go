package dirsig

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"

	uuid "github.com/nu7hatch/gouuid"
)

// Ignore some virtual filesystems
var ignoreFilesystems = map[string]struct{}{
	"binfmt_misc":     struct{}{},
	"cgroup":          struct{}{},
	"configfs":        struct{}{},
	"debugfs":         struct{}{},
	"devpts":          struct{}{},
	"devtmpfs":        struct{}{},
	"fuse.gvfsd-fuse": struct{}{},
	"fusectl":         struct{}{},
	"mqueue":          struct{}{},
	"proc":            struct{}{},
	"pstore":          struct{}{},
	"securityfs":      struct{}{},
	"sysfs":           struct{}{},
	"xenfs":           struct{}{},
}

// Stores a UUID signature to identify a group of directories
type Signature struct {
	UUID     string
	Filename string
}

// Creates a new, random, signature
func NewSignature(filename string) (s *Signature) {
	u, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return &Signature{UUID: u.String(), Filename: filename}
}

func hackExpandStringEscape(in string) string {
	// /proc/mounts (thankfully) replaces spaces with \040, but
	// this causes file not found errors later, so expand to spaces
	return strings.Replace(in, "\\040", " ", -1)
}

// Recursively search all system mounts and the user's home folder for
// signature files named 'filename' to a maximum depth of 'depth.' If
// 'dirHint' is not "" only recurse into folders named 'dirHint'
//
// Returns a map of signature UUIDs to slices of paths sharing that signature
func Find(filename string, dirHint string, depth uint) map[string][]string {
	// Map of maps so that duplicate paths only get recorded once
	groups := make(map[Signature]map[string]struct{})
	m := searchLocations(dirHint, depth)
	for i := range m {
		i = hackExpandStringEscape(i)
		s, err := ReadSignature(i, filename)
		if err != nil {
			continue
		}
		if groups[*s] == nil {
			groups[*s] = make(map[string]struct{})
		}
		groups[*s][i] = struct{}{}
	}
	// Convert map of maps to map of slices
	groupsList := make(map[string][]string)
	for s, j := range groups {
		// Detect and remove aliases (mounts pointing to the same place)
		for k := range j {
			tf, err := ioutil.TempFile(k, ".goannex")
			tf.Close()
			if err == nil {
				for l := range j {
					if l == k {
						continue
					}
					if _, err := os.Stat(l + "/" + filepath.Base(tf.Name())); err == nil {
						delete(j, l)
					}
				}
				os.Remove(tf.Name())
			}
			groupsList[s.UUID] = make([]string, len(j))
			i := 0
			for k := range j {
				groupsList[s.UUID][i] = k
				i++
			}
		}
	}
	return groupsList
}

// Writes the signature to the specified directory.
func (s Signature) Write(dir string) (err error) {
	err = ioutil.WriteFile(s.sigFile(dir), []byte(s.UUID+"\n"), 0644)
	return err
}

// Reads a signature file 'filename' from 'dir'
func ReadSignature(dir string, filename string) (s *Signature, err error) {
	s = &Signature{Filename: filename}
	b, err := ioutil.ReadFile(s.sigFile(dir))
	if err != nil {
		return nil, err
	}
	u, err := uuid.ParseHex(strings.Trim(string(b), "\n"))
	if err != nil {
		return nil, err
	}
	return &Signature{UUID: u.String()}, nil
}

func (s *Signature) sigFile(dir string) string {
	return path.Join(dir, s.Filename)
}

func searchLocations(dirHint string, depth uint) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		usr, err := user.Current()
		if err == nil {
			out <- usr.HomeDir
		}
		m := enumerateMountpoints()
		for s := range m {
			out <- s
		}
	}()
	return recurseSubdirectories(out, dirHint, depth)
}

func recurseSubdirectories(in <-chan string, dirHint string, depth uint) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		wg := sync.WaitGroup{}
		for i := range in {
			i = hackExpandStringEscape(i)
			var r func(string, uint)
			r = func(i string, d uint) {
				defer wg.Done()
				out <- i
				if d == 0 {
					return
				}
				d--
				files, err := ioutil.ReadDir(i)
				if err != nil {
					return
				}
				for _, f := range files {
					if f.IsDir() {
						if dirHint != "" && f.Name() != dirHint {
							// If directory hint is set, only recurse into directories
							// matching the hint
							continue
						}
						wg.Add(1)
						if i[len(i)-1] == '/' {
							go r(i+f.Name(), d)
						} else {
							go r(i+"/"+f.Name(), d)
						}
					}
				}
			}
			wg.Add(1)
			go r(i, depth)
			wg.Wait()
		}
	}()
	return out
}

func enumerateMountpoints() <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		// Read system mounts from /proc/mounts (linux only)
		f, err := os.Open("/proc/mounts")
		if err != nil {
			return
		}
		b := bufio.NewReader(f)
		for {
			line, err := b.ReadString('\n')
			if err != nil {
				// End of file
				break
			}
			line = strings.Trim(line, "\r\n")
			c := strings.SplitN(line, " ", 4)
			if _, ok := ignoreFilesystems[c[2]]; ok {
				// Ignored filesystem type
				continue
			}
			out <- c[1]
		}
	}()
	return out
}
