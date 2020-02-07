package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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