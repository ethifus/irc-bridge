package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"text/template"

	"github.com/thoj/go-ircevent"
)

type NetworkConfig struct {
	Name    string
	Address string
	Channel string
}

type Configuration struct {
	Nicks     []string
	Username  string
	Networks  []NetworkConfig
	Forward   []string
	Templates map[string]string
}

type NetworkConfigAll struct {
	Network   NetworkConfig
	Nicks     []string
	Username  string
	Sink      chan Message
	Reciver   chan Message
	Forward   []string
	Templates map[string]*template.Template
}

type Message struct {
	*irc.Event
	Eventcode string
	Network   string // Network's name, same as in NetworkConfig.Name
}

var logger = log.New(os.Stdout, "irc_bridge:", log.LstdFlags)

// load configuration fron JSON file into Configuration structure
func loadConfig(configPath string) *Configuration {
	configFile, err := os.Open(configPath)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't open config file.")
	}
	defer configFile.Close()

	var config Configuration
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&config); err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't parse config file.")
	}

	logger.Println(config)

	return &config
}

// Setup all callbacks for given connection.
func setupCallbacks(conn *irc.Connection, config NetworkConfigAll) {
	NetworkName := config.Network.Name

	// join desired irc channel on connection success
	conn.AddCallback("001", func(e *irc.Event) {
		logger.Printf("[%s] // join -> %s\n", NetworkName, config.Network.Channel)
		conn.Join(config.Network.Channel)
	})

	// select next nick if current one is already in use
	nickIndex := 0
	conn.AddCallback("433", func(e *irc.Event) {
		nickIndex++
		logger.Printf("[%s] // 433 trying change nick to %s\n", NetworkName,
			config.Nicks[nickIndex])
		conn.Nick(config.Nicks[nickIndex])
	})

	// register all callbacks for events to retransmit
	for _, eventcode := range config.Forward {
		conn.AddCallback(eventcode, makeEventHandler(eventcode, config))
	}
}

func makeEventHandler(eventcode string, config NetworkConfigAll) func(*irc.Event) {
	return func(event *irc.Event) {
		message := Message{
			Network:   config.Network.Name,
			Eventcode: eventcode,
			Event:     event,
		}
		template, ok := config.Templates[eventcode]
		if !ok {
			template = config.Templates["default"]
		}
		logger.Println(formatMessage(template, message))
		config.Sink <- message
	}
}

// Convert given message to string usign given template.
func formatMessage(template *template.Template, message Message) string {
	var buffer bytes.Buffer

	err := template.Execute(&buffer, message)
	if err != nil {
		logger.Fatalf("Invalid template: %s\n", err)
	}

	return buffer.String()
}

// Connect to single Network and join desired channel.
func makeConnection(config NetworkConfigAll) {
	logger.Printf("[%s] (%s/%s)\n", config.Network.Name, config.Network.Address,
		config.Network.Channel)

	conn := irc.IRC(config.Nicks[0], config.Username)
	//conn.VerboseCallbackHandler = true
	//conn.Debug = true

	err := conn.Connect(config.Network.Address)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatalf("Can't connect to network %s.\n", config.Network.Name)
	}

	setupCallbacks(conn, config)

	go func() {
		// wait for messages on config.Reciver and write to irc channel only
		// those that are not recived on this Network/channel
		for {
			message := <-config.Reciver
			if message.Network != config.Network.Name {
				template, ok := config.Templates[message.Eventcode]
				if !ok {
					template = config.Templates["default"]
				}
				text := formatMessage(template, message)
				conn.Privmsg(config.Network.Channel, text)
			}
		}
	}()

	go conn.Loop()
}

func makeConnections(config *Configuration, sink chan Message) []chan Message {
	recivers := make([]chan Message, len(config.Networks))
	templates := makeTemplates(config.Templates)

	for i, Network := range config.Networks {
		reciver := make(chan Message)
		NetworkConfig := NetworkConfigAll{
			Network:   Network,
			Nicks:     config.Nicks,
			Username:  config.Username,
			Sink:      sink,
			Reciver:   reciver,
			Forward:   config.Forward,
			Templates: templates,
		}
		recivers[i] = reciver
		makeConnection(NetworkConfig)
	}
	return recivers
}

// Initialize template.Template object for each template defined in configuration
func makeTemplates(definition map[string]string) map[string]*template.Template {
	result := make(map[string]*template.Template)

	for key, value := range definition {
		tmpl, err := template.New(key).Parse(value)
		if err != nil {
			logger.Fatalf("Could not create template for '%s': %s\n", key, err)
		}
		result[key] = tmpl
	}

	return result
}

// Write all messages recived from sink to all recivers.
func loop(sink chan Message, recivers []chan Message) {
	for {
		message := <-sink
		for _, reciver := range recivers {
			reciver <- message
		}
	}
}

func main() {
	if len(os.Args) == 1 {
		logger.Fatal("No config file specified.")
	}

	config := loadConfig(os.Args[1])
	sink := make(chan Message)
	recivers := makeConnections(config, sink)

	loop(sink, recivers)
}
