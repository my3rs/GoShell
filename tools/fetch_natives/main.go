package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type TargetFile struct {
	Target string
	Name   string
	URL    string
	SHA256 string
}

var targets = []TargetFile{
	{
		Target: "darwin-amd64",
		Name:   "libghostty-vt.dylib",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-darwin-amd64.dylib",
		SHA256: "421e98d2683b39555209c59395655b4a217da0382e7669c92f6dff3949832664",
	},
	{
		Target: "darwin-arm64",
		Name:   "libghostty-vt.dylib",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-darwin-arm64.dylib",
		SHA256: "6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41",
	},
	{
		Target: "linux-amd64",
		Name:   "libghostty-vt.so",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-linux-amd64.so",
		SHA256: "9dcd8af0de67778a561f436cb673a9fb1738be8fa00849dad49606baed1e57d7",
	},
	{
		Target: "linux-arm64",
		Name:   "libghostty-vt.so",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-linux-arm64.so",
		SHA256: "fb163f8f123206956751f1c4f0303b40899513b7dc171a8ab8d85ba7cd541bc0",
	},
	{
		Target: "windows-amd64",
		Name:   "ghostty-vt.dll",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-windows-amd64.dll",
		SHA256: "53e8e2dd979702e8f2c48107af19be5a76efc2e88eb7cf1097218e899665da41",
	},
	{
		Target: "windows-arm64",
		Name:   "ghostty-vt.dll",
		URL:    "https://github.com/egoist/mygo/releases/download/libghostty-vt-befcdfd2c3a1/libghostty-vt-windows-arm64.dll",
		SHA256: "de56a362f12cce8be38e883ffc8aec90a890e0574eb73aac6e833a60fda26064",
	},
}

func main() {
	client := &http.Client{Timeout: 5 * time.Minute}

	for _, t := range targets {
		destDir := filepath.Join("assets", "natives", t.Target)
		destFile := filepath.Join(destDir, t.Name)

		if err := os.MkdirAll(destDir, 0o755); err != nil {
			fmt.Printf("Error creating dir %s: %v\n", destDir, err)
			os.Exit(1)
		}

		if stat, err := os.Stat(destFile); err == nil && stat.Size() > 0 {
			if checkSHA256(destFile, t.SHA256) {
				fmt.Printf("[%s] Already exists and verified: %s\n", t.Target, destFile)
				continue
			}
		}

		fmt.Printf("[%s] Downloading %s -> %s\n", t.Target, t.URL, destFile)
		req, err := http.NewRequest("GET", t.URL, nil)
		if err != nil {
			fmt.Printf("Error building request: %v\n", err)
			os.Exit(1)
		}
		req.Header.Set("User-Agent", "GoShell-NativeDownloader/1.0")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error downloading: %v\n", err)
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Printf("Error HTTP status: %s\n", resp.Status)
			os.Exit(1)
		}

		tmpFile := destFile + ".tmp"
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			resp.Body.Close()
			fmt.Printf("Error creating file: %v\n", err)
			os.Exit(1)
		}

		hasher := sha256.New()
		w := io.MultiWriter(f, hasher)
		_, err = io.Copy(w, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			os.Remove(tmpFile)
			fmt.Printf("Error saving stream: %v\n", err)
			os.Exit(1)
		}

		sum := hex.EncodeToString(hasher.Sum(nil))
		if sum != t.SHA256 {
			os.Remove(tmpFile)
			fmt.Printf("Checksum mismatch for %s! Got %s, want %s\n", t.Target, sum, t.SHA256)
			os.Exit(1)
		}

		if err := os.Rename(tmpFile, destFile); err != nil {
			fmt.Printf("Error renaming file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[%s] Successfully downloaded and verified: %s\n", t.Target, destFile)
	}
	fmt.Println("All native libraries are ready!")
}

func checkSHA256(path string, expected string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false
	}
	return hex.EncodeToString(hasher.Sum(nil)) == expected
}
