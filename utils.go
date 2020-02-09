package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)


type errNotDir struct {
    path string
}

func (e errNotDir) Error() string {
    return "'" + e.path + "' is not a directory"
}

type walkFunc func(file string, info os.FileInfo) error

// Calls walkFunc with directories while walking up the diretory tree
// only stopping when reaching the filesystem root or walkFunc returns filepath.SkipDir
func walkUp(root string, walkFn walkFunc) error {
    path, err := filepath.Abs(root)
    if err != nil {
        return err
    }

    for {
        stat, err := os.Stat(path)
        if err != nil {
            return err
        }
    
        if !stat.IsDir() {
            return errNotDir{
                path: path,
            }
        }
        
        err = walkFn(path, stat)
        if err != nil {
            return err
        }
        if r, _ := utf8.DecodeLastRuneInString(path); r == filepath.Separator {
            // This is fs root so we return
            break
        }

        path = filepath.Dir(path)
    }

    return nil
}


func countLines(s string) int {
    count := 0
    d := []byte(s)
    for _, b := range d {
        if b == 10 {
            count++
        }
    }
    return count
}


func getFirstLines(text string, n int) (string, int) {
    lines := 0

    d := []byte(text)
    for i, b := range d {
        if lines == n {
            return text[:i], lines
        }
        if b == 10 {
            lines++
        }
    }

    return text[:], lines
}


func getLastLines(text string, n int) (string, int) {
    lines := 0
    d := []byte(text)
    for i := len(text)-1; i >= 0; i-- {
        if lines == n {
            return text[i:], lines
        }
        if d[i] == 10 {
            lines++
        }
    }

    return text, lines
}


func printTextWithPrefixSuffix(writer io.Writer, text string, prefix string, suffix string) int {
    totalLines := 0
    scanner := bufio.NewScanner(strings.NewReader(text))

    for scanner.Scan() {
        fmt.Fprint(writer, prefix)
        fmt.Fprint(writer, scanner.Text())
        fmt.Fprintln(writer, suffix)
        totalLines++
    }

    return totalLines
}



func writeFile(path string, text string) error {
    return ioutil.WriteFile(path, []byte(text), 0644)
}


func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()

    out, err := os.Create(dst)
    if err != nil {
        return err
    }

    _, err = io.Copy(out, in)
    if err != nil {
        return err
    }

    err = out.Sync()
    if err != nil {
        return err
    }

    fi, err := os.Stat(src)
    if err != nil {
        return err
    }

    err = os.Chmod(dst, fi.Mode())
    if err != nil {
        return err
    }

    return out.Close()
}


func createEmptyFile(path string) {
    head, err := os.Create(path)
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