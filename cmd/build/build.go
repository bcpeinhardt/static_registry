package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type ModuleVersionsResponse struct {
	Modules []Module `json:"modules"`
}

type Module struct {
	Versions []Version `json:"versions"`
}

type Version struct {
	Version string `json:"version"`
}

func main() {
	dir, err := os.MkdirTemp("", "modules")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: "https://github.com/coder/modules",
	})

	if err != nil {
		log.Fatal(err)
	}

	// We want to generate the source for all terraform we need to serve.
	// That means generating source files and tar bundles for every version of
	// every module

	tags, err := repo.Tags()
	if err != nil {
		log.Fatal(err)
	}

	// Right now, versions are simply done with git tags
	versions := []string{}
	tags.ForEach(func(tr *plumbing.Reference) error {
		versions = append(versions, tr.Name().Short())
		return nil
	})

	fileHandles := make(map[string]*os.File)
	gzWriters := make(map[string]*gzip.Writer)
	tarWriters := make(map[string]*tar.Writer)

	// for each version of each module, we need to generate a tarfile
	os.RemoveAll("./assets")
	os.Mkdir("./assets", 0750)
	for _, version := range versions {
		versionPathSegment := strings.ReplaceAll(version, ".", "-")
		os.Mkdir("./assets/"+versionPathSegment, 0750)

		tagRef, err := repo.Tag(version)
		if err != nil {
			log.Fatal(err)
		}
		obj, err := repo.TagObject(tagRef.Hash())
		var commit *object.Commit
		if err == nil {
			// The tag is an annotated tag, get the commit it points to
			commit, err = obj.Commit()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			// If it's a lightweight tag, the tagRef is already pointing to the commit
			commit, err = repo.CommitObject(tagRef.Hash())
			if err != nil {
				log.Fatal(err)
			}
		}

		tree, err := commit.Files()
		if err != nil {
			log.Fatal(err)
		}

		tree.ForEach(func(f *object.File) error {
			// Modules will be in named directories, so they should start with
			// a module name followed by a "/".
			// We also don't want to include hidden directories like .vscode.
			slashIndex := strings.Index(f.Name, "/")
			if !strings.HasPrefix(f.Name, ".") && slashIndex > 0 {
				moduleName := strings.Split(f.Name, "/")[0]
				fileName := versionPathSegment + "/" + moduleName + ".tar.gz"

				if _, exists := tarWriters[fileName]; !exists {
					tarFile, err := os.Create("./assets/" + fileName)
					if err != nil {
						log.Fatal(err)
					}

					// This song and dance is so I can close all this crap properly,
					// clearly I have some Go learning to do.
					fileHandles[fileName] = tarFile
					gzWriters[fileName] = gzip.NewWriter(tarFile)
					tarWriters[fileName] = tar.NewWriter(gzWriters[fileName])
				}

				// Write file header and content to the appropriate tar.Writer
				tw := tarWriters[fileName]
				hdr := &tar.Header{
					Name: strings.Split(f.Name, "/")[1],
					Mode: int64(f.Mode),
					Size: f.Blob.Size,
				}
				if err := tw.WriteHeader(hdr); err != nil {
					log.Fatal(err)
				}
				contents, err := f.Contents()
				if err != nil {
					log.Fatal(err)
				}
				if _, err := tw.Write([]byte(contents)); err != nil {
					log.Fatal(err)
				}
			}

			return nil
		})
	}

	for key, tw := range tarWriters {
		tw.Close()
		gzWriters[key].Close()
		fileHandles[key].Close()
	}

	// Now I need to build a json file to serve the versions
	// for each module
	typed_versions := []Version{}
	for _, v := range versions {
		typed_versions = append(typed_versions, Version{Version: v})
	}
	versions_resp := ModuleVersionsResponse{Modules: []Module{{Versions: typed_versions}}}
	tags_json, _ := json.Marshal(versions_resp)
	os.WriteFile("./versions.json", tags_json, 0666)
}
