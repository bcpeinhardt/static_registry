// The idea here is rather than pulling resources at runtime, we
// serve the resources statically.
// I'm thinking about just full on packing the resources into the binary,
// no container bullshit needed
package main

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

//go:embed modules
var modulesRepo embed.FS

// The in memory git repository that the api can use as context
var repo *git.Repository

type RemoteServiceDiscoveryResponse struct {
	ModulesV1 string `json:"modules.v1"`
}

type ModuleVersionsResponse struct {
    Modules []Module `json:"modules"`
}

type Module struct {
    Versions []Version `json:"versions"`
}

type Version struct {
    Version string `json:"version"`
}

func copyEmbedToFS(embedFS embed.FS, bfs billy.Filesystem, root string) error {
	fs.WalkDir(embedFS, root, func(path string, de fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		// If it's a directory, the files will be looped over in later iterations.
		if !de.IsDir() {
			data, err := embedFS.ReadFile(path)
			if err != nil {
				return err
			}

			path = strings.TrimPrefix(path, "modules/")

			// Sorry go embed directive, I'm doing cowboy shit.
			if strings.HasPrefix(path, "the_literal_git_folder") {
				path = strings.Replace(path, "the_literal_git_folder", ".git", 1)
			}

			file, err := bfs.Create(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = file.Write(data)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

func main() {

	// Copy the embedded git submodule into the in-memory file system
	// so we can load it into go-git
	memFs := memfs.New()
	err := copyEmbedToFS(modulesRepo, memFs, "modules")
	if err != nil {
		log.Fatal(err)
	}

	dotGit, err := memFs.Chroot(".git")
	if err != nil {
		log.Fatal(err)
	}

	storer := filesystem.NewStorage(dotGit, cache.NewObjectLRU(cache.DefaultMaxSize))

	// Load the repo
	repo, err = git.Open(storer, memFs)
	if err != nil {
		log.Fatal(err)
	}

	// Now we setup the API that just takes the in memory
	// git repo as context.

	http.HandleFunc("GET /.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		u, _ := json.Marshal(RemoteServiceDiscoveryResponse{ModulesV1: "/api/modules/v1"})
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(u)
		return
	})

	http.HandleFunc("GET /api/modules/v1/modules/{name}/coder/versions", func(w http.ResponseWriter, r *http.Request) {
		tags, err := repo.Tags()
		if err != nil {
			http.Error(w, "Error retrieving module versions", http.StatusInternalServerError)
		}

		versions := []Version{}
		tags.ForEach(func(tr *plumbing.Reference) error {
			versions = append(versions, Version{ Version: tr.Name().Short() })
			return nil
		})


		tagsJson, _ := json.Marshal(ModuleVersionsResponse{ Modules: []Module{ Module{ Versions: versions} }})
		w.WriteHeader(http.StatusOK)
		w.Write(tagsJson)
	})

	http.HandleFunc("GET /api/modules/v1/modules/{name}/coder/{version}/download", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		version := r.PathValue("version")

		w.Header().Add("X-Terraform-Get", "/api/modules/"+name+"?archive=tar.gz&ref="+version)
		w.WriteHeader(http.StatusNoContent)
	})

	http.HandleFunc("GET /api/modules/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		version := r.URL.Query().Get("ref")
		if version == "" {
			version = "main"
		} else if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		if !strings.Contains(r.Header.Get("accept-encoding"), "gzip") {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		tagRef, err := repo.Tag(version)
		if err != nil {
			http.Error(w, "Could not find a matching tag", http.StatusNotFound)
		}

		obj, err := repo.TagObject(tagRef.Hash())
		var commit *object.Commit
		if err == nil {
			// The tag is an annotated tag, get the commit it points to
			commit, err = obj.Commit()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		} else {
			// If it's a lightweight tag, the tagRef is already pointing to the commit
			commit, err = repo.CommitObject(tagRef.Hash())
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}

		tree, err := commit.Files()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		w.Header().Set("Content-Disposition", "attachment; filename=" + name + ".tar.gz")
    	w.Header().Set("Content-Type", "application/gzip")

		gw := gzip.NewWriter(w)
		defer gw.Close()

		tw := tar.NewWriter(gw)
		defer tw.Close()

		tree.ForEach(func(file *object.File) error {

			if strings.HasPrefix(file.Name, name+"/") {
				hdr := &tar.Header{
					Name: file.Name,
					Mode: int64(file.Mode),
					Size: file.Blob.Size,
				}
				if err := tw.WriteHeader(hdr); err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
				contents, _ := file.Contents()
				if _, err := tw.Write([]byte(contents)); err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}

			return nil
		})

		err = tw.Close()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		
	})

	log.Fatal(http.ListenAndServeTLS(":8080", "certs/localhost-cert.pem", "certs/localhost-key.pem", nil))
}
