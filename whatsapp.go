package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	// "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)
func init_whatsapp(
	wg *sync.WaitGroup,
	from_discord chan string,
	to_discord chan WhatsappMessage,
	shutdown_chan chan bool,
) {
	defer wg.Done()
	dbLog := waLog.Stdout("Database", "DEBUG", false)
	ctx := context.Background()
	container, err := sqlstore.New(ctx, "sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", false)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	whatsappEventHandler := makeEventHandler(client, to_discord)

	client.AddEventHandler(whatsappEventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}
	defer client.Disconnect()
	for {
		select {
			case shutdown := <- shutdown_chan:
				if shutdown {
					return
				}
		}
	}

}


func makeEventHandler(client *whatsmeow.Client, out_chan chan WhatsappMessage) (eventHandler func(interface{})) { 
	eventHandler = func (evt interface{}) {
		output_channel := out_chan
		whatsapp_client := client
		switch v := evt.(type) {
		case *events.Message:
			var fwd_message  WhatsappMessage
			fwd_message.IsGroup = v.Info.IsGroup
			if (v.Info.IsGroup) {
				group, err := whatsapp_client.GetGroupInfo(context.Background(), v.Info.Chat)
				if (err != nil) {
					fmt.Println("Erorr getting group info")
					fmt.Println(err)
				}
				fwd_message.GroupId = v.Info.Chat.String()
				fwd_message.GroupName = group.Name
			} 

			fwd_message.SenderName = v.Info.PushName
			fwd_message.SenderId = v.Info.Sender.String()
			fwd_message.Message = v.Message.GetConversation()
			output_channel <- fwd_message
			return
		}
	}

	return
}
