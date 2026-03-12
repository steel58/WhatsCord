package main

import (
	"fmt"
	"os"
	"sync"
	"encoding/json"
	"strings"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

type ChatBindings struct {
	ChannelId string `json:"channel_id"`
	WhatsappId string `json:"whatsapp_id"`
}


func init_discord(
	wg *sync.WaitGroup,
	from_whatsapp chan WhatsappMessage,
	to_whatsapp chan WhatsappMessage, 
	shutdown_chan chan bool,
) {
	defer wg.Done()
	// Get the bot token from environment variable (recommended)
	Token := os.Getenv("DISCORD_TOKEN")
	if Token == "" {
		fmt.Println("No token provided. Set the DISCORD_TOKEN environment variable.")
		return
	}

	var bindings []ChatBindings
	err := init_chat_bindings("./channel_bindings.json", &bindings)
	if err != nil {
		fmt.Println("error loading bindings,", err)
		return
	}
	defer store_chat_bindings("./channel_bindings.json", &bindings)

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	dg.Identify.Intents = discordgo.IntentsGuildMessages |
                      	  discordgo.IntentsMessageContent

	// Register the messageCreate function as a callback for the MessageCreate event.
	power := make(chan bool)
	discordMessageHandler := makeMessageHandler(&bindings, to_whatsapp, power)
	dg.AddHandler(discordMessageHandler)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer dg.Close()


	fmt.Println("Bot is now running. Press CTRL+C to exit.")

	for {
		select {
			case msg := <- from_whatsapp:
				id, message := forwardWhatsappMessage(msg, &bindings)
				dg.ChannelMessageSend(id, message)
			case shutdown := <- power:
				if shutdown {
					shutdown_chan <- true
					return
				}
		}
	}
}

func forwardWhatsappMessage(msg WhatsappMessage, bindings *[]ChatBindings)(string, string) {
	var message string
	for _, binding := range *bindings {
		if msg.IsGroup {
			if binding.WhatsappId == msg.GroupId {
				message = fmt.Sprintf("%s: %s", msg.SenderName, msg.Message)
				return binding.ChannelId, message
			}
		} else {
			if binding.WhatsappId == msg.SenderId {
				return binding.ChannelId, msg.Message
			}
		}
	}
	var id string
	for _, binding := range *bindings {
		if binding.WhatsappId == "default" {
			id = binding.ChannelId
		}
	}

	if msg.IsGroup {
		message = fmt.Sprintf("Recieved a message from unbound group: %s, with ID: %s", msg.GroupName, msg.GroupId)
	} else {
		message = fmt.Sprintf("Recieved a message from unbound contact: %s, with ID: %s", msg.SenderName, msg.SenderId)
	}


	return id, message
}


// Add a constructor for the handler that passes in our array of places so we can use it when
// we call !connect

func makeMessageHandler(
	bindings *[]ChatBindings,
	to_whatsapp chan WhatsappMessage,
	power chan bool,
) (
	messageHandler func(*discordgo.Session, *discordgo.MessageCreate),
) {
	messageHandler = func(s *discordgo.Session, m *discordgo.MessageCreate) {
		chat_bindings := bindings
		forward_chan := to_whatsapp
		// Ignore messages from the bot itself.
		if m.Author.ID == s.State.User.ID {
			return
		}

		// Respond to a simple command "!ping"
		if m.Content == "!ping" {
			s.ChannelMessageSend(m.ChannelID, "Pong!")
			return
		}
		// Call !connect default in general so we can filter out unknown messages and put them
		// somewhere pretty
		if strings.HasPrefix(m.Content, "!connect") {
			split := strings.Split(m.Content, " ")
			whatsapp_id := split[len(split) - 1]

			var new_binding ChatBindings
			new_binding.ChannelId = m.ChannelID
			new_binding.WhatsappId = whatsapp_id
			*chat_bindings = append(*chat_bindings, new_binding)

			s.ChannelMessageSend(m.ChannelID, "Bound Chat Here")
			return
		}
		if strings.HasPrefix(m.Content, "!shutdown") {
			s.ChannelMessageSend(m.ChannelID, "Shutting Down")
			power <- true
			return
		}

		// Forward any message to whatsapp side
		for _, binding := range *bindings {
			if binding.ChannelId == m.ChannelID {
				var new_message WhatsappMessage
				new_message.Destination = binding.WhatsappId
				new_message.Message = m.Content
				forward_chan <- new_message
				return
			}
		}
	}
	return
}

// Create get found chats function
func init_chat_bindings(filepath string, bindings *[]ChatBindings) (error) {
	content, err := os.ReadFile(filepath)
	if err != nil { 
		return err
	}

	err = json.Unmarshal(content, &bindings)
	if err != nil { 
		return err
	}
	return nil
}
// Create serialize chats function that can be defered and saves all current chats to the json file
func store_chat_bindings(filepath string, bindings *[]ChatBindings) (error) {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	_, err = file.Write(bytes)

	if err != nil {
		return err
	}

	return nil
}
