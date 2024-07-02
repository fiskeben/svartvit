package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
)

var (
	BuildTime    string
	BuildVersion string
)

//go:embed templates/*.go.html
var embeddedTemplateRoot embed.FS

var templateRoot fs.FS

// init sets up the template filesystem which allows us to use either embedded or local templates.
// It also checks that the photos folder and the database file exist.
func init() {
	switch os.Getenv("DEVELOPMENT") {
	case "true":
		templateRoot = os.DirFS(".")
	default:
		templateRoot = embeddedTemplateRoot
	}

	_, err := os.Stat(imageFolder)
	if err != nil {
		log.Printf("photos folder not found, creating it (error: %v)", err)
		if err = os.Mkdir(imageFolder, 0755); err != nil {
			log.Fatalf("failed to create photos folder: %v", err)
		}
	}

	if _, err = os.Stat(databaseFile); err != nil {
		log.Printf("database file not found, creating it (error: %v)", err)
		if _, err = os.Create(databaseFile); err != nil {
			log.Fatalf("failed to create database file: %v", err)
		}
	}

	log.Printf("photoframe initialized (build %s, version: %s)", BuildTime, BuildVersion)
}

const imageFolder = "photos"

type IndexData struct {
	Images []Image
}

type Image struct {
	Filename string
	Path     string
	Disabled bool
}

func main() {
	var port string
	flag.StringVar(&port, "port", "8080", "port to listen on")
	flag.Parse()

	s := http.NewServeMux()

	s.HandleFunc("GET /reindex", indexExistingPhotosHandler)
	s.HandleFunc("GET /", indexHandler)
	s.HandleFunc("POST /upload", uploadHandler)
	s.HandleFunc("DELETE /photo/{name}", deleteHandler)
	s.HandleFunc("PUT /photo/{name}", toggleDisableHandler)

	fs := http.FileServer(http.Dir(imageFolder))
	s.Handle("GET /photos/", http.StripPrefix("/photos/", fs))

	log.Printf("starting server on port %s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), s); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	allImages, err := read()
	if err != nil {
		log.Printf("failed to read images from database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	images := make([]Image, 0, len(allImages))
	for _, entry := range allImages {
		images = append(images, Image{
			Filename: entry.Filename,
			Path:     imageFolder + "/" + entry.Filename,
			Disabled: entry.Disabled,
		})
	}

	t, err := template.ParseFS(templateRoot, "templates/index.go.html", "templates/photo.go.html")
	if err != nil {
		log.Printf("failed to parse template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = t.Execute(w, IndexData{Images: images}); err != nil {
		log.Printf("failed to execute template: %v", err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("photo")
	if err != nil {
		log.Printf("Error Retrieving the file: %v", err)
		return
	}

	defer file.Close()
	log.Printf("Uploaded File: %+v\n", handler.Filename)
	log.Printf("File Size: %+v\n", handler.Size)
	log.Printf("MIME Header: %+v\n", handler.Header)

	destinationFilename := imageFolder + "/" + handler.Filename

	dest, err := os.Create(destinationFilename)
	if err != nil {
		log.Printf("failed to create file '%s': %v", destinationFilename, err)
		return
	}

	defer dest.Close()
	i, err := io.Copy(dest, file)
	if err != nil {
		log.Printf("failed to copy file: %v", err)
		return
	}

	log.Printf("Copied %d bytes to %s", i, destinationFilename)

	allImages, err := read()
	if err != nil {
		log.Printf("failed to read images from database: %v", err)
		return
	}

	allImages = append(allImages, Entry{
		Filename: handler.Filename,
	})

	if err = write(allImages); err != nil {
		log.Printf("failed to write images to database: %v", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Image uploaded!"))
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := r.PathValue("name")
	filename := imageFolder + "/" + name

	_, err := os.Stat(filename)
	if err != nil {
		log.Printf("failed to open file '%s': %v", name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = os.Remove(filename); err != nil {
		log.Printf("failed to delete file '%s': %v", name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allImages, err := read()
	if err != nil {
		log.Printf("failed to read images from database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var i int
	for _, img := range allImages {
		if img.Filename == name {
			continue
		}
		allImages[i] = img
		i++
	}

	allImages = allImages[:i]

	if err = write(allImages); err != nil {
		log.Printf("failed to write images to database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func toggleDisableHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := r.PathValue("name")

	allImages, err := read()
	if err != nil {
		log.Printf("failed to read images from database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var image Image
	for i, img := range allImages {
		if img.Filename == name {
			switch img.Disabled {
			case true:
				allImages[i].Disabled = false
			case false:
				allImages[i].Disabled = true
			}
			image = Image{
				Filename: img.Filename,
				Path:     imageFolder + "/" + img.Filename,
				Disabled: allImages[i].Disabled,
			}
			break
		}
	}

	if err = write(allImages); err != nil {
		log.Printf("failed to write images to database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Toggled disabled state for image '%s'", name)
	w.Header().Add("Content-Type", "text/html")
	t, err := template.ParseFS(templateRoot, "templates/photo.go.html")
	if err != nil {
		log.Printf("failed to parse template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = t.ExecuteTemplate(w, "photo", image); err != nil {
		log.Printf("failed to execute template: %v", err)
	}
}

func indexExistingPhotosHandler(w http.ResponseWriter, r *http.Request) {
	dir, err := os.ReadDir(imageFolder)
	if err != nil {
		log.Printf("failed to read directory: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	images := make([]Entry, 0, len(dir))
	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}
		images = append(images, Entry{
			Filename: entry.Name(),
		})
	}

	if err = write(images); err != nil {
		log.Printf("failed to write images to database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Location", "/")
	w.WriteHeader(http.StatusMovedPermanently)
}
