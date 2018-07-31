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
func RepoAddApp(scaler Scaler) {
	if !RepoAppInApps(scaler.AppID) {
		scalers = append(scalers, scaler)
		scaler.StartMonitor()
	}
}

//RepoAppInApps finds if an app is present in the apps list
func RepoAppInApps(appID string) bool {
	for _, scaler := range scalers {
		if scaler.AppID == appID {
			return true
		}
	}
	return false
}

//RepoFindApp returns an Scaler object based on app ID
func RepoFindApp(appID string) Scaler {
	for _, scaler := range scalers {
		if scaler.AppID == appID {
			return scaler
		}
	}
	return Scaler{}
}

//RepoRemoveApp re-slices the apps list to remove an app by its ID
func RepoRemoveApp(appID string) error {
	for i, scaler := range scalers {
		if scaler.AppID == appID {
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
	for _, scaler := range scalers {
		if err := RepoRemoveApp(scaler.AppID); err != nil {
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
