package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joho/godotenv"
)

type Creator struct {
	Name  string
	Nth   uint64
	Limit int64
}

type Temporary struct {
	Name  string
	Nth   uint64
	Limit int64
}

var tmpChannelCreators = map[string]*Creator{}
var tmpChannels = map[string]*Temporary{}

func init() {
	var err error

	db, err := os.Open("db.gob")
	if err != nil {
		log.Printf("could not open the db.gob file: %v", err)
		return
	}

	defer db.Close()

	r := bufio.NewReader(db)
	d := gob.NewDecoder(r)

	d.Decode(&tmpChannelCreators)
	// d.Decode(&tmpChannels)

}

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "tmpch",
			Description: "Create a new temporary channel spawner",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Temporary channel spawner name",
					Required:     true,
					Autocomplete: false,
					Type:         discordgo.ApplicationCommandOptionString,
				},
				{
					Name:         "limit",
					Description:  "User Limit, 0 to disable limits",
					Required:     true,
					Autocomplete: false,
					Type:         discordgo.ApplicationCommandOptionInteger,
				},
			},
		},
		{
			Name:        "status",
			Description: "Set the bot status",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{{
				Name:         "status",
				Description:  "New status",
				Required:     true,
				Autocomplete: true,
				Type:         discordgo.ApplicationCommandOptionString,
			}},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"tmpch": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			var name = "channel"
			if opt, ok := optionMap["name"]; ok {
				name = opt.StringValue()
			}

			var limit int64 = 0
			if opt, ok := optionMap["limit"]; ok {
				limit = opt.IntValue()
			}

			ch, err := s.GuildChannelCreateComplex(i.GuildID, discordgo.GuildChannelCreateData{
				Name:      fmt.Sprintf("✚ %s", name),
				Type:      discordgo.ChannelTypeGuildVoice,
				UserLimit: int(limit),
			})
			if err != nil {
				log.Printf("failed to create a channel: %v", err)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("failed to create the channel: %s", err),
					},
				})
				return
			}

			tmpChannelCreators[ch.ID] = &Creator{
				Name:  name,
				Nth:   1,
				Limit: limit,
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "done",
				},
			})
		},

		"name": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			ch, ok := tmpChannels[i.ChannelID]
			if !ok {
				log.Printf("no name given")
				return
			}

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "modals_name",
					Title:    "Set name",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.TextInput{
									CustomID:    "name",
									Label:       "New channel name",
									Style:       discordgo.TextInputShort,
									Value:       ch.Name,
									Placeholder: "required",
									MinLength:   3,
									MaxLength:   20,
								},
							},
						},
					},
				},
			})
			if err != nil {
				log.Printf("failed to create a modal: %v", err)
			}
		},

		"modals_name": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			ch, ok := tmpChannels[i.ChannelID]
			if !ok {
				return
			}

			data := i.ModalSubmitData()

			ch.Name = data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
			_, err := s.ChannelEdit(i.ChannelID, &discordgo.ChannelEdit{
				Name: fmt.Sprintf("%s #%03v", ch.Name, ch.Limit),
			})
			if err != nil {
				log.Printf("failed to edit a tmp channel: %v", err)
				return
			}

			// err = s.MessageReactionAdd(i.Message.ChannelID, i.Message.ID, "👍")
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "new channel name set",
				},
			})
			if err != nil {
				log.Printf("failed to send the response: %v", err)
				return
			}
		},

		"kick": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			_, ok := tmpChannels[i.ChannelID]
			if !ok {
				log.Printf("no name given")
				return
			}

			min := 1
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					CustomID: "modals_kick",
					Title:    "Kick someone",
					Flags:    discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:  "kick",
									MenuType:  discordgo.UserSelectMenu,
									MinValues: &min,
								},
							},
						},
					},
				},
			})
			if err != nil {
				log.Printf("failed to create a modal: %v", err)
			}
		},

		"modals_kick": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			_, ok := tmpChannels[i.ChannelID]
			if !ok {
				return
			}

			data := i.ModalSubmitData()

			values := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.SelectMenu).Options

			for _, val := range values {
				err := s.GuildMemberMove(i.GuildID, val.Value, nil)
				if err != nil {
					log.Printf("failed to kick: %v", err)
				}
			}

			// err = s.MessageReactionAdd(i.Message.ChannelID, i.Message.ID, "👍")
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "intruder kicked",
				},
			})
			if err != nil {
				log.Printf("failed to send the response: %v", err)
				return
			}
		},

		"clear": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "TODO",
				},
			})
		},

		"limit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "TODO",
				},
			})
		},

		"ban": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "TODO",
				},
			})
		},

		"give": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "TODO",
				},
			})
		},

		"status": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "nice try",
				},
			})
		},
	}
)

