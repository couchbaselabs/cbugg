package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
)

var buildCmd = flag.String("buildCmd", "", "Command to perform rebuild on request.")

func rebuilder(ch chan bool) {
	for _ = range ch {
		log.Printf("Got rebuild request, executing %v", *buildCmd)
		cmd := exec.Command(*buildCmd)
		err := cmd.Run()
		if err == nil {
			log.Printf("Exiting after successful rebuild.")
			os.Exit(0)
		} else {
			log.Printf("Build error: %v", err)
		}
	}
}
