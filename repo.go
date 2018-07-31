package main

import (
	"fmt"
	"strings"
	"time"
)

// TODO make this part of repo
var scalers Scalers
var tickers map[string]*time.Ticker

func init() {
	tickers = make(map[string]*time.Ticker)
}

//RepoAddApp adds a Scaler to the repo
func RepoAddApp(r Scaler) {
	if !RepoAppInApps(r.AppID) {
		apps = append(scalers, r)
		r.StartMonitor()
	}
}

//RepoAppInApps finds if an app is present in the apps list
func RepoAppInApps(appID string) bool {
	for _, r := range scalers {
		if r.AppID == appID {
			return true
		}
	}
	return false
}

//RepoFindApp returns an Scaler object based on app ID
func RepoFindApp(appID string) Scaler {
	for _, r := range scalers {
		if r.AppID == appID {
			return r
		}
	}
	return Scaler{}
}

//RepoRemoveApp re-slices the apps list to remove an app by its ID
func RepoRemoveApp(appID string) error {
	for i, r := range scalers {
		if r.AppID == appID {
			scalers = append(scalers[:i], scalers[i+1:]...)
			//Stopping the ticker
			tickers[appID].Stop()
			return nil
		}
	}
	return fmt.Errorf("could not find Scaler with id of %s to delete", appID)
}

//RepoRemoveAllApps cycles through the apps array and removes them all
func RepoRemoveAllApps() error {
	for _, r := range Scalers {
		if err := RepoRemoveApp(r.AppID); err != nil {
			return err
		}
	}
	return nil
}

func prependSlash(appID string) string {
	if strings.Index(appID, "/") != 1 {
		appID = fmt.Sprintf("/%s", appID)
	}
	return appID
}
