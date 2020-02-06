package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// HEAD -> id of current commit
// COMMIT -> list of ids of files and their blob data
// BLOBS -> data

// Could allow blobs for commit messages, identify by prefixng the message with "blob:".
// make sure to disallow "blob:" in short commit message for this to work

// Make stage more stage like, aka make a blob when you stage a file
// store the hash of that in the stage file and use that in for example
// lvc diff so that you can see when a staged file has changes.
// Also useful so that you dont accidentally change files after staging
// them and having those changes come with the commit.

// how are we going to set author? per commit?


// commit format:
//  commitid ; id of parent commit
//  commitmsg ; commit message
//  author ; cand be anything, probably something like "thebirk <pingnor@gmail.com>"
//  timestamp ; utc+0 timestamp of commit
//  filename<space>blobid ; one entry for each tracked file

// Create a new file object every time a files is changed

// lvc init
// lvc add <file>
// lvc commit "Message"
// lvc rm stop tracking file, not actually remove it because that would be silly

func printUsage() {
    const usageStr = "lvc - lesser version control\n" +
        "\n" +
        "commands:\n" +
        " - init\n" +
        " - add\n" +
        " - rm\n" +
        " - commit\n" +
        "\n"
    fmt.Print(usageStr)
}


// ID representing any object
type ID [32]byte
var zeroID = ID([32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})


func createBlob(data []byte) ID {
    id := ID(sha256.Sum256(data))
    name := hex.EncodeToString(id[:])

    if err := ioutil.WriteFile(name, data, 0644); err != nil {
        panic(err)
    }

    return id
}



func createEmptyFile(path string) {
    head, err := os.Create(".lvc/head")
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: failed to create .lvc/head")
        fmt.Fprintln(os.Stderr, err)
    }
    head.Close()
}


func createDirectory(path string) bool {
    if err := os.Mkdir(path, 0777); err != nil {
        fmt.Fprintln(os.Stderr, "error: failed to create " + path)
        fmt.Fprintln(os.Stderr, err)
        return false
    }
    return true
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

    createDirectory(".lvc")
    createDirectory(".lvc/commits")
    createDirectory(".lvc/blobs")


    // create the baseline commit
    baseCommit := []byte("\n\n\n"+time.Now().Format(time.RFC3339)+"\n")
    commitID := ID(sha256.Sum256([]byte(baseCommit)))
    ioutil.WriteFile(".lvc/commits/" + hex.EncodeToString(commitID[:]), baseCommit, 0644)

    // point head to bare commit
    ioutil.WriteFile(".lvc/head", []byte(hex.EncodeToString(commitID[:]) + "\n"), 0644)

    stage, err := os.Create(".lvc/stage")
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: failed to create .lvc/head")
        fmt.Fprintln(os.Stderr, err)
        // clear
        return
    }
    stage.Close()

    fmt.Println("lvc now tracks this directory!")
}

func readStageFile() []string {
    stageReader, err := os.Open(".lvc/stage")
    if err != nil {
        //TODO: Handle
        panic(err)
    }
    defer stageReader.Close()

    files := make([]string, 0)
    scanner := bufio.NewScanner(stageReader)
    for scanner.Scan() {
        files = append(files, scanner.Text())
    }

    return files
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


func commandStatus() {
    //TODO: ensure .lvc


    fmt.Println("Staged files:")
    files := readStageFile()
    for _, f := range files {
        fmt.Println("   " + f)
    }
}


func getFileHash(path string) []byte {
    f, err := os.Open(path)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        panic(err)
    }

    return h.Sum(nil)
}

// CommitFile represents a file with its name and its ID
type CommitFile struct {
    name string
    id ID
}


func getFilesFromCommit(id ID) []CommitFile {
    reader, err := os.Open(".lvc/commits/" + hex.EncodeToString(id[:]))
    if err != nil {
        //TODO: Handle
        panic(err)
    }
    defer reader.Close()

    files := make([]CommitFile, 0)

    skipLines := 4
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        if skipLines > 0 {
            skipLines--
            scanner.Text()
            continue
        }

        line := strings.Split(scanner.Text(), " ")
        sid, err := hex.DecodeString(line[1])
        if err != nil {
            panic(err)
        }
        id := ID{}
        copy(id[:], sid)
        files = append(files, CommitFile{
            name: line[0],
            id: id,
        })
    }

    return files
}


func getFilesFromHead() []CommitFile {
    return getFilesFromCommit(getHeadID())
}


func getHeadID() ID {
    head, err := os.Open(".lvc/head")
    if err != nil {
        panic(err)
    }
    defer head.Close()
    headScanner := bufio.NewScanner(head)
    headScanner.Scan()
    strID := headScanner.Text()
    sliceID, err := hex.DecodeString(strID)

    id := ID{}
    copy(id[:], sliceID[:])

    return id
}


func createBlobForFileWithID(path string, id ID) {
    //TODO: ensure it does not already exist
    data, err := ioutil.ReadFile(path)
    if err != nil {
        panic(err)
    }
    ioutil.WriteFile(".lvc/blobs/" + hex.EncodeToString(id[:]), data, 0644)
}


