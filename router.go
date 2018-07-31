package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

//Route struct describing a router route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

var routes = []Routes {
	Route {
		"Index",
		"GET",
		"/",
		Index,
	},
	Route {
		"IndexApps",
		"GET",
		"/apps",
		IndexApps,
	},
	Route {
		"GetApp",
		"GET",
		"/app",
		GetApp,
	},
	Route {
		"AddApp",
		"POST",
		"/apps",
		AddApp,
	},
	Route {
		"RemoveApp",
		"DELETE",
		"/apps",
		RemoveApp,
	},
}

//NewRouter handles creation of new mux router
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)

	}
	return router
}
