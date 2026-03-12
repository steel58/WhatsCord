package main

import (
	"sync"
)

func main() {

	wg := new (sync.WaitGroup)
	wg.Add(2)
	to_discord := make(chan WhatsappMessage)
	to_whatsapp := make(chan WhatsappMessage)
	shutdown := make(chan bool)

	go init_discord(wg, to_discord, to_whatsapp, shutdown)
	go init_whatsapp(wg, to_whatsapp, to_discord, shutdown)


	wg.Wait()
}

