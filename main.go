package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tj330/csvtojson"
)

type templateData struct {
	Data       string
	ErrMessage string
}

func createJsonFile(filename string) (string, error) {
	extension := filepath.Ext(filename)
	newfile := fmt.Sprintf("%s.json", strings.TrimSuffix(filename, extension))
	newfile = "data/" + newfile

	reader, err := os.Open("data/" + filename)
	if err != nil {
		return "", err
	}
	data, err := csvtojson.Convert(reader)
	_, err = os.Create(newfile)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(newfile, data, 0644)
	if err != nil {
		return "", err
	}
	return newfile, nil
}

func main() {
	tpl := template.Must(template.ParseFiles("./templates/index.gohtml"))
	fs := http.FileServer(http.Dir("./static"))

	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, nil)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			r.ParseMultipartForm(10 << 20) //10 MB
			file, header, err := r.FormFile("uploadedFile")
			t := templateData{}
			if filepath.Ext(header.Filename) != ".csv" {
				log.Println("not a csv file")
				w.WriteHeader(http.StatusBadRequest)
				t.ErrMessage = "Not a CSV file"
				tpl.Execute(w, t)
				return
			}
			if err != nil {
				log.Println("error retrieving file", err)
				w.WriteHeader(http.StatusInternalServerError)
				t.ErrMessage = "Something went wrong"
				tpl.Execute(w, t)
				return
			}
			defer file.Close()

			dst, err := os.Create("data/" + header.Filename)
			if err != nil {
				log.Println("error creating file", err)
				w.WriteHeader(http.StatusInternalServerError)
				t.ErrMessage = "Something went wrong"
				tpl.Execute(w, t)
				return
			}
			defer dst.Close()
			if _, err := io.Copy(dst, file); err != nil {
				log.Println("error copying file", err)
				w.WriteHeader(http.StatusInternalServerError)
				t.ErrMessage = "Something went wrong"
				tpl.Execute(w, t)
				return
			}

			newfile, err := createJsonFile(header.Filename)
			if err != nil {
				log.Println("error creating json file", err)
				w.WriteHeader(http.StatusInternalServerError)
				t.ErrMessage = "Something went wrong"
				tpl.Execute(w, t)
				return
			}

			tpl := template.Must(template.ParseFiles("./templates/page.gohtml"))
			t = templateData{
				Data: "/download/" + newfile,
			}

			tpl.Execute(w, t)
		}
	})

	http.HandleFunc("/download/{file...}", downloadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file") // e.g. "data/month.json"

	clean := path.Clean(file)
	if strings.Contains(clean, "..") {
		log.Println("invalid path")
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, path.Base(clean)),
	)

	http.ServeFile(w, r, "./"+clean)
}
