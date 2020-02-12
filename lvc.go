package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// HEAD -> id of current commit
// COMMIT -> list of ids of files and their blob data
// BLOBS -> data

// Could allow blobs for commit messages, identify by prefixng the message with "blob:".
// make sure to disallow "blob:" in short commit message for this to work

// TODO: Make stage more stage like, aka make a blob when you stage a file
// store the hash of that in the stage file and use that in for example
// lvc diff so that you can see when a staged file has changes.
// Also useful so that you dont accidentally change files after staging
// them and having those changes come with the commit.

// how are we going to set author? per commit?

// TODO: Switch over to some other terminology for commands
//       untrack instead of rm
//       stage instead of add
//       

// TODO: Store files in Commit as map with filename as key
// FIXME: Checkout removes untracked files, this is not what we want

// TODO: For mergin, allow commits to have multiple parents
//       How do we handle merge conflicts? `lvc merge`, edits, then `lvc merge` again?

// TODO: Have the possibliy to checkout tags and commits and land in a state like detached-HEAD in git,
// but instead of allowing commit, require a branch first.
// ex.
//   lvc checkout <commitid>           ;; "detached HEAD", commits are disallowed
//   lvc branch new-branch-at-commit   ;; create a new branch, currently at <commitid>
//   lvc checkout new-branch-at-commit ;; checkout the new branch, commits are allowed again

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


// TODO:
//   - Make the function that checks if we are in a lvc repo
//   - Make all .lvc paths relative so we can call from anywhere
//   - User config in ~/.config/lvc/config
//     - If env LVC_HOME is set => $LVC_HOME/config
//     - Option for default author


// ID representing any object
type ID [32]byte
var zeroID = ID([32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})


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
    id   ID
}

// Branch represents a branch and its current commit id
type Branch struct {
    name string
    id   ID
}

// Tag represens a tag and its commit id
type Tag struct {
    name string
    id   ID
}


////////////////////////////////////////////////////////////////////////////////////////////////////


var _lvcRoot = ""
var errNotARepo = errors.New("not a repository")
func findLvcRoot() (string, error) {
    if _lvcRoot != "" {
        return _lvcRoot, nil
    }

    errFoundPath := errors.New("found root path")
    rootPath := ""

    //FIXME: Seems to fail when called inside '.lvc'
    wd, _ := os.Getwd()
    err := walkUp(wd, func(pathUp string, info os.FileInfo) error {
        return filepath.Walk(pathUp, func(path string, info os.FileInfo, err error) error {
            if pathUp == path {
                return nil
            }
            if info.IsDir() {
                if info.Name() == ".lvc" {
                    rootPath = filepath.Dir(path)
                    return errFoundPath
                }
                return filepath.SkipDir
            }
            return nil
        })
    })
    
    if err == errFoundPath {
        _lvcRoot = rootPath
        return rootPath, nil
    } else if err == nil {
        return "", errNotARepo
    }
    return "", err
}


func createBlob(data []byte) ID {
    id := ID(sha256.Sum256(data))
    name := hex.EncodeToString(id[:])

    //TODO: Check if blob already exists, if so, we can skip writing it again

    if err := ioutil.WriteFile(name, data, 0644); err != nil {
        panic(err)
    }

    return id
}


func pathIsValidRepo(path string) bool {
    dir, err := os.Open(path);
    if err != nil {
        return false
    }

    files, err := dir.Readdir(0)
    if err != nil {
        return false
    }

    for _, f := range files {
        if f.IsDir() && f.Name() == ".lvc" {
            return true
        }
    }

    return false
}


func pathIsChildOfRoot(path string) bool {
    root, _ := findLvcRoot()
    abs, err := filepath.Abs(path)
    if err != nil {
        return false
    }

    rel, err := filepath.Rel(root, abs)
    if err != nil {
        return false
    }

    return !strings.HasPrefix(rel, "..")
}


func stageFiles(files []string) {
    root, _ := findLvcRoot()

    sw, err := os.OpenFile(filepath.Join(root, ".lvc", "stage"), os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        //TODO
        panic(err)
    }

    stagedFiles := readStageFile()

    file_loop:
    for _, f := range files {
        if !pathIsChildOfRoot(f) {
            fmt.Fprintln(os.Stderr, "error: '" + f + "' is outside the repository")
            continue
        }
        info, err := os.Stat(f);
        if os.IsNotExist(err) {
            fmt.Fprintln(os.Stderr, "error: '" + f + "' does not exist")
            continue
        } else if err != nil {
            panic(err)
        }

        if info.IsDir() {
            fmt.Fprintln(os.Stderr, "error: cannot stage directory "  + f)
            continue
        }

        //TODO: Check if 'f' is inside OUR .lvc, if so ignore it

        for _, sf := range stagedFiles {
            if pathsAreEqual(sf, f) {
                continue file_loop
            }
        }

        abs, err := filepath.Abs(f)
        if err != nil {
            panic(err)
        }
        rel, err := filepath.Rel(root, abs)
        if err != nil {
            panic(err)
        }
        sw.WriteString(rel + "\n")

        fmt.Println("Staged " + rel + "")
    }
}


