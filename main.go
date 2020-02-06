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
	"path/filepath"
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


// Commit represents a single commit
type Commit struct {
    id        ID
    parent    ID
    message   string
    author    string
    timestamp time.Time
    files     []CommitFile
}


// CommitFile represents a file with its name and its ID
type CommitFile struct {
    name string
    id ID
}

// Branch represents a branch and its current commit id
type Branch struct {
    name string
    id   ID
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
        fmt.Fprintln(os.Stderr, "error: failed to create file '" + path + "'")
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


func createBlobForFileWithID(path string, id ID) {
    //TODO: ensure it does not already exist
    data, err := ioutil.ReadFile(path)
    if err != nil {
        panic(err)
    }
    ioutil.WriteFile(".lvc/blobs/" + hex.EncodeToString(id[:]), data, 0644)
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


func getFilesFromHead() []CommitFile {
    return getFilesFromCommit(getHeadID())
}


func getHeadID() ID {
    headBytes, err := ioutil.ReadFile(".lvc/head")
    if err != nil {
        panic(err)
    }
    // Chop of newline
    head := string(headBytes[:len(headBytes)-1])

    if _, err := os.Stat(".lvc/branches/" + head); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + head + "'")
        os.Exit(1)
    }

    branchBytes, err := ioutil.ReadFile(".lvc/branches/" + head)
    if err != nil {
        panic(err)
    }
    // Chop of newline at the end
    branch := string(branchBytes[:len(branchBytes)-1])

    sliceID, err := hex.DecodeString(branch)
    if err != nil {
        panic(err)
    }

    id := ID{}
    copy(id[:], sliceID[:])

    return id
}


func getHead() Commit {
    id := getHeadID()
    return getCommit(id)
}


func getBranchID(name string) ID {
    if _, err := os.Open(".lvc/branches/" + name); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + name + "'")
        os.Exit(1)
    }

    branchBytes, err := ioutil.ReadFile(".lvc/branches/" + name)
    if err != nil {
        panic(err)
    }
    // Chop of newline
    branchIDString := string(branchBytes[:len(branchBytes)-1])

    branchIDBytes, err := hex.DecodeString(branchIDString)
    if err != nil {
        panic(err)
    }

    id := ID{}
    copy(id[:], branchIDBytes)

    return id
}


func getBranch(name string) Commit {
    id := getBranchID(name)
    return getCommit(id)
}


func getBranchFromHead() Branch {
    headBytes, err := ioutil.ReadFile(".lvc/head")
    if err != nil {
        panic(err)
    }
    // Chop of newline
    headString := string(headBytes[:len(headBytes)-1])

    return Branch{
        name: headString,
        id: getBranchID(headString),
    }
}


func updateBranch(name string, id ID) {
    if _, err := os.Open(".lvc/branches/" + name); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + name + "'")
        os.Exit(1)
    }

    // WriteFile truncates
    err := ioutil.WriteFile(".lvc/branches/" + name, []byte(hex.EncodeToString(id[:]) + "\n"), 0644)
    if err != nil {
        panic(err)
    }
}


func updateHead(id ID) {
    currentBranch := getBranchFromHead()
    updateBranch(currentBranch.name, id)
}


func clearStage() {
    // clear stage file
    if err := os.Truncate(".lvc/stage", 0); err != nil {
        panic(err)
    }
}


func createNewBranchFromHEAD(name string) {
    branches := getAllBranches()
    for _, b := range branches {
        if b.name == name {
            fmt.Fprintln(os.Stderr, "error: branch '" + name + "' already exists")
            os.Exit(1)
        }
    }

    id := getHeadID()
    err := ioutil.WriteFile(".lvc/branches/" + name, []byte(hex.EncodeToString(id[:]) + "\n"), 0644)
    if err != nil {
        panic(err)
    }
}


func getAllBranches() []Branch {
    result := make([]Branch, 0)

    fileinfos, err := ioutil.ReadDir(".lvc/branches")
    for _, fi := range fileinfos {
        name := fi.Name()
        result = append(result, Branch{
            name: name,
            id: getBranchID(name),
        })
    }
    if err != nil {
        panic(err)
    }

    return result
}


//////////////////////////////////////////////////////////////////////////////////////////////////////////


func initialize() {
    createDirectory(".lvc")
    createDirectory(".lvc/commits")
    createDirectory(".lvc/blobs")
    createDirectory(".lvc/branches")
    createDirectory(".lvc/tags")


    // create the baseline commit
    baseCommit := []byte("\n\n\n"+time.Now().Format(time.RFC3339)+"\n")
    commitID := ID(sha256.Sum256([]byte(baseCommit)))
    ioutil.WriteFile(".lvc/commits/" + hex.EncodeToString(commitID[:]), baseCommit, 0644)

    // point master to bare commit
    ioutil.WriteFile(".lvc/branches/master", []byte(hex.EncodeToString(commitID[:]) + "\n"), 0644)

    // point head to master
    ioutil.WriteFile(".lvc/head", []byte("master\n"), 0644)

    stage, err := os.Create(".lvc/stage")
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: failed to create .lvc/head")
        fmt.Fprintln(os.Stderr, err)
        // clear
        return
    }
    stage.Close()

    abs, _ := filepath.Abs(".lvc")
    fmt.Println("Initilized lvc in " + abs)
}


func commitStage(msg string, author string, ) {
    commit := make([]CommitFile, 0)

    head := getHead()
    stageFiles := readStageFile()

    headFilesLoop:
    for _, hf := range head.files {
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
        for _, hf := range head.files {
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
    builder.WriteString(msg + "\n")
    builder.WriteString(author + "\n")
    builder.WriteString(time.Now().Format(time.RFC3339) + "\n")
    for _, c := range commit {
        builder.WriteString(c.name + " " + hex.EncodeToString(c.id[:]) + "\n")
    }

    final := builder.String()
    id := ID(sha256.Sum256([]byte(final)))

    // write commit to file
    ioutil.WriteFile(".lvc/commits/" + hex.EncodeToString(id[:]), []byte(final), 0644)

    clearStage()

    updateHead(id)
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
    case "branch":
        commandBranch()
    default:
        printUsage()
        return
    }
}
