package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/uuid"
)

const databaseFile = "database.json"

type Entry struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Disabled bool   `json:"disabled"`
}

type Entries []Entry

func newEntry(filename string) Entry {
	return Entry{
		Id:       uuid.NewString(),
		Filename: filename,
		Name:     filename,
	}
}

func write(entries Entries) error {
	f, err := os.OpenFile(databaseFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("failed to open database file: %v", err)
		f, err = os.Create(databaseFile)
		if err != nil {
			log.Printf("failed to create database file: %v", err)
			return err
		}
	}

	if err = json.NewEncoder(f).Encode(entries); err != nil {
		log.Printf("failed to encode entries: %v", err)
		return err
	}

	return nil
}

func read() (Entries, error) {
	f, err := os.Open(databaseFile)
	if err != nil {
		log.Printf("failed to open database file: %v", err)
		return nil, nil
	}

	var entries Entries
	if err = json.NewDecoder(f).Decode(&entries); err != nil {
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		log.Printf("failed to decode entries: %v", err)
		return nil, err
	}

	return entries, nil
}

func get(id string) (Entry, error) {
	entries, err := read()
	if err != nil {
		return Entry{}, err
	}

	for _, entry := range entries {
		if entry.Id == id {
			return entry, nil
		}
	}

	return Entry{}, fmt.Errorf("image not found (%v)", id)
}

func save(img Entry) error {
	allImages, err := read()
	if err != nil {
		return err
	}

	for i := range allImages {
		if allImages[i].Id == img.Id {
			allImages[i] = img
			break
		}
	}

	return write(allImages)
}

func deleteImage(id string) error {
	allImages, err := read()
	if err != nil {
		return err
	}

	var i int
	for _, img := range allImages {
		if img.Id == id {
			continue
		}
		allImages[i] = img
		i++
	}

	allImages = allImages[:i]

	return write(allImages)
}