func commandCommit() {
    //TODO: ensure .lvc etc.
    if flag.NArg() != 2 {
        printUsage()
        fmt.Fprintln(os.Stderr, "error: commit only takes the form 'commit \"msg\"")
        return
    }

    commit := make([]CommitFile, 0)

    headFiles := getFilesFromHead()
    stageFiles := readStageFile()

    fmt.Println(headFiles)

    headFilesLoop:
    for _, hf := range headFiles {
        for _, sf := range stageFiles {
            if sf == hf.name {
                continue headFilesLoop
            }
        }
        commit = append(commit, hf)
    }

    stageFileLoop:
    for _, f := range stageFiles {
        // If file is new, commit anyways
        // If the files is not new, checked if the hash differ, if so commit it
        for _, hf := range headFiles {
            if hf.name == f {
                // File is not new
                hash := getFileHash(f)
                
                if bytes.Equal(hash, hf.id[:]) {
                    commit = append(commit, hf)
                } else {
                    id := ID{}
                    copy(id[:], hash)
                    commit = append(commit, CommitFile{
                        name: f,
                        id: id,
                    })
                    createBlobForFileWithID(f, id)
                }

                continue stageFileLoop
            }
        }

        hash := getFileHash(f)
        id := ID{}
        copy(id[:], hash)
        commit = append(commit, CommitFile{
            name: f,
            id: id,
        })
        createBlobForFileWithID(f, id)
    }

    headid := getHeadID()

    builder := strings.Builder{}
    builder.WriteString(hex.EncodeToString(headid[:]) + "\n")
    builder.WriteString(flag.Args()[1] + "\n")
    builder.WriteString("thebirk <pingnor@gmail.com>\n")
    builder.WriteString(time.Now().Format(time.RFC3339) + "\n")
    for _, c := range commit {
        builder.WriteString(c.name + " " + hex.EncodeToString(c.id[:]) + "\n")
    }

    final := builder.String()
    id := ID(sha256.Sum256([]byte(final)))

    // write commit to file
    ioutil.WriteFile(".lvc/commits/" + hex.EncodeToString(id[:]), []byte(final), 0644)

    // clear stage file
    if err := os.Truncate(".lvc/stage", 0); err != nil {
        panic(err)
    }

    // update HEAD
    head, err := os.OpenFile(".lvc/head", os.O_TRUNC|os.O_WRONLY, 0644)
    if err != nil {
        panic(err)
    }
    head.WriteString(hex.EncodeToString(id[:]))
    head.Close()
}

// Commit represents a single commit
type Commit struct {
    id        ID
    parent    ID
    message   string
    author    string
    timestamp time.Time
    files     []CommitFile
}

func getCommitWithoutFiles(id ID) Commit {
    reader, err := os.Open(".lvc/commits/" + hex.EncodeToString(id[:]))
    if err != nil {
        //TODO: Handle
        panic(err)
    }
    defer reader.Close()

    commit := Commit{}
    commit.id = id

    scanner := bufio.NewScanner(reader)

    // parent
    scanner.Scan()
    parentID, err := hex.DecodeString(scanner.Text())
    if err != nil {
        panic(err)
    }
    copy(commit.parent[:], parentID)

    scanner.Scan()
    commit.message = scanner.Text()

    scanner.Scan()
    commit.author = scanner.Text()

    scanner.Scan()
    commit.timestamp, err = time.Parse(time.RFC3339, scanner.Text())
    if err != nil {
        panic(err)
    }

    return commit
}

func getCommit(id ID) Commit {
    reader, err := os.Open(".lvc/commits/" + hex.EncodeToString(id[:]))
    if err != nil {
        //TODO: Handle
        panic(err)
    }
    defer reader.Close()

    commit := Commit{}
    commit.id = id

    scanner := bufio.NewScanner(reader)

    // parent
    scanner.Scan()
    parentID, err := hex.DecodeString(scanner.Text())
    if err != nil {
        panic(err)
    }
    copy(commit.parent[:], parentID)

    scanner.Scan()
    commit.message = scanner.Text()

    scanner.Scan()
    commit.author = scanner.Text()

    scanner.Scan()
    commit.timestamp, err = time.Parse(time.RFC3339, scanner.Text())
    if err != nil {
        panic(err)
    }

    commit.files = make([]CommitFile, 0)
    
    for scanner.Scan() {
        line := strings.Split(scanner.Text(), " ")
        sid, err := hex.DecodeString(line[1])
        if err != nil {
            panic(err)
        }
        id := ID{}
        copy(id[:], sid)
        commit.files = append(commit.files, CommitFile{
            name: line[0],
            id: id,
        })
    }

    return commit
}


func getHead() Commit {
    id := getHeadID()
    return getCommit(id)
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


func main() {
    if len(os.Args) <= 1 {
        printUsage()
        return
    }
    //os.RemoveAll(".lvc")

    flag.Parse()

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
    default:
        printUsage()
        return
    }
}
