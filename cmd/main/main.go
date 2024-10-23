package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed site/dist
var frontend embed.FS // embedding the frontend

// Response types

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

func discoveryHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := json.Marshal(RemoteServiceDiscoveryResponse{ModulesV1: "/api/modules/v1"})
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(u)
	return
}

func versionsHandler(w http.ResponseWriter, r *http.Request) {
	tagsJsonBytes, _ := os.ReadFile("./versions.json")
	w.WriteHeader(http.StatusOK)
	w.Write(tagsJsonBytes)
}

func downloadVersionHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")

	w.Header().Add("X-Terraform-Get", "/api/modules/"+name+"?archive=tar.gz&ref="+version)
	w.WriteHeader(http.StatusNoContent)
}

func downloadSourceTarHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.URL.Query().Get("ref")

	if version == "" {
		version = "main"
	} else if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	if !strings.Contains(r.Header.Get("accept-encoding"), "gzip") {
		http.Error(w, "Must accept gzip encoding", http.StatusBadRequest)
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+name+".tar.gz")
	w.Header().Set("Content-Type", "application/gzip")

	// Path to the tar file on disk
	tarFilePath := "./assets/" + strings.ReplaceAll(version, ".", "-") + "/" + name + ".tar.gz"

	// Open the tar file
	file, err := os.Open(tarFilePath)
	if err != nil {
		http.Error(w, "Unable to open tar file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Serve the file by copying it to the response writer
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Error reading file stat", http.StatusInternalServerError)
	}
	http.ServeContent(w, r, name+".tar.gz", stat.ModTime(), file)
	if err != nil {
		http.Error(w, "Error serving the tar file", http.StatusInternalServerError)
	}
}

func main() {

	http.HandleFunc("GET /.well-known/terraform.json", discoveryHandler)
	http.HandleFunc("GET /api/modules/v1/modules/{name}/coder/versions", versionsHandler)
	http.HandleFunc("GET /api/modules/v1/modules/{name}/coder/{version}/download", downloadVersionHandler)
	http.HandleFunc("GET /api/modules/{name}", downloadSourceTarHandler)

	dist, err := fs.Sub(frontend, "site/dist")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("GET /", http.FileServerFS(dist))

	log.Fatal(http.ListenAndServeTLS(":8080", "certs/localhost-cert.pem", "certs/localhost-key.pem", nil))
}
