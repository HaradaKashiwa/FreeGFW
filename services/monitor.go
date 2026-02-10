package services

import (
	"log"
	"time"
)

func StartMonitoring() {
	go monitorDirectly()
}

func monitorDirectly() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from monitorDirectly panic:", r)
			time.Sleep(3 * time.Second)
			go monitorDirectly()
		}
	}()

	for {
		if coreInstance == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if coreInstance.CurrentEngine == "xray" {
			if coreInstance.xrayInstance != nil {
				monitorXrayLoop(coreInstance.xrayInstance)
			}
		} else {
			monitorSingboxLoop()
		}
		time.Sleep(1 * time.Second)
	}
}
