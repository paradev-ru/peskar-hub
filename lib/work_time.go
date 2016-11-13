package lib

import "time"

func CurrentDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func IsAvailable(t time.Time, dndStart, dndStop int) bool {
	var currDayT, dndStartT, dndStopT time.Time
	var dndStartD, dndStopD time.Duration

	if t.Weekday() == 0 || t.Weekday() == 6 {
		return true
	}

	dndStop++
	if dndStart > 0 {
		dndStart--
	}

	dndStartD = time.Duration(dndStart) * time.Hour
	dndStopD = time.Duration(dndStop) * time.Hour

	currDayT = CurrentDay(t)

	dndStartT = currDayT.Add(dndStartD)
	dndStopT = currDayT.Add(dndStopD)

	if dndStart > dndStop {
		dndStopT = dndStopT.Add(24 * time.Hour)
	}

	if t.After(dndStartT) && t.Before(dndStopT) {
		return false
	}

	dndStartT = dndStartT.Add(-24 * time.Hour)
	dndStopT = dndStopT.Add(-24 * time.Hour)

	if t.After(dndStartT) && t.Before(dndStopT) {
		return false
	}

	return true
}
