package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)


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


func commandInit() {
    if flag.NArg() != 1 {
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
    for _, file := range flag.Args()[1:] {
        if _, err := os.Stat(file); os.IsNotExist(err) {
            fmt.Fprintln(os.Stderr, "error: " + file + " does not exist")
            continue
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
    //TODO: ensure .lvc etc.
    if flag.NArg() != 2 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: commit only takes the form 'commit \"msg\"")
        return
    }

    commitStage(flag.Args()[1], "thebirk <totally@fake.mail>")
}



func commandStatus() {
    //TODO: ensure .lvc


    fmt.Println("Staged files:")
    files := readStageFile()
    for _, f := range files {
        fmt.Println("   " + f)
    }
}



func commandLog() {
    // make sure .lvc etc.

    var commit Commit
    
    if flag.NArg() >= 2 {
        // if arg is a valid id, check if that exists
        // otherwise check if it is a branch
        arg := flag.Args()[1]
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
            panic("//TODO : branches")
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
    if flag.NArg() == 1 {
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
        createNewBranchFromHEAD(flag.Arg(1))
    }
}