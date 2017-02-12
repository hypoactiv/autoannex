# Description
`autoannex` is a simple tool for automating the use of `git-annex` repositories. Among other things, it eases the discovery of repositories on external media or other computers that may not always be present. Once the repositories are discovered, `git-annex` commands such as `git annex sync` and `git annex get --auto` can be run on each repository automatically.

# Usage
If you haven't already, create a `git-annex` repository.
    $ mkdir ~/test
    $ cd ~/test
    $ git init
    $ git annex init

Create a signature file, and add it to the repository.
    $ autoannex dirsig new .

Verify that `autoannex` can find the repository.
    $ autoannex dirsig find

# How are the repositories discovered?
`autoannex` uses files containing a UUID to mark and later discover repository locations throughout the system. By default, all mount points (via `/proc/mounts`) and the user's home directory are searched recursively to a maximum depth of one. The maximum search depth can be modified to find repositories located deeper in the filesytem.
