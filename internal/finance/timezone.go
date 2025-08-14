package finance

import "time"

// getEasternTime returns America/New_York location, falling back to fixed EST if tzdata is missing.
func getEasternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("EST", -5*3600)
	}
	return loc
}
