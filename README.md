# Description
`autoannex` is a simple tool for automating the use of `git-annex` repositories. Among other things, it eases the discovery of repositories on external media or other computers that may not always be present. Once the repositories are discovered, `git-annex` commands such as `git annex sync` and `git annex get --auto` can be run on each repository automatically.

# Usage
First configure your Go environment, and then install `autoannex`.

    $ go get github.com/hypoactiv/autoannex

If you haven't already, create a `git-annex` repository.

    $ mkdir ~/test
    $ cd ~/test
    $ git init
    $ git annex init

Create a signature file, and add it to the repository.

    $ autoannex sig new .
    $ git add .signature
    $ git commit -m "initial commit"

Verify that `autoannex` can find the repository. Note that your repository UUID will be different, as they are randomly generated.

    $ autoannex sig find
    2c20fe8d-0768-4050-6a3b-e180c5f12b25:
    - /home/user/test

Clone the repository

    $ cd ~
    $ git clone ~/test test2

You can now use `autoannex` to discover the two repositories, add them as remotes for each other, and sync them.

    $ autoannex sync $(cat ~/test/.signature)
    Found repository group 2c20fe8d-0768-4050-6a3b-e180c5f12b25 with 2 members
    Now syncing /home/user/test ...
    Done. Took 2s
    Now syncing /home/user/test2 ...
    Done. Took 2s    

`autoannex` can also use SSH to connect to remote machines and look for members of the specified repository groups. Note that `autoannex` must be installed on the target hosts and accessible in `$PATH` for this to function.

    $ autoannex sync $(cat ~/test/.signature) --ssh-hosts=hostA,hostB

You can also run `git-annex fsck,` `git-annex add,` and `git-annex get,` as well as arbitrary `git` commands. Run `autoannex --help` to see full usage.

# How are the repositories discovered?
`autoannex` uses files containing a UUID to mark and later discover repository locations throughout the system. By default, all mount points (via `/proc/mounts`) and the user's home directory are searched recursively to a maximum depth of one. The maximum search depth can be modified to find repositories located deeper in the filesytem.