func readStageFile() []string {
    root, _ := findLvcRoot()
    stageReader, err := os.Open(filepath.Join(root, ".lvc", "stage"))
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


func getModifiedFiles() []string {
    trackedFiles := getHead()

    files := make([]string, 0)

    root, err := findLvcRoot()
    if err != nil {
        panic(err)
    }

    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        abs := path
        path, err = filepath.Rel(root, path)
        if err != nil {
            panic(err)
        }
        if info.IsDir() && info.Name() == ".lvc" {
            return filepath.SkipDir
        }

        for _, tf := range trackedFiles.files {
            if pathsAreEqual(tf.name, path) {
                currentHash := getFileHash(abs)

                if !idsAreEqual(currentHash, tf.id) {
                    files = append(files, tf.name)
                    return nil
                }
            }
        }
        
        return nil
    })

    return files
}


func getFileHash(path string) ID {
    f, err := os.Open(path)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        panic(err)
    }

    id := ID{}
    copy(id[:], h.Sum(nil))

    return id
}


func createBlobForFileWithID(path string, id ID) {
    //TODO: ensure it does not already exist
    data, err := ioutil.ReadFile(path)
    if err != nil {
        panic(err)
    }

    root, _ := findLvcRoot()

    ioutil.WriteFile(filepath.Join(root, ".lvc/blobs/", hex.EncodeToString(id[:])), data, 0644)
}


func getCommitWithoutFiles(id ID) Commit {
    root, _ := findLvcRoot()
    reader, err := os.Open(filepath.Join(root, ".lvc/commits/", hex.EncodeToString(id[:])))
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
    root, _ := findLvcRoot()
    reader, err := os.Open(filepath.Join(root, ".lvc/commits/", hex.EncodeToString(id[:])))
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
        line := strings.SplitN(scanner.Text(), " ", 2)
        sid, err := hex.DecodeString(line[0])
        if err != nil {
            panic(err)
        }
        id := ID{}
        copy(id[:], sid)
        commit.files = append(commit.files, CommitFile{
            name: line[1],
            id: id,
        })
    }

    return commit
}


func getHeadID() ID {
    root, _ := findLvcRoot()
    headBytes, err := ioutil.ReadFile(filepath.Join(root, ".lvc/head"))
    if err != nil {
        panic(err)
    }
    // Chop of newline
    head := string(headBytes[:len(headBytes)-1])

    if _, err := os.Stat(filepath.Join(root, ".lvc/branches/", head)); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + head + "'")
        os.Exit(1)
    }

    branchBytes, err := ioutil.ReadFile(filepath.Join(root, ".lvc/branches/", head))
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
    root, _ := findLvcRoot()
    if _, err := os.Open(filepath.Join(root, ".lvc/branches/", name)); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + name + "'")
        os.Exit(1)
    }

    branchBytes, err := ioutil.ReadFile(filepath.Join(root, ".lvc/branches/", name))
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
    root, _ := findLvcRoot()
    headBytes, err := ioutil.ReadFile(filepath.Join(root, ".lvc/head"))
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
    root, _ := findLvcRoot()
    if _, err := os.Open(filepath.Join(root, ".lvc/branches/", name)); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown branch '" + name + "'")
        os.Exit(1)
    }

    // WriteFile truncates
    err := ioutil.WriteFile(filepath.Join(root, ".lvc/branches/", name), []byte(hex.EncodeToString(id[:]) + "\n"), 0644)
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
    root, _ := findLvcRoot()
    if err := os.Truncate(filepath.Join(root, ".lvc/stage"), 0); err != nil {
        panic(err)
    }
}


func createNewBranchFromHead(name string) {
    branches := getAllBranches()
    for _, b := range branches {
        if b.name == name {
            fmt.Fprintln(os.Stderr, "error: branch '" + name + "' already exists")
            os.Exit(1)
        }
    }

    root, _ := findLvcRoot()

    id := getHeadID()
    err := ioutil.WriteFile(filepath.Join(root, ".lvc/branches/", name), []byte(hex.EncodeToString(id[:]) + "\n"), 0644)
    if err != nil {
        panic(err)
    }
}


func getAllBranches() []Branch {
    result := make([]Branch, 0)
    root, _ := findLvcRoot()

    fileinfos, err := ioutil.ReadDir(filepath.Join(root, ".lvc/branches"))
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


func createTagAtHead(name string) {
    root, _ := findLvcRoot()

    if _, err := os.Open(filepath.Join(root, ".lvc/tags/", name)); !os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: tag '" + name + "' already exists")
        os.Exit(1)
    }
    headID := getHeadID()

    ioutil.WriteFile(filepath.Join(root, ".lvc/tags/", name), []byte(hex.EncodeToString(headID[:]) + "\n"), 0644)
}


