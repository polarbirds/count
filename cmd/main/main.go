package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/count/internal/count"
	"github.com/polarbirds/jako/pkg/command"
	log "github.com/sirupsen/logrus"
)

var (
	Token string
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Error("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		log.Error("error opening connection,", err)
		return
	}

	// createData(dg)
	createDataAsync(dg)

	log.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if strings.HasPrefix(m.Content, "!") {
		source, args, err := command.GetCommand(m.Content)
		if err != nil {
			log.Error(err)
			return
		}

		var discErr error

		switch strings.ToLower(source) {
		case "count":
			var msg string
			var err error

			if len(args) < 1 {
				msg, err = count.TopCount("all")
			} else if len(args) <= 2 {
				target := args[0]
				if len(args) == 2 {
					msg, err = count.SingleWordCount(target, args[1])
				} else {
					msg, err = count.TopCount(target)
				}
			} else {
				err = errors.New("1 or 2 args plz")
			}

			if err != nil {
				log.Error(err)
				s.UpdateStatus(0, err.Error())
				break
			}

			_, discErr = s.ChannelMessageSend(m.ChannelID, msg)
		}

		if discErr != nil {
			log.Error(discErr)
		}
	} else {
		count.BuildMessage(m.Message)
	}
}

func getMessagesFromChannel(s *discordgo.Session, channel *discordgo.Channel) []*discordgo.Message {
	beforeID := channel.LastMessageID
	var msgs []*discordgo.Message
	var failedAttempts = 0
	for {
		m, err := s.ChannelMessages(channel.ID, 100, beforeID, "", "")
		if err != nil {
			log.Fatal(err)
			if failedAttempts > 10 {
				log.Error(err)
				break
			} else {
				failedAttempts++
				continue
			}
		}

		if len(m) < 1 {
			break
		}

		msgs = append(msgs, m...)
		log.Infof("fetched %d messages so far on channel %s", len(msgs), channel.Name)
		beforeID = m[len(m)-1].ID
		failedAttempts = 0
	}
	return msgs
}

func createDataAsync(s *discordgo.Session) {
	s.UpdateStatus(0, "Building data...")
	for _, guild := range s.State.Guilds {
		log.Infof("Parsing guild %s: %s", guild.Name, guild.ID)

		channels, err := s.GuildChannels(guild.ID)
		if err != nil {
			log.Fatal(err)
			return
		}

		ch := make(chan []*discordgo.Message)
		var wg sync.WaitGroup

		wg.Add(len(channels) + 1)

		// build all channels async
		for _, v := range channels {
			if v.Type != discordgo.ChannelTypeGuildText {
				wg.Done()
				continue
			}

			log.Infof("name: %s, id: %s", v.Name, v.ID)
			go func(dChan *discordgo.Channel, c chan<- []*discordgo.Message, w *sync.WaitGroup) {
				c <- getMessagesFromChannel(s, dChan)
				w.Done()
			}(v, ch, &wg)
		}

		// build trump async
		go func(c chan<- []*discordgo.Message, w *sync.WaitGroup) {
			c <- getTrumpAsDiscordMsgs()
			w.Done()
		}(ch, &wg)

		go func() {
			wg.Wait()
			close(ch)
		}()

		for m := range ch {
			for _, msg := range m {
				count.BuildMessage(msg)
			}
		}
		log.Info("data-parsing done")
	}

	s.UpdateStatus(0, "Finished building data")
}

func getTrumpAsDiscordMsgs() []*discordgo.Message {
	log.Info("Building trump")

	resp, err := http.Get("https://raw.githubusercontent.com/ryanmcdermott/trump-speeches/master/speeches.txt")
	if err != nil {
		log.Error(err)
		return nil
	}

	b, err := ioutil.ReadAll(resp.Body)
	bod := string(b)

	lines := strings.Split(bod, "\n")
	pattern, err := regexp.Compile("^SPEECH \\d+")
	if err != nil {
		log.Error(err)
		return nil
	}

	msgs := []*discordgo.Message{}

	author := discordgo.User{
		Username: "trump",
		Bot:      true,
	}

	for _, line := range lines {
		if len(line) == 0 || pattern.MatchString(line) {
			continue
		}

		msg := discordgo.Message{
			Content: line,
			Author:  &author,
		}
		msgs = append(msgs, &msg)
	}
	log.Info("Trump built")

	return msgs // this is death. discordgo is pointer spaghetti
}
