package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html"
	"html/template"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/fiskeben/svartvit/internal/epd"
)

var (
	BuildTime        string
	BuildVersion     string
	workingDirectory = "."
	databaseFile     = "database.json"
	currentImageFile = "/tmp/current.png"
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
}

func initializeStorage() {
	photosDir := workingDirectory + "/photos"
	_, err := os.Stat(photosDir)
	if err != nil {
		log.Printf("photos folder not found, creating it (error: %v)", err)
		if err = os.Mkdir(photosDir, 0755); err != nil {
			log.Fatalf("failed to create photos folder: %v", err)
		}
	}

	if _, err = os.Stat(databaseFile); err != nil {
		log.Printf("database file not found, creating it (error: %v)", err)
		if _, err = os.Create(databaseFile); err != nil {
			log.Fatalf("failed to create database file: %v", err)
		}
	}

	log.Printf("svartvit initialized (build %s, version: %s)", BuildTime, BuildVersion)
}

type IndexData struct {
	Images []Image
}

type Image struct {
	Id       string
	Name     string
	Path     string
	Disabled bool
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var port, dir, fontFile, overlay, locale string
	var interval time.Duration
	var useDisplay bool
	flag.StringVar(&port, "port", "8080", "port to listen on")
	flag.StringVar(&dir, "dir", "", "working directory, blank for cwd")
	flag.StringVar(&fontFile, "font", "font.ttf", "TrueType font used for the date label")
	flag.StringVar(&currentImageFile, "current", currentImageFile, "where to write the image currently shown on the frame")
	flag.DurationVar(&interval, "interval", time.Hour, "how often to change the image on the frame")
	flag.StringVar(&overlay, "overlay", "date", "what to draw on top of the photo: "+names(overlays))
	flag.StringVar(&locale, "locale", "en", "language of the date: "+names(dateFormats))
	flag.BoolVar(&useDisplay, "epd", false, "show images on the e-Paper display (Raspberry Pi only)")
	flag.Parse()

	if dir != "" {
		workingDirectory = dir
		databaseFile = dir + "/" + databaseFile
	}

	initializeStorage()

	if err := loadFont(fontFile); err != nil {
		log.Fatalf("failed to load font '%s': %v", fontFile, err)
	}

	var ok bool
	if currentOverlay, ok = overlays[overlay]; !ok {
		log.Fatalf("unknown overlay '%s', use one of: %s", overlay, names(overlays))
	}
	if dateString, ok = dateFormats[locale]; !ok {
		log.Fatalf("unknown locale '%s', use one of: %s", locale, names(dateFormats))
	}

	if useDisplay {
		d, closer, err := epd.Open()
		if err != nil {
			log.Fatalf("failed to open e-Paper display: %v", err)
		}
		defer closer.Close()
		display = d
	}

	go updateImage(ctx, interval)

	s := http.NewServeMux()

	requestLogger := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s", r.Method, r.URL)
			h.ServeHTTP(w, r)
		})
	}

	s.Handle("GET /", requestLogger(indexHandler()))
	s.Handle("GET /reindex", requestLogger(indexExistingPhotosHandler()))
	s.Handle("POST /upload", requestLogger(uploadHandler()))
	s.Handle("GET /photo/{id}", requestLogger(getPhotoHandler()))
	s.Handle("PUT /photo/{id}", requestLogger(savePhotoHandler()))
	s.Handle("GET /photo/{id}/edit", requestLogger(editPhotoHandler()))
	s.Handle("PUT /photo/{id}/toggle", requestLogger(toggleDisableHandler()))
	s.Handle("PUT /photo/{id}/set", requestLogger(setPhotoHandler()))
	s.Handle("GET /photo/{id}/set/events", requestLogger(setPhotoEventsHandler()))
	s.Handle("GET /photo/{id}/preview", requestLogger(previewHandler()))
	s.Handle("DELETE /photo/{id}", requestLogger(deleteHandler()))

	imageHandler := http.FileServer(http.Dir(path.Join(workingDirectory, "photos")))
	s.Handle("GET /photos/", http.StripPrefix("/photos", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("serving file %s", r.URL.Path)
		imageHandler.ServeHTTP(w, r)
	})))

	log.Printf("starting server on port %s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), s); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func indexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allImages, err := read()
		if err != nil {
			log.Printf("failed to read images from database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		images := make([]Image, 0, len(allImages))
		for _, entry := range allImages {
			images = append(images, Image{
				Id:       entry.Id,
				Name:     entry.Name,
				Path:     "/photos/" + entry.Filename,
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
}

func uploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		destinationFilename := workingDirectory + "/photos/" + handler.Filename

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

		allImages = append(allImages, newEntry(handler.Filename))

		if err = write(allImages); err != nil {
			log.Printf("failed to write images to database: %v", err)
			return
		}

		w.Header().Add("Location", "/")
		w.Header().Add("HX-Location", "/")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Image uploaded!"))
	}
}

func deleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		filename := workingDirectory + "/photos/" + img.Filename

		_, err = os.Stat(filename)
		if err != nil {
			log.Printf("failed to open file '%s': %v", filename, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = os.Remove(filename); err != nil {
			log.Printf("failed to delete file '%s': %v", filename, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = deleteImage(id); err != nil {
			log.Printf("failed to delete image from database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func toggleDisableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		img.Disabled = !img.Disabled
		if err = save(img); err != nil {
			log.Printf("failed to save image to database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		image := Image{
			Id:       img.Id,
			Name:     img.Name,
			Path:     "/photos/" + img.Filename,
			Disabled: img.Disabled,
		}

		log.Printf("Toggled disabled state for image '%s'", id)
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
}

func editPhotoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		image := Image{
			Id:       img.Id,
			Name:     img.Name,
			Path:     "/photos/" + img.Filename,
			Disabled: img.Disabled,
		}

		w.Header().Add("Content-Type", "text/html")
		t, err := template.ParseFS(templateRoot, "templates/edit.go.html")
		if err != nil {
			log.Printf("failed to parse template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = t.Execute(w, image); err != nil {
			log.Printf("failed to execute template: %v", err)
		}
	}
}

func getPhotoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		image := Image{
			Id:       img.Id,
			Name:     img.Name,
			Path:     "/photos/" + img.Filename,
			Disabled: img.Disabled,
		}

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
}

func savePhotoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id := r.PathValue("id")

		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		img.Name = r.FormValue("name")
		img.Disabled = r.FormValue("disabled") == "on"

		if err = save(img); err != nil {
			log.Printf("failed to save image to database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated image '%s'", id)

		image := Image{
			Id:       img.Id,
			Name:     img.Name,
			Path:     "/photos/" + img.Filename,
			Disabled: img.Disabled,
		}

		w.Header().Add("Content-Type", "text/html")
		t, err := template.ParseFS(templateRoot, "templates/photo.go.html")
		if err != nil {
			log.Printf("failed to parse template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = t.ExecuteTemplate(w, "photo", image); err != nil {
			log.Printf("failed to execute template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func indexExistingPhotosHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := reindex(workingDirectory + "/photos"); err != nil {
			log.Printf("failed to reindex photos: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Location", "/")
		w.WriteHeader(http.StatusSeeOther)
	}
}

// writeEvent writes a single SSE event and flushes it to the client.
func writeEvent(w io.Writer, event, data string) error {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	return nil
}

// setPhotoHandler returns a fragment that connects to setPhotoEventsHandler,
// which does the actual work and streams the output back.
func setPhotoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		t, err := template.ParseFS(templateRoot, "templates/set.go.html")
		if err != nil {
			log.Printf("failed to parse template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = t.Execute(w, Image{Id: img.Id}); err != nil {
			log.Printf("failed to execute template: %v", err)
		}
	}
}

// setPhotoEventsHandler shows the photo on the frame and streams progress as server-sent events.
func setPhotoEventsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		progress := func(msg string) {
			log.Print(msg)
			writeEvent(w, "Progress", "<p>"+html.EscapeString(msg)+"</p>")
		}

		if err = showPhoto(workingDirectory+"/photos/"+img.Filename, progress); err != nil {
			log.Printf("failed to show photo: %v", err)
			progress("Error: " + err.Error())
		}

		writeEvent(w, "Done", "")
	}
}

// previewHandler renders the photo as it would look on the frame, without updating the frame.
func previewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		img, err := get(id)
		if err != nil {
			log.Printf("could not find image in database: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		frame, err := renderFile(workingDirectory + "/photos/" + img.Filename)
		if err != nil {
			log.Printf("failed to render preview: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		if err = png.Encode(w, frame); err != nil {
			log.Printf("failed to encode preview: %v", err)
		}
	}
}