func getTagID(name string) ID {
    root, _ := findLvcRoot()
    if _, err := os.Open(filepath.Join(root, ".lvc/tags/", name)); os.IsNotExist(err) {
        fmt.Fprintln(os.Stderr, "error: unknown tag '" + name + "'")
        os.Exit(1)
    }

    tagBytes, err := ioutil.ReadFile(filepath.Join(root, ".lvc/tags/", name))
    if err != nil {
        panic(err)
    }
    // Chop of newline
    tagIDString := string(tagBytes[:len(tagBytes)-1])

    tagIDBytes, err := hex.DecodeString(tagIDString)
    if err != nil {
        panic(err)
    }

    id := ID{}
    copy(id[:], tagIDBytes)

    return id
}


func getAllTags() []Tag {
    tags := make([]Tag, 0)

    root, _ := findLvcRoot()

    fileinfos, err := ioutil.ReadDir(filepath.Join(root, ".lvc/tags"))
    for _, fi := range fileinfos {
        name := fi.Name()
        tags = append(tags, Tag{
            name: name,
            id: getTagID(name),
        })
    }
    if err != nil {
        panic(err)
    }
    
    return tags
}


func getFirstCommit(startCommit ID) Commit {
    commit := getCommit(startCommit)
    for commit.parent != zeroID {
        commit = getCommit(commit.parent)
    }
    return commit
}


func countCommitsInBranch(branch string) int {
    count := 0

    b := getBranch(branch)
    for b.parent != zeroID {
        b = getCommit(b.parent)
        count++
    }

    return count
}


func idsAreEqual(a ID, b ID) bool {
    return bytes.Equal(a[:], b[:])
}


func setHead(branch string) {
    // This will check if the branch exists
    getBranch(branch)
    root, _ := findLvcRoot()
    ioutil.WriteFile(filepath.Join(root, ".lvc/head"), []byte(branch + "\n"), 0644)
}


// Converts paths to the same seprator and directly compares the strings
func pathsAreEqual(a, b string) bool {
    return filepath.ToSlash(a) == filepath.ToSlash(b)
}


func checkoutBranch(name string) {
    head := getHead()
    branch := getBranch(name)
    
    root, _ := findLvcRoot()

    // make sure the user is aware that their files will be overwritten
    for _, f := range head.files {
        currentID := getFileHash(filepath.Join(root, f.name))

        if !idsAreEqual(f.id, currentID) {
            if !yesno(fmt.Sprintf("Contents of file '%s' has changed since last commit, checking out this branch will OVERWRITE it, Are you sure you want to proceed?", f.name), false) {
                fmt.Println("Stopping checkout due to user input.")
                os.Exit(0)
            }
        }
    }
    
    //TODO: If a directory is empty after checkout, remove it, we dont need it

    filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if path == ".lvc" {
            return filepath.SkipDir
        }
        
        if info.IsDir() {
            return nil
        }

        found := false
        for _, bf := range branch.files {
            if pathsAreEqual(bf.name, path) {
                found = true
                break
            }
        }

        if !found {
            os.Remove(path)
        }

        return nil
    })
    

    for _, bf := range branch.files {
        blobPath := filepath.Join(root, ".lvc/blobs/", hex.EncodeToString(bf.id[:]))
        copyFile(blobPath, bf.name)
    }

    // set head to current branch
    setHead(name)
}


