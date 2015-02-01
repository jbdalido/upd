// Route receiving the data when a file is uploaded.
// Copyright © 2015 - Rémy MATHIEU
package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type SendHandler struct {
	Server *Server // pointer to the started server
}

const (
	SECRET_KEY_HEADER = "X-Clioud-Key"
)

// Json returned to the client
type SendResponse struct {
	Name         string    `json:"name"`
	DeleteKey    string    `json:"delete_key"`
	DeletionTime time.Time `json:"availaible_until"`
}

const (
	MAX_MEMORY = 1024 * 1024
	DICTIONARY = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func (s *SendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// checks the secret key
	key := r.Header.Get(SECRET_KEY_HEADER)
	if s.Server.Config.SecretKey != "" && key != s.Server.Config.SecretKey {
		w.WriteHeader(403)
		return
	}

	// parse the form
	reader, _, err := r.FormFile("data")

	if err != nil {
		w.WriteHeader(500)
		log.Println("[err] Error while receiving data (FormFile).")
		log.Println(err)
		return
	}

	// read the data
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		w.WriteHeader(500)
		log.Println("[err] Error while receiving data (ReadAll).")
		log.Println(err)
		return
	}

	// write the file in the directory
	var name string
	var original string

	// name
	if len(r.Form["name"]) == 0 {
		w.WriteHeader(400)
		return
	} else {
		original = filepath.Base(r.Form["name"][0])
	}

	for {
		name = s.randomString(8)
		if s.Server.Metadata.Data[name].Filename == "" {
			break
		}
	}

	var expirationTime time.Time
	now := time.Now()

	// reads the TTL
	var ttl string
	if len(r.Form["ttl"]) > 0 {
		ttl = r.Form["ttl"][0]
		// check that the value is a correct duration
		_, err := time.ParseDuration(ttl)
		if err != nil {
			println(err.Error())
			w.WriteHeader(400)
			return
		}

		// compute the expiration time
		expirationTime = s.Server.computeEndOfLife(ttl, now)
	}

	// reads the tags
	tags := make([]string, 0)
	if len(r.Form["tags"]) > 0 {
		tags = strings.Split(r.Form["tags"][0], ",")
	}

	// writes the data on the storage
	s.Server.WriteFile(name, data)

	// add to metadata
	deleteKey := s.randomString(16)
	s.addMetadata(name, original, tags, ttl, deleteKey, now)
	s.Server.writeMetadata(true)

	// encode the response json
	response := SendResponse{
		Name:         name,
		DeleteKey:    deleteKey,
		DeletionTime: expirationTime,
	}

	resp, _ := json.Marshal(response)

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// randomString generates a random valid URL string of the given size
func (s *SendHandler) randomString(size int) string {
	result := ""

	for i := 0; i < size; i++ {
		result += string(DICTIONARY[rand.Int31n(int32(len(DICTIONARY)))])
	}

	return result
}

// addMetadata adds the given entry to the Server metadata information.
func (s *SendHandler) addMetadata(name string, original string, tags []string, ttl string, key string, now time.Time) {
	metadata := Metadata{
		Filename:     name,
		Original:     original,
		Tags:         tags,
		TTL:          ttl,
		DeleteKey:    key,
		CreationTime: now,
	}
	s.Server.Metadata.Data[name] = metadata

	// add the entry
	s.Server.Metadata.LastUploaded = append([]string{name}, s.Server.Metadata.LastUploaded...)
	// keep only 20 entries
	limitMax := 19
	if len(s.Server.Metadata.LastUploaded) < limitMax {
		limitMax = len(s.Server.Metadata.LastUploaded)
	}
	s.Server.Metadata.LastUploaded = s.Server.Metadata.LastUploaded[0:limitMax]
}
