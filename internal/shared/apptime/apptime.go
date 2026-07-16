// Package apptime centralizes the application's timezone, used for "today" and
// week boundaries shared by the workout and group modules.
package apptime

import "time"

// Location is the app timezone (America/Sao_Paulo), falling back to UTC when the
// zone database is unavailable. Resolved once at startup.
var Location = load()

func load() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}
