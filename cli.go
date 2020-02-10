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
    //TODO: call functions that ensure this is a valid lvc dir

    if flag.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "error: add takes at minimum one argument")
        return
    }

    stagedFiles := readStageFile()

    sw, err := os.OpenFile(".lvc/stage", os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        //TODO
        panic(err)
    }

    file_loop:
    for _, file := range flag.Args() {
        info, err := os.Stat(file);
        if os.IsNotExist(err) {
            fmt.Fprintln(os.Stderr, "error: " + file + " does not exist")
            continue
        } else if err != nil {
            panic(err)
        }

        if info.IsDir() {
            fmt.Fprintln(os.Stderr, "error: cannot stage directory "  + file)
        }

        for _, sf := range stagedFiles {
            if sf == file {
                continue file_loop
            }
        }
        sw.WriteString(file + "\n")
    }
}



func commandCommit() {
    assumeLvcRepo()

    //TODO: ensure .lvc etc.
    if flag.NArg() != 1 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: commit only takes the form 'commit \"msg\"")
        return
    }

    commitStage(flag.Args()[0], "thebirk <totally@fake.mail>")
}



func commandStatus() {
    assumeLvcRepo()

    //TODO: ensure .lvc
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

    for {
        if commit.parent == zeroID {
            break
        }

        fmt.Println(hex.EncodeToString(commit.id[:]))
        fmt.Println("date: " + commit.timestamp.Local().String())
        fmt.Println("author: " + commit.author)
        fmt.Println("message: " + commit.message)
        fmt.Println()

        commit = getCommitWithoutFiles(commit.parent)
    }
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
            "commit_%s -> commit_%s [label=\"%s\"]\n",
            hex.EncodeToString(commit.parent[:]),
            hex.EncodeToString(id[:]),
            commit.message,
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


func main() {
    if len(os.Args) <= 1 {
        printUsage()
        return
    }
    //os.RemoveAll(".lvc")

    //IDEA: Have some argument like '--root=path' and plop that shit into _lvcRoot

    flag.CommandLine.Parse(os.Args[2:])

    switch os.Args[1] {
    case "init":
        commandInit()
    case "add":
        commandAdd()
    case "status":
        commandStatus()
    case "commit":
        commandCommit()
    case "rm":
        panic("//TODO")
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
    default:
        printUsage()
        return
    }
}
