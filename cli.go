package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)


func yesno(prompt string, defaultResp bool) bool {
    if defaultResp {
        fmt.Printf("%s [Y/n]: ", prompt)
    } else {
        fmt.Printf("%s [y/N]: ", prompt)
    }

    resp := ""
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()

    if scanner.Err() == nil {
        resp = scanner.Text()
    } else {
        return defaultResp
    }

    resp = strings.ToLower(resp)

    if (defaultResp && (resp == "n" || resp == "no")) || (!defaultResp && (resp == "y" || resp == "yes")) {
        return !defaultResp
    }
    return defaultResp
}


func printUsage() {
    const usageStr = "lvc - lesser version control\n" +
                     "\n" +
                     "commands:\n" +
                     " - init\n" +
                     " - add\n" +
                     " - rm\n" +
                     " - commit\n" +
                     " - log\n" +
                     "\n"
    fmt.Print(usageStr)
}


func assumeLvcRepo() {
    _, err := findLvcRoot()
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: not a lvc repository")
        os.Exit(1)
    }
}


func commandInit() {
    if flag.NArg() != 0 {
        printUsage()
        fmt.Println("error: init takes no arguments")
        return
    }

    if flag.NFlag() > 0 {
        printUsage()
        fmt.Println("error: init takes no flags")
        return
    }

    if f, err := os.Stat(".lvc"); err == nil {
        if !f.IsDir() {
            fmt.Fprintln(os.Stderr, "error: '.lvc' appears to be a file, this installation may be corrupt")
            return
        }

        fmt.Fprintln(os.Stderr, "error: this directory is already tracked by lvc")
        return
    }

    initialize()
}



func commandAdd() {
    assumeLvcRepo()

    if flag.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "error: add takes at minimum one argument")
        return
    }

    files := make([]string, 0)

    for _, f := range flag.Args() {
        globs, err := filepath.Glob(f)
        if err != nil {
            //TODO: check why glob can fail
        }
        if globs == nil {
            continue
        }
        for _, g := range globs {
            files = append(files, g)
        }
    }

    stageFiles(files)
}



func commandCommit() {
    assumeLvcRepo()

    if flag.NArg() != 1 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: commit only takes the form 'commit \"msg\"")
        return
    }

    commitStage(flag.Args()[0], "thebirk <totally@fake.mail>")
}



func commandStatus() {
    assumeLvcRepo()

    fmt.Println("Current branch: " + getBranchFromHead().name)
    fmt.Println()

    stagedFiles := readStageFile()
    if len(stagedFiles) > 0 {
        fmt.Println("Staged files:")
        for _, f := range stagedFiles {
            fmt.Println("    " + f)
        }
    } else {
        fmt.Println("No staged files")
    }

    fmt.Println()

    modifiedFiles := getModifiedFiles()
    if len(modifiedFiles) > 0 {
        fmt.Println("Unstaged Modified files:")
        for _, f := range modifiedFiles {
            fmt.Println("    " + f)
        }
    }
}



func commandLog() {
    assumeLvcRepo()
    // make sure .lvc etc.

    var commit Commit
    
    if flag.NArg() >= 1 {
        // if arg is a valid id, check if that exists
        // otherwise check if it is a branch
        arg := flag.Args()[0]
        if len(arg) == 64 {
            // This looks like an id
            stringID, err := hex.DecodeString(arg)
            if err != nil {
                fmt.Fprintln(os.Stderr, "error: invalid commit '" + arg + "'")
                return
            }

            id := ID{}
            copy(id[:], stringID)

            commit = getCommitWithoutFiles(id)
        } else {
            // cant be an idea, assume branch

            foundBranch := false
            branches := getAllBranches()
            for _, b := range branches {
                if b.name == arg {
                    commit = getCommitWithoutFiles(b.id)
                    foundBranch = true
                }
            }

            if !foundBranch {
                fmt.Fprintln(os.Stderr, "error: unknown branch '" + arg + "'")
                return
            }
        }
    } else {
        commit = getHead()
    }

    cmd, in := startPager()

    for {
        if commit.parent == zeroID {
            break
        }

        fmt.Fprintln(in, hex.EncodeToString(commit.id[:]))
        fmt.Fprintln(in, "date: " + commit.timestamp.Local().String())
        fmt.Fprintln(in, "author: " + commit.author)
        fmt.Fprintln(in, "message: " + commit.message)
        fmt.Fprintln(in, )

        commit = getCommitWithoutFiles(commit.parent)
    }

    endPager(cmd, in)
}


func commandBranch() {
    assumeLvcRepo()
    if flag.NArg() == 0 {
        branches := getAllBranches()
        current := getBranchFromHead()
        for _, b := range branches {
            if b.name == current.name {
                fmt.Print("*")
            } else {
                fmt.Print(" ")
            }
    
            fmt.Println(b.name)
        }
    } else {
        //TODO: assume only one extra arg for now, handle this later
        createNewBranchFromHead(flag.Arg(0))
    }
}


func commandTag() {
    assumeLvcRepo()

    if flag.NArg() != 1 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: usage: tag <tag-name>")
        return
    }

    tagName := flag.Arg(0)

    createTagAtHead(tagName)
}


func commandTags() {
    assumeLvcRepo()

    if flag.NArg() != 0 || flag.NFlag() != 0 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: command 'tags' takes no arguments")
        return
    }

    tags := getAllTags()
    for _, t := range tags {
        fmt.Println(t.name, hex.EncodeToString(t.id[:]))
    }
}