func spawnerChannelUpdate(s *discordgo.Session, vc *discordgo.VoiceStateUpdate, creator *Creator) {
	n := creator.Nth
	creator.Nth += 1
	ch, err := s.GuildChannelCreateComplex(vc.GuildID, discordgo.GuildChannelCreateData{
		Name:      fmt.Sprintf("%s #%03d", creator.Name, n),
		Type:      discordgo.ChannelTypeGuildVoice,
		UserLimit: int(creator.Limit),
	})
	if err != nil {
		log.Printf("failed to create a channel: %v", err)
		return
	}

	err = s.GuildMemberMove(vc.GuildID, vc.UserID, &ch.ID)
	if err != nil {
		log.Printf("failed to move the user: %v", err)
		return
	}

	_, err = s.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
		Content: fmt.Sprintf("**TMPCH SETTINGS** for <@%s> **CHANNEL**", vc.UserID),
		TTS:     false,
		Embeds:  []*discordgo.MessageEmbed{},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Set name",
						CustomID: "name",
						Emoji: discordgo.ComponentEmoji{
							Name:     "📝",
							Animated: false,
						},
					},
					discordgo.Button{
						Label:    "Kick user",
						CustomID: "kick",
						Style:    discordgo.SecondaryButton,
						Emoji: discordgo.ComponentEmoji{
							Name:     "🚫",
							Animated: false,
						},
					},
					discordgo.Button{
						Label:    "Clean chat",
						CustomID: "clear",
						Emoji: discordgo.ComponentEmoji{
							Name:     "🧹",
							Animated: false,
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Set limit",
						CustomID: "limit",
						Emoji: discordgo.ComponentEmoji{
							Name:     "🎚️",
							Animated: false,
						},
					},
					discordgo.Button{
						Label:    "Ban user",
						CustomID: "ban",
						Style:    discordgo.DangerButton,
						Emoji: discordgo.ComponentEmoji{
							Name:     "⛔",
							Animated: false,
						},
					},
					discordgo.Button{
						Label:    "Give channel",
						CustomID: "give",
						Emoji: discordgo.ComponentEmoji{
							Name:     "👑",
							Animated: false,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("failed to send the tmp channel msg: %v", err)
	}
	// s.ChannelMessageSend(ch.ID, fmt.Sprintf("hello there <@%s>", vc.UserID))

	tmpChannels[ch.ID] = &Temporary{
		Name:  creator.Name,
		Nth:   n,
		Limit: creator.Limit,
	}
}

func tmpChannelUpdate(s *discordgo.Session, vc *discordgo.VoiceStateUpdate) {
	if vc.ChannelID != vc.BeforeUpdate.ChannelID {
		ch, err := s.Channel(vc.BeforeUpdate.ChannelID)
		if err != nil {
			log.Printf("failed to get user count: %v", err)
			return
		}
		if ch.MemberCount == 0 {
			_, err := s.ChannelDelete(vc.BeforeUpdate.ChannelID)
			if err != nil {
				log.Printf("failed to delete the channel: %v", err)
				return
			}
			delete(tmpChannels, vc.ChannelID)

		}
	}
}

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
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			log.Printf("cmd %s", i.ApplicationCommandData().Name)
			if handler, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				handler(s, i)
			}
		case discordgo.InteractionMessageComponent:
			log.Printf("comp %s", i.MessageComponentData().CustomID)
			if handler, ok := commandHandlers[i.MessageComponentData().CustomID]; ok {
				handler(s, i)
			}
		case discordgo.InteractionModalSubmit:
			log.Printf("modal %s", i.ModalSubmitData().CustomID)
			if handler, ok := commandHandlers[i.ModalSubmitData().CustomID]; ok {
				handler(s, i)
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		go (func() {
			statusList := []string{"TEST1", "TEST2", "TEST3"}
			for {
				for _, status := range statusList {
					s.UpdateStatusComplex(discordgo.UpdateStatusData{
						Status: "online",
						Activities: []*discordgo.Activity{
							{
								Name:  "Custom Status",
								Type:  discordgo.ActivityTypeCustom,
								State: status,
							},
						},
					})
					time.Sleep(60)
				}
			}
		})()
		log.Printf("Logged in as %s#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	sess.AddHandler(func(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
		// dbg_log := "voice state update"
		// if m.VoiceState != nil {
		// 	dbg_log = fmt.Sprintf("%s new:%#v", dbg_log, m.VoiceState)
		// } else {
		// 	dbg_log = fmt.Sprintf("%s new:nil", dbg_log)
		// }
		// if m.BeforeUpdate != nil {
		// 	dbg_log = fmt.Sprintf("%s old:%#v", dbg_log, m.BeforeUpdate)
		// } else {
		// 	dbg_log = fmt.Sprintf("%s old:nil", dbg_log)
		// }
		// log.Printf("%s", dbg_log)

		if m.Member == nil {
			return
		}

		if creator, ok := tmpChannelCreators[m.ChannelID]; ok {
			spawnerChannelUpdate(s, m, creator)
		}

		if m.BeforeUpdate != nil {
			if _, ok := tmpChannels[m.BeforeUpdate.ChannelID]; ok {
				tmpChannelUpdate(s, m)
			}

		} else if _, ok := tmpChannels[m.ChannelID]; ok {
			// idk
		}
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

	// close channels

	// for ch := range tmpChannelCreators {
	// 	sess.ChannelDelete(ch)
	// }
	log.Printf("closing tmpChannels")
	for ch := range tmpChannels {
		sess.ChannelDelete(ch)
	}

	// dump channel info into a db.gob file

	log.Printf("saving tmpChannelCreators")
	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)

	err = e.Encode(tmpChannelCreators)
	if err != nil {
		log.Panicf("could not serialize tmpChannelCreators: %v", err)
	}

	// err = e.Encode(tmpChannels)
	// if err != nil {
	// 	log.Panicf("could not serialize tmpChannels: %v", err)
	// }

	db, err := os.Create("db.gob")
	if err != nil {
		log.Panicf("could not open the db.gob file: %v", err)
	}

	db.Write(b.Bytes())

	defer db.Close()

	// log.Printf("removing commands ...")
	// for _, v := range registeredCmds {
	// 	err := sess.ApplicationCommandDelete(sess.State.User.ID, "", v.ID)
	// 	if err != nil {
	// 		log.Panicf("cannot delete cmd '%s': %v", v.Name, err)
	// 	}
	// }
	// log.Printf("removing commands done")
}
