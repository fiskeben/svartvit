package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
)

const databaseFile = "database.json"

type Entry struct {
	Filename string `json:"filename"`
	Disabled bool   `json:"disabled"`
}

type Entries []Entry

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
