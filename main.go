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

func main() {
	fmt.Println("Hello, World!")

	err := godotenv.Load()
	if err != nil {
		log.Fatal()
	}

	dc_token := os.Getenv("DC_TOKEN")

	sess, err := discordgo.New(fmt.Sprintf("Bot %s", dc_token))

	sess.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		if m.Content == "setup" {
			s.ChannelMessageSend(m.ChannelID, "wait")
		}
	})

	sess.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err = sess.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	fmt.Println("bot online")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
