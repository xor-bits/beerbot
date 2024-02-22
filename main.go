package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"

	"github.com/joho/godotenv"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "setup",
			Description: "Create a new spawner channel",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"setup": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "slash command",
				},
			})
		},
	}
)

var sess *discordgo.Session

func init() {
	var err error

	err = godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	dc_token := os.Getenv("DC_TOKEN")

	sess, err = discordgo.New(fmt.Sprintf("Bot %s", dc_token))
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if handler, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			handler(s, i)
		}
	})

	sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		s.UpdateGameStatus(0, "whassup")
		log.Printf("Logged in as %s#%v", s.State.User.Username, s.State.User.Discriminator)
	})
}

func main() {
	sess.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err := sess.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	log.Printf("registering commands ...")
	registeredCmds := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := sess.ApplicationCommandCreate(sess.State.User.ID, "", v)
		if err != nil {
			log.Panicf("cannot create cmd '%s': %v", v.Name, err)
		}
		log.Printf("created cmd %s", v.Name)
		registeredCmds[i] = cmd
	}
	log.Printf("registering commands done")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Printf("removing commands ...")
	for _, v := range registeredCmds {
		err := sess.ApplicationCommandDelete(sess.State.User.ID, "", v.ID)
		if err != nil {
			log.Panicf("cannot delete cmd '%s': %v", v.Name, err)
		}
	}
	log.Printf("removing commands done")
}