func commandCheckout() {
    assumeLvcRepo()

    if flag.NArg() != 1 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: usage: checkout <branch>")
        return
    }

    branchName := flag.Arg(0)
    checkoutBranch(branchName)
}


func commandDiff() {
    assumeLvcRepo()

    if flag.NArg() != 0 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: usage: diff")
        return
    }

    diffWorkingWith(getHeadID())
}


func commandGraph() {
    assumeLvcRepo()

    f, err := os.Create("lvc.dot")
    if err != nil {
        panic(err)
    }

    f.WriteString("digraph lvc {\nrankdir=\"TB\";\n")

    root, _ := findLvcRoot()
    filepath.Walk(root + "/.lvc/commits/", func(path string, info os.FileInfo, err error) error {
        if info.IsDir() {
            return nil
        }
        idBytes, _ := hex.DecodeString(info.Name())
        id := ID{}
        copy(id[:], idBytes)
        commit := getCommit(id)

        if commit.parent == zeroID {
            return nil
        }

        f.WriteString(fmt.Sprintf(
            "commit_%s [label=\"%s\"]\n",
            hex.EncodeToString(id[:]),
            commit.message,
        ))

        parentsParent := getCommit(commit.parent)
        if parentsParent.parent == zeroID {
            return nil   
        }

        f.WriteString(fmt.Sprintf(
            "commit_%s -> commit_%s [label=\"%s\"]\n",
            hex.EncodeToString(commit.parent[:]),
            hex.EncodeToString(id[:]),
            "", //commit.message,
        ))

        return nil
    })

    filepath.Walk(root + "/.lvc/branches/", func(path string, info os.FileInfo, err error) error {
        if info.IsDir() {
            return nil
        }

        id := getBranchID(info.Name())
        
        f.WriteString(fmt.Sprintf(
            "\"%s\" [shape=box]\n",
            info.Name(),
        ))

        f.WriteString(fmt.Sprintf(
            "{rank=same; \"%s\" -> commit_%s}\n",
            info.Name(),
            hex.EncodeToString(id[:]),
        ))

        return nil
    })

    head := getBranchFromHead()
    f.WriteString("HEAD [shape=box, color=red]\n")
    f.WriteString(fmt.Sprintf(
        "HEAD -> \"%s\"\n",
        head.name,
    ))


    f.WriteString("}\n")
    f.Close()
}


func commandInfo() {
    assumeLvcRepo()

    cmd, in := startPager()

    root, _ := findLvcRoot()
    fmt.Fprintln(in, "Root directory:    " + root)

    head := getHead()

    firstCommit := getFirstCommit(head.id)
    fmt.Fprintln(in, "First commit date: " + firstCommit.timestamp.Local().String())

    fmt.Fprintln(in, "Last  commit date: " + head.timestamp.Local().String())

    fmt.Fprintf(in, "Most recent commit message:\n    %s\n", head.message)
    fmt.Fprintln(in)

    fmt.Fprintf(in, "Number of currently tracked files: %d\n", len(head.files))
 
    { // ALl branches and their commit count.
        allBranches := getAllBranches()
        currentBranch := getBranchFromHead()

        maxBranchNameWidth := 0
        maxCommitsWidth := 0
        for _, b := range allBranches {
            if len(b.name) > maxBranchNameWidth {
                maxBranchNameWidth = len(b.name)
            }

            commits := countCommitsInBranch(b.name)
            digits := 0
            for commits > 0 {
                digits++
                commits /= 10
            }

            if digits > maxCommitsWidth {
                maxCommitsWidth = digits
            }
        }

        fmt.Fprintf(in, "Branches: (name, total commits)\n")
        for _, b := range allBranches {
            commits := countCommitsInBranch(b.name)
            if idsAreEqual(currentBranch.id, b.id) {
                fmt.Fprintf(in, "    *")
            } else {
                fmt.Fprintf(in, "     ")
            }

            fmt.Fprintf(in, "%-*s - %*d\n", maxBranchNameWidth, b.name, maxCommitsWidth, commits)
        }
    }

    fmt.Fprintln(in)


    endPager(cmd, in)
}


func main() {
    if len(os.Args) <= 1 {
        printUsage()
        return
    }
    //os.RemoveAll(".lvc")

    //IDEA: Have some argument like '--root=path' and plop that shit into _lvcRoot

    userRoot := ""
    flag.StringVar(&userRoot, "root", "", "Operate on a directory outside of the current repository.")

    flag.CommandLine.Parse(os.Args[2:])

    if userRoot != "" {
        //TODO: Check if actually root
        _lvcRoot, _ = filepath.Abs(userRoot)
    }

    //TODO: Commands:
    // - untrack
    // - merge
    // - list : list all currently tracked files
    // - info : some info and stats about the repo, number of files, root dir, creation date ,last commit date, total commits in active branch, active branch

    switch os.Args[1] {
    case "init":
        commandInit()
    case "add":
        commandAdd()
    case "status":
        commandStatus()
    case "commit":
        commandCommit()
    case "remove":
        panic("//TODO remove")
    case "log":
        commandLog()
    case "branch":
        commandBranch()
    case "tag":
        commandTag()
    case "tags":
        commandTags()
    case "checkout":
        commandCheckout()
    case "diff":
        commandDiff()
    case "graph":
        commandGraph()
    case "info":
        commandInfo()
    default:
        printUsage()
        return
    }
}
