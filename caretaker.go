package main

import (
	"log"
	"time"
)

func init() {
	go janitorize()
}

func moveOldInboxItems(t time.Time) error {
	t = t.UTC()
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

func processReminder(rid string) error {
	r := Reminder{}
	err := db.Get(rid, &r)
	if err != nil {
		return err
	}

	bug, err := getBug(r.BugId)
	if err != nil {
		return err
	}

	log.Printf("Reminding %v about %v", r.User, bug.Title)

	sendNotifications("reminder_notification", []string{r.User},
		map[string]interface{}{
			"Bug":      bug,
			"Reminder": r,
		})

	return db.Delete(rid)
}

func processReminders(t time.Time) error {
	args := map[string]interface{}{
		"end_key": t.UTC(),
		"stale":   false,
	}

	viewRes := struct {
		Rows []struct {
			ID string
		}
	}{}

	err := db.ViewCustom("cbugg", "reminders", args, &viewRes)
	if err != nil {
		return err
	}

	for _, row := range viewRes.Rows {
		maybeLog(row.ID, processReminder(row.ID))
	}

	return nil
}

func maybeLog(name string, err error) {
	if err != nil {
		log.Printf("Error in %v: %v", name, err)
	}
}

func doPeriodicStuff(t time.Time) {
	log.Printf("Janitoring")

	maybeLog("move inbox items", moveOldInboxItems(t))
	maybeLog("process reminders", processReminders(t))
}

func janitorize() {
	for t := range time.Tick(time.Minute * 5) {
		doPeriodicStuff(t)
	}
}
