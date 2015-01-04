// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package drive

import (
	"fmt"
	"path/filepath"
	"strings"
)

type byteDescription func(b int64) string

func memoizeBytes() byteDescription {
	cache := map[int64]string{}
	suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	maxLen := len(suffixes) - 1

	f := func(b int64) string {
		description, ok := cache[b]
		if ok {
			return description
		}

		i := 0
		for {
			if b/1024 < 1 || i >= maxLen {
				return fmt.Sprintf("%v%s", b, suffixes[i])
			}
			b /= 1024
			i += 1
		}
	}

	return f
}

var prettyBytes = memoizeBytes()

func (g *Commands) List() (err error) {
	root := g.context.AbsPathOf("")
	var relPath string
	var relPaths []string
	var remotes []*File

	for _, p := range g.opts.Sources {
		relP := g.context.AbsPathOf(p)
		relPath, err = filepath.Rel(root, relP)
		if err != nil {
			return
		}
		if relPath == "." {
			relPath = ""
		}
		relPath = "/" + relPath
		relPaths = append(relPaths, relPath)
		r, rErr := g.rem.FindByPath(relPath)
		if rErr != nil {
			fmt.Printf("%v: '%s'\n", rErr, relPath)
			return
		}
		remotes = append(remotes, r)
	}

	for _, r := range remotes {
		g.depthFirst(r.Id, "/"+r.Name, g.opts.Depth)
	}

	return
}

type attribute struct {
	human  bool
	parent string
}

func (f *File) pretty(opt attribute) {
	if f.IsDir {
		fmt.Printf("d")
	} else {
		fmt.Printf("-")
	}
	if f.Shared {
		fmt.Printf("s ")
	} else {
		fmt.Printf("- ")
	}
	if f.UserPermission != nil {
		fmt.Printf("%-10s ", f.UserPermission.Role)
	}
	fPath := fmt.Sprintf("%s/%s", opt.parent, f.Name)
	fmt.Printf("%-10s %-6s %s", prettyBytes(f.Size), fPath, f.ModTime)
	fmt.Println()
}

func (g *Commands) depthFirst(parentId, parentName string, depth int) bool {
	if depth == 0 {
		return false
	}
	if depth > 0 {
		depth -= 1
	}
	pageToken := ""

	req := g.rem.service.Files.List()
	req.Q(fmt.Sprintf("'%s' in parents and trashed=false", parentId))

	// TODO: Get pageSize from g.opts
	req.MaxResults(30)

	for {
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		res, err := req.Do()
		if err != nil {
			return false
		}

		opt := attribute{
			human:  true,
			parent: parentName,
		}
		for _, file := range res.Items {
			rem := NewRemoteFile(file)
			rem.pretty(opt)
			g.depthFirst(file.Id, parentName+"/"+file.Title, depth)
		}

		pageToken = res.NextPageToken
		if pageToken == "" {
			break
		}

		var input string
		fmt.Printf("---Next---")
		fmt.Scanln(&input)
		if len(input) >= 1 && strings.ToLower(input[:1]) == "q" {
			return false
		}
	}
	return true
}