func diffWorkingWith(id ID) {
    root, err := findLvcRoot()
    if err != nil {
        panic(err)
    }

    commit := getCommit(id)

    //TODO: Handle files that are present in the commit, but missing in working
    dmp := diffmatchpatch.New()

    cmd, pagerIn := startPager()
    
    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() && info.Name() == ".lvc" {
            return filepath.SkipDir
        }
        abs := path
        path, _ = filepath.Rel(root, path)

        for _, cf := range commit.files {
            if pathsAreEqual(path, cf.name) {
                // files is tracked
                id := getFileHash(abs)
                if !idsAreEqual(id, cf.id) {
                    // start := time.Now()
                    
                    if(true) {
                        workingFile, _ := ioutil.ReadFile(abs)
                        commitFile, _ := ioutil.ReadFile(filepath.Join(root, "/.lvc/blobs/", hex.EncodeToString(cf.id[:])))

                        a, b, arr := dmp.DiffLinesToChars(string(commitFile), string(workingFile))
                        diff := dmp.DiffMain(a, b, false)
                        diff = dmp.DiffCharsToLines(diff, arr)
                        //diff = dmp.DiffCleanupSemantic(diff)
                        
                        // Show only the 4 first and last lines of and equals
                        // if its longer than 8 or so lines maybe
                        totalInserts := 0
                        totalDeletions := 0

                        for _, d := range diff {
                            switch d.Type {
                            case diffmatchpatch.DiffInsert:
                                totalInserts += countLines(d.Text)
                            case diffmatchpatch.DiffDelete:
                                totalDeletions += countLines(d.Text)
                            }
                        }

                        fmt.Fprintf(pagerIn, "%s - %d inserts(+), %d deletions(-)\n", path, totalInserts, totalDeletions)

                        offset := 0
                        for i, d := range diff {
                            switch d.Type {
                            case diffmatchpatch.DiffEqual:
                                first, _ := getFirstLines(d.Text, 3)
                                last, _ := getLastLines(d.Text, 3)
                                
                                if i-1 >= 0 {
                                    printTextWithPrefixSuffix(pagerIn, first, " ", "")
                                }
                                // TODO: This isnt quite right, but usable for now
                                line, _ := getLineAndOffsetInString(string(commitFile), offset+len(d.Text))
                                fmt.Fprintf(pagerIn, "@ %s - %d\n", path, line)
                                printTextWithPrefixSuffix(pagerIn, last, " ", "")
                            case diffmatchpatch.DiffInsert:
                                printTextWithPrefixSuffix(pagerIn, d.Text, "\033[32m+", "\033[0m")
                            case diffmatchpatch.DiffDelete:
                                printTextWithPrefixSuffix(pagerIn, d.Text, "\033[31m-", "\033[0m")
                            }
                            offset += len(d.Text)
                        }

                        fmt.Fprintln(pagerIn)
                    } else {
                        cmd := exec.Command("diff", "-u", filepath.Join(root, "/.lvc/blobs/", hex.EncodeToString(cf.id[:]), path))
                        cmd.Stdout = os.Stdout
                        cmd.Run()
                    }

                }
            }
        }
                
        return nil
    })

    endPager(cmd, pagerIn)
}


//////////////////////////////////////////////////////////////////////////////////////////////////////////


func initialize() {
    createDirectory(".lvc")
    hideFile(".lvc")
    createDirectory(".lvc/commits")
    createDirectory(".lvc/blobs")
    createDirectory(".lvc/branches")
    createDirectory(".lvc/tags")


    // create the baseline commit
    baseCommit := []byte("\n\n\n"+time.Now().Format(time.RFC3339)+"\n")
    commitID := ID(sha256.Sum256([]byte(baseCommit)))
    ioutil.WriteFile(filepath.Join(".lvc/commits/", hex.EncodeToString(commitID[:])), baseCommit, 0644)

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

    root, _ := findLvcRoot()

    head := getHead()
    stageFiles := readStageFile()

    headFilesLoop:
    for _, hf := range head.files {
        for _, sf := range stageFiles {
            if pathsAreEqual(sf, hf.name) {
                continue headFilesLoop
            }
        }
        commit = append(commit, hf)
    }

    filesChanged := 0
    filesCreated := 0

    stageFileLoop:
    for _, f := range stageFiles {
        // If file is new, commit anyways
        // If the files is not new, checked if the hash differ, if so commit it
        for _, hf := range head.files {
            if pathsAreEqual(hf.name, f) {
                // File is not new
                hash := getFileHash(filepath.Join(root, f))
                
                if bytes.Equal(hash[:], hf.id[:]) {
                    commit = append(commit, hf)
                } else {
                    commit = append(commit, CommitFile{
                        name: f,
                        id: hash,
                    })
                    createBlobForFileWithID(f, hash)
                    filesChanged++
                }

                continue stageFileLoop
            }
        }

        id := getFileHash(filepath.Join(root, f))
        commit = append(commit, CommitFile{
            name: f,
            id: id,
        })
        createBlobForFileWithID(filepath.Join(root, f), id)
        filesCreated++
    }

    headid := getHeadID()

    builder := strings.Builder{}
    builder.WriteString(hex.EncodeToString(headid[:]) + "\n")
    builder.WriteString(msg + "\n")
    builder.WriteString(author + "\n")
    builder.WriteString(time.Now().Format(time.RFC3339) + "\n")
    for _, c := range commit {
        builder.WriteString(hex.EncodeToString(c.id[:]) + " " + c.name + "\n")
    }

    final := builder.String()
    id := ID(sha256.Sum256([]byte(final)))

    // write commit to file
    ioutil.WriteFile(filepath.Join(".lvc/commits/", hex.EncodeToString(id[:])), []byte(final), 0644)

    clearStage()

    updateHead(id)

    fmt.Printf("%s\n%d file(s) changes. %d file(s) created\n", hex.EncodeToString(id[:]), filesChanged, filesCreated)
}
