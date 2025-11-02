package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

var gitModes = map[string]string{
	"040000": "tree",
	"100644": "blob",
	"100755": "blob",
	"120000": "blob",
	"160000": "commit",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: got <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}

		fmt.Println("Initialized git directory")
	case "cat-file":
		if len(os.Args) < 4 {
			handleError(errors.New("usage: got cat-file -p [<args>...]"))
		}
		if os.Args[2] != "-p" {
			handleError(errors.New("usage: got cat-file -p [<args>...]"))
		}

		hash := os.Args[3]
		if len(hash) < 40 {
			handleError(errors.New("invalid hash"))
		}
		filePath := fmt.Sprintf(".git/objects/%s/%s", hash[:2], hash[2:])
		b, err := os.ReadFile(filePath)
		if err != nil {
			handleError(err)
		}
		reader := bytes.NewReader(b)
		r, err := zlib.NewReader(reader)
		if err != nil {
			handleError(err)
		}
		defer r.Close()

		content, err := io.ReadAll(r)
		if err != nil {
			handleError(err)
		}
		nullIndex := bytes.IndexByte(content, byte(0))
		if nullIndex == -1 {
			handleError(errors.New("invalid git object format"))
		}
		// typeIndex := bytes.IndexByte(content, byte(' '))
		// sizeIndex := typeIndex + 1
		// objectType := string(content[:typeIndex])
		// contentSize := string(content[sizeIndex:nullIndex])
		fmt.Print(string(content[nullIndex+1:]))
	case "hash-object":
		if len(os.Args) < 4 {
			handleError(errors.New("usage: got hash-object [<args>...]"))
		}
		if os.Args[2] != "-w" {
			handleError(errors.New("usage: got hash-object -w [<args>...]"))
		}

		filepath := os.Args[3]

		b, err := os.ReadFile(filepath)
		if err != nil {
			handleError(err)
		}

		header := fmt.Sprintf("blob %d\x00", len(b))
		b = append([]byte(header), b...)
		hash := computeHash(b)
		fmt.Println(hash)

		buf := new(bytes.Buffer)

		writer := zlib.NewWriter(buf)

		if _, err := writer.Write(b); err != nil {
			handleError(err)
		}

		if err := writer.Close(); err != nil {
			handleError(err)
		}

		dir := fmt.Sprintf(".git/objects/%s", hash[:2])

		if err := os.MkdirAll(dir, 0755); err != nil {
			handleError(err)
		}

		objectPath := fmt.Sprintf("%s/%s", dir, hash[2:])
		if err := os.WriteFile(objectPath, buf.Bytes(), 0644); err != nil {
			handleError(err)
		}
	case "ls-tree":
		if len(os.Args) < 3 {
			handleError(errors.New("usage: got ls-tree [<args>...] [hash]"))
		}

		var nameOnly bool
		var hash string

		if os.Args[2] == "--name-only" {
			nameOnly = true
			hash = os.Args[3]
		} else {
			hash = os.Args[2]
		}

		if len(hash) != 40 {
			handleError(errors.New("invalid hash"))
		}

		filePath := fmt.Sprintf(".git/objects/%s/%s", hash[:2], hash[2:])
		b, err := os.ReadFile(filePath)
		if err != nil {
			handleError(err)
		}

		r, err := zlib.NewReader(bytes.NewReader(b))
		if err != nil {
			handleError(err)
		}
		defer r.Close()

		content, err := io.ReadAll(r)
		if err != nil {
			handleError(err)
		}

		nullIndex := bytes.IndexByte(content, 0)
		if nullIndex == -1 {
			handleError(errors.New("invalid git object format"))
		}

		type TreeEntry struct {
			Mode string
			Type string
			Hash string
			Name string
		}

		// Parse tree entries
		var entries []TreeEntry
		data := content[nullIndex+1:]
		for len(data) > 0 {
			// Parse mode
			spaceIdx := bytes.IndexByte(data, ' ')
			if spaceIdx == -1 {
				handleError(errors.New("malformed entry: missing mode"))
			}
			mode := string(data[:spaceIdx])
			data = data[spaceIdx+1:]

			nullIdx := bytes.IndexByte(data, 0)
			if nullIdx == -1 {
				handleError(errors.New("malformed entry: missing name terminator"))
			}
			name := string(data[:nullIdx])
			data = data[nullIdx+1:]

			if len(data) < 20 {
				handleError(errors.New("malformed entry: incomplete hash"))
			}
			hashBytes := data[:20]
			data = data[20:]

			entries = append(entries, TreeEntry{
				Mode: mode,
				Type: gitModes[mode],
				Hash: sha1toHex(hashBytes),
				Name: name,
			})
		}

		for _, entry := range entries {
			if nameOnly {
				fmt.Println(entry.Name)
			} else {
				fmt.Printf("%s %s %s %s\n", entry.Mode, entry.Type, entry.Hash, entry.Name)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func handleError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
func computeHash(data []byte) string {
	h := sha1.Sum(data)
	return hex.EncodeToString(h[:])
}
func sha1toHex(sha1sum []byte) string {
	return hex.EncodeToString(sha1sum)
}
