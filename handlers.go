package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

//Message struct to be displayed as json
type Message struct {
	Message string `json:"message"`
}

//AppID for form input of IDs
type AppID struct {
	AppID string `json:"app_id"`
}

//JSONResponse write generic message as json response
func JSONResponse(w http.ResponseWriter, message string) {
	response := Message{message}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Panicln(err)
	}
}

//RemoveApp removes app by its ID from the pool of apps being monitored
func RemoveScaler(w http.ResponseWriter, r *http.Request) {
	var appID AppID

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		log.Panicln(err)
	}
	if err := r.Body.Close(); err != nil {
		log.Panicln(err)
	}
	if err := json.Unmarshal(body, &appID); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			log.Panicln(err)
		}
	}

	if err := RepoRemoveApp(appID.AppID); err != nil {
		w.WriteHeader(400)
		log.Panicln(err)
	}
	JSONResponse(w, "OK")
}

//AddApp adds a scaler to the pool of monitored apps
func AddScaler(w http.ResponseWriter, r *http.Request) {
	var scaler Scaler
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		log.Panicln(err)
	}
	if err := r.Body.Close(); err != nil {
		log.Panicln(err)
	}
	if err := json.Unmarshal(body, &scaler); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			log.Panicln(err)
		}
	}
	RepoAddApp(scaler)

	w.WriteHeader(200)
	JSONResponse(w, "OK")
}

//GetApp finds and displays a monitored app by its ID
func GetScaler(w http.ResponseWriter, r *http.Request) {
	var appID AppID
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		log.Panicln(err)
	}
	if err := r.Body.Close(); err != nil {
		log.Panicln(err)
	}
	if err := json.Unmarshal(body, &appID); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(422) // unprocessable entity
		if err := json.NewEncoder(w).Encode(err); err != nil {
			log.Panicln(err)
		}
	}

	app := RepoFindApp(appID.AppID)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(app); err != nil {
		log.Panicln(err)
	}
}

//IndexApps displays all monitored apps
func ListScalers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(scalers); err != nil {
		log.Panicln(err)
	}
}

//Index for slash, returns version
func Index(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, "Autoscaler, v0.0.2")
}
