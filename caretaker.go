package main

import (
	"log"
	"time"
)

func init() {
	go janitorize()
}

func moveOldInboxItems(t time.Time) error {
	args := map[string]interface{}{
		"end_key":    []interface{}{"inbox", t.Add(-time.Duration(time.Hour))},
		"start_key":  []interface{}{"inbox", map[string]string{}},
		"descending": true,
	}

	viewRes := struct {
		Rows []struct {
			ID string
		}
	}{}

	err := db.ViewCustom("cbugg", "aging", args, &viewRes)
	if err != nil {
		return err
	}

	for _, row := range viewRes.Rows {
		log.Printf("Moving %v from inbox to new", row.ID)
		_, err = updateBug(row.ID, "status", "new", *mailFrom)
		if err != nil {
			return err
		}
	}

	return nil
}

func doPeriodicStuff(t time.Time) error {
	log.Printf("Janitoring")

	return moveOldInboxItems(t)
}

func janitorize() {
	for t := range time.Tick(time.Hour) {
		err := doPeriodicStuff(t)
		if err != nil {
			log.Printf("Error doing periodic stuff: %v", err)
		}
	}
}
