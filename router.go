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

// TODO remove redundant /app(s) endpoints
var routes = []Route{
	Route{
		"Index",
		"GET",
		"/",
		Index,
	},
	Route{
		"ListScalers",
		"GET",
		"/scalers",
		ListScalers,
	},
	Route{
		"GetScaler",
		"GET",
		"/scaler",
		GetScaler,
	},
	Route{
		"AddScaler",
		"POST",
		"/scalers",
		AddScaler,
	},
	Route{
		"RemoveScaler",
		"DELETE",
		"/scalers",
		RemoveScaler,
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
